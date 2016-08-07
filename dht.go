// Package dht implements the bittorrent dht protocol. For more information
// see http://www.bittorrent.org/beps/bep_0005.html.

package dht

import (
	"encoding/hex"
	"errors"
	"expvar"
	"net"
	"time"
)

var (
	requestNum  = expvar.NewInt("requestNum")
	responseNum = expvar.NewInt("responseNum")
	sendNum     = expvar.NewInt("sendInt")
)

const (
	StandardMode = iota
	CrawlMode
)

// DHTConfig represents the configure of dht.
type Config struct {
	K                    int
	KBucketSize          int
	Network              string
	Address              string
	PrimeNodes           []string
	KBucketExpiredAfter  time.Duration
	NodeExpriedAfter     time.Duration
	CheckKBucketPeriod   time.Duration
	TokenExpiredAfter    time.Duration
	MaxTransactionCursor uint64
	MaxNodes             int
	OnGetPeers           func(string, string, int)
	OnAnnouncePeer       func(string, string, int)
	BlockedIPs           []string
	Mode                 int
	Try                  int
}

// newStandardConfig returns a Config pointer with default values.
func NewStandardConfig() *Config {
	return &Config{
		K:           8,
		KBucketSize: 8,
		Network:     "udp4",
		Address:     ":6881",
		PrimeNodes: []string{
			"router.bittorrent.com:6881",
			"router.utorrent.com:6881",
			"dht.transmissionbt.com:6881",
		},
		NodeExpriedAfter:     time.Duration(time.Minute * 15),
		KBucketExpiredAfter:  time.Duration(time.Minute * 15),
		CheckKBucketPeriod:   time.Duration(time.Second * 30),
		TokenExpiredAfter:    time.Duration(time.Minute * 10),
		MaxTransactionCursor: 4294967296, // 2 ^ 32
		MaxNodes:             5000,
		BlockedIPs:           make([]string, 0),
		Try:                  2,
		Mode:                 StandardMode,
	}
}

func NewCrawlConfig() *Config {
	config := NewStandardConfig()
	config.NodeExpriedAfter = 0
	config.KBucketExpiredAfter = 0
	config.CheckKBucketPeriod = time.Second * 5
	config.KBucketSize = 4294967296
	config.Mode = CrawlMode

	return config
}

// DHT represents a DHT node.
type DHT struct {
	*Config
	node               *node
	conn               *net.UDPConn
	routingTable       *routingTable
	transactionManager *transactionManager
	peersManager       *peersManager
	tokenManager       *tokenManager
	blackList          *blackList
	Ready              bool
}

// NewDHT returns a DHT pointer.
// If config is nil, then config will be set to the default config.
func New(config *Config) *DHT {
	if config == nil {
		config = NewStandardConfig()
	}

	node, err := newNode(randomString(20), config.Network, config.Address)
	if err != nil {
		panic(err)
	}

	d := &DHT{
		Config:    config,
		node:      node,
		blackList: newBlackList(),
	}

	for _, ip := range config.BlockedIPs {
		d.blackList.insert(ip, -1)
	}

	go func() {
		for _, ip := range getLocalIPs() {
			d.blackList.insert(ip, -1)
		}

		ip := getRemoteIP()
		if ip != "" {
			d.blackList.insert(ip, -1)
		}
	}()

	return d
}

// IsStandardMode returns whether mode is StandardMode.
func (dht *DHT) IsStandardMode() bool {
	return dht.Mode == StandardMode
}

// IsStandardMode returns whether mode is CrawlMode.
func (dht *DHT) IsCrawlMode() bool {
	return dht.Mode == CrawlMode
}

// init initializes global varables.
func (dht *DHT) init() {
	listener, err := net.ListenPacket(dht.Network, dht.Address)
	if err != nil {
		panic(err)
	}

	dht.conn = listener.(*net.UDPConn)
	dht.routingTable = newRoutingTable(dht.KBucketSize, dht)
	dht.peersManager = newPeersManager(dht)
	dht.tokenManager = newTokenManager(dht.TokenExpiredAfter, dht)
	dht.transactionManager = newTransactionManager(
		dht.MaxTransactionCursor, dht)

	go dht.transactionManager.run()
	go dht.blackList.clear()
}

// join make current node join the dht network.
func (dht *DHT) join() {
	for _, addr := range dht.PrimeNodes {
		raddr, err := net.ResolveUDPAddr(dht.Network, addr)
		if err != nil {
			continue
		}

		// NOTE: Temporary node has NOT node id.
		dht.transactionManager.findNode(
			&node{addr: raddr},
			dht.node.id.RawString(),
		)
	}
}

// listen receives message from udp.
func (dht *DHT) listen(packetChan chan packet) {
	go func() {
		buff := make([]byte, 4096)
		for {
			n, raddr, err := dht.conn.ReadFromUDP(buff)
			if err != nil {
				continue
			}

			packetChan <- packet{buff[:n], raddr}
		}
	}()
}

// id returns a id near to target if target is not null, otherwise it returns
// the dht's node id.
func (dht *DHT) id(target string) string {
	if dht.IsStandardMode() || target == "" {
		return dht.node.id.RawString()
	}
	return target[:10] + dht.node.id.RawString()[:10]
}

// GetPeers returns peers who have announced having infoHash.
func (dht *DHT) GetPeers(infoHash string) ([]*peer, error) {
	if !dht.Ready {
		return nil, errors.New("dht not ready")
	}

	if len(infoHash) == 40 {
		data, err := hex.DecodeString(infoHash)
		if err != nil {
			return nil, err
		}
		infoHash = string(data)
	}

	peers := dht.peersManager.GetPeers(infoHash, dht.K)
	if len(peers) != 0 {
		return peers, nil
	}

	ch := make(chan struct{})

	go func() {
		neighbors := dht.routingTable.GetNeighbors(
			newBitmapFromString(infoHash), dht.K)

		for _, no := range neighbors {
			dht.transactionManager.getPeers(no, infoHash)
		}

		i := 0
		for _ = range time.Tick(time.Second * 1) {
			i++
			peers = dht.peersManager.GetPeers(infoHash, dht.K)
			if len(peers) != 0 || i == 30 {
				break
			}
		}

		ch <- struct{}{}
	}()

	<-ch
	return peers, nil
}

// Run starts the dht.
func (dht *DHT) Run() {
	var pkt packet

	packetChan := make(chan packet)
	tick := time.Tick(dht.CheckKBucketPeriod)

	dht.init()
	dht.listen(packetChan)
	dht.join()

	dht.Ready = true

	for {
		select {
		case pkt = <-packetChan:
			go handle(dht, pkt)
		case <-tick:
			if dht.routingTable.cachedNodes.Len() == 0 {
				go dht.join()
			} else if dht.transactionManager.len() == 0 {
				go dht.routingTable.Fresh()
			}
		}
	}
}
