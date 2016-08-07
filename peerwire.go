package dht

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"time"
)

const (
	REQUEST = iota
	DATA
	REJECT
)

const (
	BLOCK             = 16384
	MAX_METADATA_SIZE = BLOCK * 1000
	EXTENDED          = 20
	HANDSHAKE         = 0
)

var handshakePrefix = []byte{
	19, 66, 105, 116, 84, 111, 114, 114, 101, 110, 116, 32, 112, 114,
	111, 116, 111, 99, 111, 108, 0, 0, 0, 0, 0, 16, 0, 1,
}

func read(conn *net.TCPConn, size int, data *bytes.Buffer) (err error) {
	buffer := make([]byte, size)
	if _, err = io.ReadFull(conn, buffer); err != nil {
		return
	}

	n, err := data.Write(buffer)
	if err != nil || n != size {
		return errors.New("read error")
	}
	return
}

// readMessage gets a message from the tcp connection.
func readMessage(conn *net.TCPConn, data *bytes.Buffer) (
	length int, err error) {

	if err = read(conn, 4, data); err != nil {
		return
	}

	length = int(bytes2int(data.Next(4)))
	if length == 0 {
		return
	}

	if err = read(conn, length, data); err != nil {
		return
	}
	return
}

// sendMessage sends data to the connection.
func sendMessage(conn *net.TCPConn, data []byte) error {
	var length int32 = int32(len(data))

	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, length)

	_, err := conn.Write(append(buffer.Bytes(), data...))
	return err
}

func sendHandshake(conn *net.TCPConn, infoHash, peerId []byte) error {
	data := make([]byte, 68)
	copy(data[:28], handshakePrefix)
	copy(data[28:48], infoHash)
	copy(data[48:], peerId)

	_, err := conn.Write(data)
	return err
}

func onHandshake(data []byte) (err error) {
	if !(bytes.Equal(handshakePrefix[:20], data[:20]) && data[25]&0x10 != 0) {
		err = errors.New("invalid handshake response")
	}
	return
}

func sendExtHandshake(conn *net.TCPConn) error {
	data := append(
		[]byte{EXTENDED, HANDSHAKE},
		Encode(map[string]interface{}{
			"m": map[string]interface{}{"ut_metadata": 1},
		})...,
	)

	return sendMessage(conn, data)
}

// getUTMetaSize returns the ut_metadata and metadata_size.
func getUTMetaSize(data []byte) (
	utMetadata int, metadataSize int, err error) {

	v, err := Decode(data)
	if err != nil {
		return
	}

	dict, ok := v.(map[string]interface{})
	if !ok {
		err = errors.New("invalid dict")
		return
	}

	if err = parseKeys(
		dict, [][]string{{"metadata_size", "int"}, {"m", "map"}}); err != nil {
		return
	}

	m := dict["m"].(map[string]interface{})
	if err = parseKey(m, "ut_metadata", "int"); err != nil {
		return
	}

	utMetadata = m["ut_metadata"].(int)
	metadataSize = dict["metadata_size"].(int)

	if metadataSize > MAX_METADATA_SIZE {
		err = errors.New("metadata_size too long")
	}
	return
}

// Request represents the request context.
type Request struct {
	InfoHash []byte
	IP       string
	Port     int
}

// Response contains the request context and the metadata info.
type Response struct {
	Request
	MetadataInfo []byte
}

// Wire represents the wire protocol.
type Wire struct {
	blackList *blackList
	queue     *syncedMap
	requests  chan Request
	responses chan Response
}

// NewWire returns a Wire pointer.
func NewWire() *Wire {
	return &Wire{
		blackList: newBlackList(),
		queue:     newSyncedMap(),
		requests:  make(chan Request, 256),
		responses: make(chan Response, 256),
	}
}

// Request pushes the request to the queue.
func (wire *Wire) Request(infoHash []byte, ip string, port int) {
	wire.requests <- Request{InfoHash: infoHash, IP: ip, Port: port}
}

// Response returns a chan of Response.
func (wire *Wire) Response() <-chan Response {
	return wire.responses
}

