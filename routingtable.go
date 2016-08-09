package dht

import (
	"container/heap"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

// maxPrefixLength is the length of DHT node.
const maxPrefixLength = 160

// node represents a DHT node.
type node struct {
	id             *bitmap
	addr           *net.UDPAddr
	lastActiveTime time.Time
}

// newNode returns a node pointer.
func newNode(id, network, address string) (*node, error) {
	if len(id) != 20 {
		return nil, errors.New("node id should be a 20-length string")
	}

	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}

	return &node{newBitmapFromString(id), addr, time.Now()}, nil
}

// newNodeFromCompactInfo parses compactNodeInfo and returns a node pointer.
func newNodeFromCompactInfo(
	compactNodeInfo string, network string) (*node, error) {

	if len(compactNodeInfo) != 26 {
		return nil, errors.New("compactNodeInfo should be a 26-length string")
	}

	id := compactNodeInfo[:20]
	ip, port, _ := decodeCompactIPPortInfo(compactNodeInfo[20:])

	return newNode(id, network, genAddress(ip.String(), port))
}

// CompactIPPortInfo returns "Compact IP-address/port info".
// See http://www.bittorrent.org/beps/bep_0005.html.
func (node *node) CompactIPPortInfo() string {
	info, _ := encodeCompactIPPortInfo(node.addr.IP, node.addr.Port)
	return info
}

// CompactNodeInfo returns "Compact node info".
// See http://www.bittorrent.org/beps/bep_0005.html.
func (node *node) CompactNodeInfo() string {
	return strings.Join([]string{
		node.id.RawString(), node.CompactIPPortInfo(),
	}, "")
}

// peer represents a peer contact.
type peer struct {
	ip    net.IP
	port  int
	token string
}

// newPeer returns a new peer pointer.
func newPeer(ip net.IP, port int, token string) *peer {
	return &peer{
		ip:    ip,
		port:  port,
		token: token,
	}
}

// newPeerFromCompactIPPortInfo create a peer pointer by compact ip/port info.
func newPeerFromCompactIPPortInfo(compactInfo, token string) (*peer, error) {
	ip, port, err := decodeCompactIPPortInfo(compactInfo)
	if err != nil {
		return nil, err
	}

	return newPeer(ip, port, token), nil
}

// CompactNodeInfo returns "Compact node info".
// See http://www.bittorrent.org/beps/bep_0005.html.
func (p *peer) CompactIPPortInfo() string {
	info, _ := encodeCompactIPPortInfo(p.ip, p.port)
	return info
}

// peersManager represents a proxy that manipulates peers.
type peersManager struct {
	sync.RWMutex
	table *syncedMap
	dht   *DHT
}

// newPeersManager returns a new peersManager.
func newPeersManager(dht *DHT) *peersManager {
	return &peersManager{
		table: newSyncedMap(),
		dht:   dht,
	}
}

// Insert adds a peer into peersManager.
func (pm *peersManager) Insert(infoHash string, peer *peer) {
	pm.Lock()
	if _, ok := pm.table.Get(infoHash); !ok {
		pm.table.Set(infoHash, newKeyedDeque())
	}
	pm.Unlock()

	v, _ := pm.table.Get(infoHash)
	v.(*keyedDeque).Push(peer.CompactIPPortInfo(), peer)
}

// GetPeers returns size-length peers who announces having infoHash.
func (pm *peersManager) GetPeers(infoHash string, size int) []*peer {
	peers := make([]*peer, 0, size)

	v, ok := pm.table.Get(infoHash)
	if !ok {
		return peers
	}

	for e := range v.(*keyedDeque).Iter() {
		peers = append(peers, e.Value.(*peer))
	}

	if len(peers) > size {
		peers = peers[len(peers)-size:]
	}
	return peers
}

// kBucket represents a k-size bucket.
type kBucket struct {
	sync.RWMutex
	nodes, candidates *keyedDeque
	lastChanged       time.Time
	prefix            *bitmap
}

// newKBucket returns a new kBucket pointer.
func newKBucket(prefix *bitmap) *kBucket {
	kbucket := &kBucket{
		nodes:       newKeyedDeque(),
		candidates:  newKeyedDeque(),
		lastChanged: time.Now(),
		prefix:      prefix,
	}
	return kbucket
}

// LastChanged return the last time when it changes.
func (kbucket *kBucket) LastChanged() time.Time {
	kbucket.RLock()
	defer kbucket.RUnlock()

	return kbucket.lastChanged
}

// RandomChildID returns a random id that has the same prefix with kbucket.
func (kbucket *kBucket) RandomChildID() string {
	prefixLen := kbucket.prefix.Size / 8

	return strings.Join([]string{
		kbucket.prefix.RawString()[:prefixLen],
		randomString(20 - prefixLen),
	}, "")
}

// UpdateTimestamp update kbucket's last changed time..
func (kbucket *kBucket) UpdateTimestamp() {
	kbucket.Lock()
	defer kbucket.Unlock()

	kbucket.lastChanged = time.Now()
}

// Insert inserts node to the kbucket. It returns whether the node is new in
// the kbucket.
func (kbucket *kBucket) Insert(no *node, rt *routingTable) bool {
	isNew := !kbucket.nodes.HasKey(no.id.RawString())

	kbucket.nodes.Push(no.id.RawString(), no)
	rt.cachedNodes.Set(no.addr.String(), no)
	rt.Touch(kbucket)

	return isNew
}

// Replace removes node, then put kbucket.candidates.Back() to the right
// place of kbucket.nodes.
func (kbucket *kBucket) Replace(no *node, rt *routingTable) {
	kbucket.nodes.Delete(no.id.RawString())

	rt.cachedNodes.Delete(no.addr.String())
	rt.Touch(kbucket)

	if kbucket.candidates.Len() == 0 {
		return
	}

	no = kbucket.candidates.Remove(kbucket.candidates.Back()).(*node)

	inserted := false
	for e := range kbucket.nodes.Iter() {
		if e.Value.(*node).lastActiveTime.After(
			no.lastActiveTime) && !inserted {

			kbucket.nodes.InsertBefore(no, e)
			inserted = true
		}
	}

	if !inserted {
		kbucket.nodes.PushBack(no)
	}
}

// Clear resets the kbucket.
func (kbucket *kBucket) Clear() {
	kbucket.Lock()
	defer kbucket.Unlock()

	kbucket.nodes.Clear()
	kbucket.candidates.Clear()
	kbucket.lastChanged = time.Now()
}

// Fresh pings the expired nodes in the kbucket.
func (kbucket *kBucket) Fresh(dht *DHT) {
	for e := range kbucket.nodes.Iter() {
		no := e.Value.(*node)
		if time.Since(no.lastActiveTime) > dht.NodeExpriedAfter {
			dht.transactionManager.ping(no)
		}
	}
}

// routingTableNode represents routing table tree node.
type routingTableNode struct {
	sync.RWMutex
	children []*routingTableNode
	kbucket  *kBucket
}

// newRoutingTableNode returns a new routingTableNode pointer.
func newRoutingTableNode(prefix *bitmap) *routingTableNode {
	return &routingTableNode{
		children: make([]*routingTableNode, 2),
		kbucket:  newKBucket(prefix),
	}
}

// Child returns routingTableNode's left or right child.
func (tableNode *routingTableNode) Child(index int) *routingTableNode {
	if index >= 2 {
		return nil
	}

	tableNode.RLock()
	defer tableNode.RUnlock()

	return tableNode.children[index]
}

// SetChild sets routingTableNode's left or right child. When index is 0, it's
// the left child, if 1, it's the right child.
func (tableNode *routingTableNode) SetChild(index int, c *routingTableNode) {
	tableNode.Lock()
	defer tableNode.Unlock()

	tableNode.children[index] = c
}

// KBucket returns the kbucket routingTableNode holds.
func (tableNode *routingTableNode) KBucket() *kBucket {
	tableNode.RLock()
	defer tableNode.RUnlock()

	return tableNode.kbucket
}

// SetKBucket sets the kbucket.
func (tableNode *routingTableNode) SetKBucket(kbucket *kBucket) {
	tableNode.Lock()
	defer tableNode.Unlock()

	tableNode.kbucket = kbucket
}