// isDone returns whether the wire get all pieces of the metadata info.
func (wire *Wire) isDone(pieces [][]byte) bool {
	for _, piece := range pieces {
		if len(piece) == 0 {
			return false
		}
	}
	return true
}

func (wire *Wire) requestPieces(
	conn *net.TCPConn, utMetadata int, metadataSize int, piecesNum int) {

	buffer := make([]byte, 1024)
	for i := 0; i < piecesNum; i++ {
		buffer[0] = EXTENDED
		buffer[1] = byte(utMetadata)

		msg := Encode(map[string]interface{}{
			"msg_type": REQUEST,
			"piece":    i,
		})

		length := len(msg) + 2
		copy(buffer[2:length], msg)

		sendMessage(conn, buffer[:length])
	}
}

// fetchMetadata fetchs medata info accroding to infohash from dht.
func (wire *Wire) fetchMetadata(r Request) {
	defer func() {
		recover()
	}()

	infoHash := r.InfoHash
	address := genAddress(r.IP, r.Port)

	dial, err := net.DialTimeout("tcp", address, time.Second*15)
	if err != nil {
		wire.blackList.insert(r.IP, r.Port)
		return
	}
	conn := dial.(*net.TCPConn)
	conn.SetLinger(0)
	defer conn.Close()

	data := bytes.NewBuffer(nil)
	data.Grow(BLOCK)

	if sendHandshake(conn, infoHash, []byte(randomString(20))) != nil ||
		read(conn, 68, data) != nil ||
		onHandshake(data.Next(68)) != nil ||
		sendExtHandshake(conn) != nil {
		//fmt.Println("send hand shake error")
		return
	}

	var (
		length       int
		msgType      byte
		pieces       [][]byte
		piecesNum    int
		utMetadata   int
		metadataSize int
	)

	for {
		length, err = readMessage(conn, data)
		if err != nil {
			return
		}

		if length == 0 {
			continue
		}

		msgType, err = data.ReadByte()
		if err != nil {
			return
		}

		switch msgType {
		case EXTENDED:
			extendedId, err := data.ReadByte()
			if err != nil {
				return
			}

			payload, err := ioutil.ReadAll(data)
			if err != nil {
				return
			}

			if extendedId == 0 {
				if pieces != nil {
					return
				}

				utMetadata, metadataSize, err = getUTMetaSize(payload)
				if err != nil {
					return
				}

				piecesNum = metadataSize / BLOCK
				if metadataSize%BLOCK != 0 {
					piecesNum++
				}

				pieces = make([][]byte, piecesNum)
				go wire.requestPieces(conn, utMetadata, metadataSize, piecesNum)

				continue
			}

			if pieces == nil {
				return
			}

			d, index, err := DecodeDict(payload, 0)
			if err != nil {
				return
			}
			dict := d.(map[string]interface{})

			if err = parseKeys(dict, [][]string{
				{"msg_type", "int"},
				{"piece", "int"}}); err != nil {
				return
			}

			if dict["msg_type"].(int) != DATA {
				continue
			}

			piece := dict["piece"].(int)
			pieceLen := length - 2 - index

			if (piece != piecesNum-1 && pieceLen != BLOCK) ||
				(piece == piecesNum-1 && pieceLen != metadataSize%BLOCK) {
				return
			}

			pieces[piece] = payload[index:]

			if wire.isDone(pieces) {
				metadataInfo := bytes.Join(pieces, nil)

				info := sha1.Sum(metadataInfo)
				if !bytes.Equal(infoHash, info[:]) {
					return
				}

				wire.responses <- Response{
					Request:      r,
					MetadataInfo: metadataInfo,
				}
				return
			}
		default:
			data.Reset()
		}
	}
}

// Run starts the peer wire protocol.
func (wire *Wire) Run() {
	go wire.blackList.clear()

	var r Request
	for {
		r = <-wire.requests

		key := strings.Join([]string{
			string(r.InfoHash), genAddress(r.IP, r.Port),
		}, ":")

		if len(r.InfoHash) != 20 || wire.blackList.in(r.IP, r.Port) ||
			wire.queue.Has(key) {
			continue
		}

		go wire.fetchMetadata(r)
	}
}