// Split splits current routingTableNode and sets it's two children.
func (tableNode *routingTableNode) Split(rt *routingTable) {
	prefixLen := tableNode.KBucket().prefix.Size

	if prefixLen == maxPrefixLength {
		return
	}

	for i := 0; i < 2; i++ {
		tableNode.SetChild(i, newRoutingTableNode(newBitmapFrom(
			tableNode.KBucket().prefix, prefixLen+1)))
	}

	tableNode.Lock()
	tableNode.children[1].kbucket.prefix.Set(prefixLen)
	tableNode.Unlock()

	for e := range tableNode.KBucket().nodes.Iter() {
		nd := e.Value.(*node)
		tableNode.Child(nd.id.Bit(prefixLen)).KBucket().nodes.PushBack(nd)
	}

	for e := range tableNode.KBucket().candidates.Iter() {
		nd := e.Value.(*node)
		tableNode.Child(nd.id.Bit(prefixLen)).KBucket().candidates.PushBack(nd)
	}

	rt.cachedKBuckets.Delete(tableNode.KBucket().prefix.String())
	// tableNode.SetKBucket(nil)

	for i := 0; i < 2; i++ {
		rt.Touch(tableNode.Child(i).KBucket())
	}
}

// Clear resets the routingTableNode.
func (tableNode *routingTableNode) Clear() {
	tableNode.Lock()
	defer tableNode.Unlock()

	tableNode.children = make([]*routingTableNode, 2)
	tableNode.kbucket.Clear()
}

// routingTable implements the routing table in DHT protocol.
type routingTable struct {
	*sync.RWMutex
	k              int
	root           *routingTableNode
	cachedNodes    *syncedMap
	cachedKBuckets *keyedDeque
	dht            *DHT
}

// newRoutingTable returns a new routingTable pointer.
func newRoutingTable(k int, dht *DHT) *routingTable {
	root := newRoutingTableNode(newBitmap(0))

	rt := &routingTable{
		RWMutex:        &sync.RWMutex{},
		k:              k,
		root:           root,
		cachedNodes:    newSyncedMap(),
		cachedKBuckets: newKeyedDeque(),
		dht:            dht,
	}

	rt.cachedKBuckets.Push(root.kbucket.prefix.String(), root.kbucket)
	return rt
}

// Touch updates the routing table. It will put the kbucket to the end of
// kbucket list.
func (rt *routingTable) Touch(kbucket *kBucket) {
	kbucket.UpdateTimestamp()
	rt.cachedKBuckets.Push(kbucket.prefix.String(), kbucket)
}

// Insert adds a node to routing table. It returns whether the node is new
// in the routingtable.
func (rt *routingTable) Insert(nd *node) bool {
	rt.Lock()
	defer rt.Unlock()

	if rt.dht.blackList.in(nd.addr.IP.String(), nd.addr.Port) ||
		rt.cachedNodes.Len() >= rt.dht.MaxNodes {
		return false
	}

	var next *routingTableNode
	root := rt.root

	for prefixLen := 1; prefixLen <= maxPrefixLength; prefixLen++ {
		next = root.Child(nd.id.Bit(prefixLen - 1))

		if next != nil {
			// If next is not the leaf.
			root = next
		} else if root.KBucket().nodes.Len() < rt.k ||
			root.KBucket().nodes.HasKey(nd.id.RawString()) {

			return root.KBucket().Insert(nd, rt)
		} else if root.KBucket().prefix.Compare(nd.id, prefixLen-1) == 0 {
			// If node has the same prefix with kbucket, split it.

			root.Split(rt)
			root = root.Child(nd.id.Bit(prefixLen - 1))
		} else {
			// Finally, store node as a candidate and fresh the kbucket.
			root.KBucket().candidates.PushBack(nd)
			if root.KBucket().candidates.Len() > rt.k {
				root.KBucket().candidates.Remove(
					root.KBucket().candidates.Front())
			}

			go root.KBucket().Fresh(rt.dht)
			return false
		}
	}
	return false
}

// GetNeighbors returns the size-length nodes closest to id.
func (rt *routingTable) GetNeighbors(id *bitmap, size int) []*node {
	rt.RLock()
	nodes := make([]interface{}, 0, rt.cachedNodes.Len())
	for item := range rt.cachedNodes.Iter() {
		nodes = append(nodes, item.val.(*node))
	}
	rt.RUnlock()

	neighbors := getTopK(nodes, id, size)
	result := make([]*node, len(neighbors))

	for i, nd := range neighbors {
		result[i] = nd.(*node)
	}
	return result
}

// GetNeighborIds return the size-length compact node info closest to id.
func (rt *routingTable) GetNeighborCompactInfos(id *bitmap, size int) []string {
	neighbors := rt.GetNeighbors(id, size)
	infos := make([]string, len(neighbors))

	for i, no := range neighbors {
		infos[i] = no.CompactNodeInfo()
	}

	return infos
}

// GetNodeKBucktById returns node whose id is `id` and the kbucket it
// belongs to.
func (rt *routingTable) GetNodeKBucktById(id *bitmap) (
	nd *node, kbucket *kBucket) {

	rt.RLock()
	defer rt.RUnlock()

	var next *routingTableNode
	root := rt.root

	for prefixLen := 1; prefixLen <= maxPrefixLength; prefixLen++ {
		next = root.Child(id.Bit(prefixLen - 1))
		if next == nil {
			v, ok := root.KBucket().nodes.Get(id.RawString())
			if !ok {
				return
			}
			nd, kbucket = v.Value.(*node), root.KBucket()
			return
		}
		root = next
	}
	return
}

// GetNodeByAddress finds node by address.
func (rt *routingTable) GetNodeByAddress(address string) (no *node, ok bool) {
	rt.RLock()
	defer rt.RUnlock()

	v, ok := rt.cachedNodes.Get(address)
	if ok {
		no = v.(*node)
	}
	return
}

// Remove deletes the node whose id is `id`.
func (rt *routingTable) Remove(id *bitmap) {
	if nd, kbucket := rt.GetNodeKBucktById(id); nd != nil {
		rt.Lock()
		kbucket.Replace(nd, rt)
		rt.Unlock()
	}
}

// Remove deletes the node whose address is `ip:port`.
func (rt *routingTable) RemoveByAddr(address string) {
	v, ok := rt.cachedNodes.Get(address)
	if ok {
		rt.Remove(v.(*node).id)
	}
}

// Clear resets the routingTable.
func (rt *routingTable) Clear() {
	rt.Lock()
	defer rt.Unlock()

	rt.root.Clear()
	rt.cachedNodes.Clear()
	rt.cachedKBuckets.Clear()
	rt.cachedKBuckets.Push(rt.root.kbucket.prefix.String(), rt.root.kbucket)
}

// Fresh sends findNode to all nodes in the expired nodes.
func (rt *routingTable) Fresh() {
	now := time.Now()

	for e := range rt.cachedKBuckets.Iter() {
		kbucket := e.Value.(*kBucket)
		if now.Sub(kbucket.LastChanged()) < rt.dht.KBucketExpiredAfter ||
			kbucket.nodes.Len() == 0 {
			continue
		}

		for e := range kbucket.nodes.Iter() {
			no := e.Value.(*node)
			rt.dht.transactionManager.findNode(no, kbucket.RandomChildID())
		}
	}

	if rt.dht.IsCrawlMode() {
		rt.Clear()
	}
}

// Len returns the number of nodes in table.
func (rt *routingTable) Len() int {
	rt.RLock()
	defer rt.RUnlock()

	return rt.cachedNodes.Len()
}

// Implemention of heap with heap.Interface.
type heapItem struct {
	distance *bitmap
	value    interface{}
}

type topKHeap []*heapItem

func (kHeap topKHeap) Len() int {
	return len(kHeap)
}

func (kHeap topKHeap) Less(i, j int) bool {
	return kHeap[i].distance.Compare(kHeap[j].distance, maxPrefixLength) == 1
}

func (kHeap topKHeap) Swap(i, j int) {
	kHeap[i], kHeap[j] = kHeap[j], kHeap[i]
}

func (kHeap *topKHeap) Push(x interface{}) {
	*kHeap = append(*kHeap, x.(*heapItem))
}

func (kHeap *topKHeap) Pop() interface{} {
	n := len(*kHeap)
	x := (*kHeap)[n-1]
	*kHeap = (*kHeap)[:n-1]
	return x
}

// getTopK solves the top-k problem with heap. It's time complexity is
// O(n*log(k)). When n is large, time complexity will be too high, need to be
// optimized.
func getTopK(queue []interface{}, id *bitmap, k int) []interface{} {
	topkHeap := make(topKHeap, 0, k+1)

	for _, value := range queue {
		node := value.(*node)
		item := &heapItem{
			id.Xor(node.id),
			value,
		}
		heap.Push(&topkHeap, item)
		if topkHeap.Len() > k {
			heap.Pop(&topkHeap)
		}
	}

	tops := make([]interface{}, topkHeap.Len())
	for i := len(tops) - 1; i >= 0; i-- {
		tops[i] = heap.Pop(&topkHeap).(*heapItem).value
	}

	return tops
}
