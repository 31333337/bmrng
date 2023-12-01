package gateway

import (
	"bytes"
	"container/list"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	gatewayv1 "github.com/31333337/bmrng/api/gen/proto/go/gateway/v1"
	"github.com/31333337/bmrng/go/0kn/pkg/logger"
	"google.golang.org/protobuf/proto"
)

// A gateway:
// - receives incoming data as streams to transmit through the mix-net
// - enables mix-net clients to retrieve messages to transmit
// - receives final messages from the mix-net
// - serves outoing data streams

// Enable gateway? Used externally to conditionally use the gateway
var Enable bool = false

// mix-net message size; initialization required
var messageSize int64 = 0

// packet protocol size; calculated and cached by GetMaxProtocolSize
var protocolSize int64 = 0

// Gateway initialization; set configuration and start in/out proxies
// Set msgSize to match the mix-net messageSize.
// Explicitly enable the gateway.
// addrIn: proxy server address and port for receiving incoming data
// addrOut server address and port for serving outgoing data
func Init(msgSize int64, enable bool, addrIn string, addrOut string) {
	messageSize = msgSize
	Enable = enable

	if !enable {
		return
	}

	// data in -> proxy -> gateway -> mix-net
	go proxyStart(addrIn)

	// mix-net -> gateway -> http server -> data out
	go httpServerStart(addrOut)
}

// Max gateway packet protocol size in bytes, not counting packet data
// This is a good estimate as protobuf serialized message size are inherently variant
// The calculated value is cached
func GetMaxProtocolSize() int64 {
	if protocolSize > 0 {
		// return cached result
		return protocolSize
	}

	// Create a packet header with max data sizes, marshal it, then measure the bytes consumption
	// For message wire type sizes, refer to https://protobuf.dev/programming-guides/encoding/#bools-and-enums

	header := &gatewayv1.PacketHeader{
		Type:     gatewayv1.PacketType_PACKET_TYPE_DUMMY,
		StreamId: math.MaxUint64,
		Sequence: math.MaxUint64,
	}

	packedHeader, err := proto.Marshal(header)
	if err != nil {
		panic(err)
	}

	staticSize := int64(2 + 4) // packet header length (uint16) + data length (uint32)
	variableSize := int64(len(packedHeader))
	protocolSize = staticSize + variableSize

	return protocolSize
}

// The max size of message packet data accounting for protocol size
func GetMaxDataSize() int64 {
	return messageSize - GetMaxProtocolSize()
}

////////////////////////////////////////////////////////////////////////
// Message Queues
// - FIFO queues are maintained for messages In and Out of the Gateway
// - incoming gateway data streams are packetized and packed as messages
// - mix-net clients retrieve the next message for each lightning round
////////////////////////////////////////////////////////////////////////

// A message queue for each client
type MessageQueue struct {
	items *list.List
	mutex sync.Mutex
}

func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		items: list.New(),
	}
}

// Incoming message queue
// Stores packetized messages as they are received by the gateway proxy
// then made available to mix-clients for transmission each round
var msgQueueIn = NewMessageQueue()

// Outgoing queues, mapped by stream id, for data leaving the mix-net
var streamOut = make(map[uint64]*MessageQueue)
var streamOutMu sync.Mutex

// Track packet stream state by id as they leave the mix-net
const (
	STREAM_OUT_START = iota // stream is transmitting through mix-net
	STREAM_OUT_END          // stream has exited mix-net completely
)

var streamOutState = make(map[uint64]int)
var streamOutStateMu sync.Mutex

// Enqueue data into the message queue
func (q *MessageQueue) Enqueue(m []byte) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.items.PushBack(m)
}

// Dequeue data from the queue at the given index
func (q *MessageQueue) Dequeue() ([]byte, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if element := q.items.Front(); element != nil {
		// Pop the front element from the queue
		message := element.Value.([]byte)
		q.items.Remove(element)
		return message, nil
	}

	return nil, errors.New("queue is empty")
}

////////////////////////////////////////////////////////////////////////
// Packets & Message Serialization
// - a protocol to transmit data streams as packetized mix-net messages
////////////////////////////////////////////////////////////////////////

// Given a packet header, prepare a mix-net message buffer for adding data
// Returns a buffer and number of bytes remaining for writing data
func prepareMessageBuffer(header *gatewayv1.PacketHeader) (*bytes.Buffer, int64, error) {
	packedHeader, err := proto.Marshal(header)
	if err != nil {
		return nil, 0, err
	}

	buffer := new(bytes.Buffer)

	// serialize packed header length in the message
	headerLength := uint16(len(packedHeader))
	if err := binary.Write(buffer, binary.LittleEndian, headerLength); err != nil {
		return nil, 0, fmt.Errorf("Error serializing header length: %w", err)
	}

	// serialize header in the message
	buffer.Write(packedHeader)

	// sizeof uint32 in bytes for recording data length
	var dataLengthSize int64 = 4

	// space remaining for data
	space := messageSize - int64(len(buffer.Bytes())) - dataLengthSize

	return buffer, space, nil
}

func writeMessageBuffer(buffer *bytes.Buffer, space int64, data []byte) error {
	dataLength := uint32(len(data))

	// serialize data length in the message
	binary.Write(buffer, binary.LittleEndian, dataLength)

	// serialize data in the message
	buffer.Write(data)

	pad := space - int64(dataLength)

	if pad < 0 {
		return errors.New("data too large for message size")
	}

	if pad > 0 {
		// pad the remaining space to match message size
		buffer.Write(make([]byte, pad))
	}

	return nil
}

// Pack a packet header and packet data into a mix-net message of the target message size.
// Use prepareMessageBuffer and writeMessageBuffer separately to determine exactly
// how much space is available for message data.
func packetPack(header *gatewayv1.PacketHeader, data []byte) ([]byte, error) {
	buffer, space, err := prepareMessageBuffer(header)
	err = writeMessageBuffer(buffer, space, data)
	return buffer.Bytes(), err
}

// Unpack a mix-net message into a packet header and packet data
func packetUnpack(message []byte) (*gatewayv1.PacketHeader, []byte, error) {
	buffer := bytes.NewReader(message)

	// deserialize uint16 for headerLength
	var headerLength uint16
	if err := binary.Read(buffer, binary.LittleEndian, &headerLength); err != nil {
		return nil, nil, fmt.Errorf("Error deserializing header length: %w", err)
	}

	// deserialize packet header from headerLength
	headerBytes := make([]byte, headerLength)
	buffer.Read(headerBytes)
	header := &gatewayv1.PacketHeader{}
	err := proto.Unmarshal(headerBytes, header)
	if err != nil {
		return nil, nil, fmt.Errorf("Error deserializing packet header: %w", err)
	}

	// deserialize uint32 for dataLength
	var dataLength uint32
	if err := binary.Read(buffer, binary.LittleEndian, &dataLength); err != nil {
		return nil, nil, fmt.Errorf("Error deserializing data length: %w", err)
	}

	// deserialize data from dataLength
	data := make([]byte, dataLength)
	buffer.Read(data)

	return header, data, nil
}

////////////////////////////////////////////////////////////////////////
// Trellis mix-net hooks
// - called within mix-net simulator
// - round start: clients get messages from the In queue
// - round end: final messages are checked and placed in the Out queue
////////////////////////////////////////////////////////////////////////

// Get the next message for a client to send through the mix-net
func GetMessageForClient(clientId int64) ([]byte, error) {
	message, err := msgQueueIn.Dequeue()

	// If message queue is empty, use a dummy message
	if err != nil {
		header := &gatewayv1.PacketHeader{
			Type:     gatewayv1.PacketType_PACKET_TYPE_DUMMY,
			StreamId: uint64(clientId), // use client id for uniqueness within round
			Sequence: 0,
		}
		// message, err = packetPack(header)
		buffer, space, err := prepareMessageBuffer(header)
		if err == nil {
			if err = writeMessageBuffer(buffer, space, nil); err == nil {
				message = buffer.Bytes()
			}
		}
	}

	return message, err
}

// Final messages from a mix-net round are sent here.
// This replaces `coordinator.Check` testing for message ids.
// Note: There are duplicates (why?), so sort unique.
func CheckFinalMessages(messages [][]byte, numExpected int) bool {
	defer logger.Sugar.Sync()

	// get a packet identifier that is unique among all packets of a round
	getPacketUID := func(h *gatewayv1.PacketHeader) uint64 {
		// For data packets: `StreamId` is expected to be universally unique to the input stream
		// For dummy packets: `StreamId` is only unique within a single round
		return h.StreamId + h.Sequence
	}

	// count unique messages and extract unique non-dummy packets
	uniqueMessages := make(map[uint64][]byte) // record message data by uid for access post-sort
	var uniquePackets []*gatewayv1.PacketHeader
	for _, m := range messages {
		packet, data, err := packetUnpack(m)
		if err != nil {
			panic(err)
		}

		uid := getPacketUID(packet)

		// if message is unique
		if _, seen := uniqueMessages[uid]; !seen {
			// mark message as seen
			uniqueMessages[uid] = data

			// if not a dummy, collect its packet
			if packet.Type != gatewayv1.PacketType_PACKET_TYPE_DUMMY {
				uniquePackets = append(uniquePackets, packet)
			}
		}
	}

	sortPacketHeaders(uniquePackets)

	// for unique sorted non-dummy packets, store data as gateway output messages for each stream
	streamOutMu.Lock()
	defer streamOutMu.Unlock()
	for _, p := range uniquePackets {
		id := p.StreamId

		switch p.Type {
		case gatewayv1.PacketType_PACKET_TYPE_START:
			logger.Sugar.Debugf("[Gateway] <<< [mix-net] 🟢 START stream [%d]", id)
			streamOut[id] = NewMessageQueue()
			streamOutStateMu.Lock()
			streamOutState[id] = STREAM_OUT_START
			streamOutStateMu.Unlock()

		case gatewayv1.PacketType_PACKET_TYPE_DATA:
			logger.Sugar.Debugf("[Gateway] <<< [mix-net] 🔶 DATA stream [%d][%d]", id, p.Sequence)
			uid := getPacketUID(p)
			streamOut[id].Enqueue(uniqueMessages[uid])

		case gatewayv1.PacketType_PACKET_TYPE_END:
			logger.Sugar.Debugf("[Gateway] <<< [mix-net] 🟥 END stream [%d]", id)
			streamOutStateMu.Lock()
			streamOutState[id] = STREAM_OUT_END
			streamOutStateMu.Unlock()
		}
	}

	// compare unique messages with number expected
	return len(uniqueMessages) == numExpected
}

// Sort packet headers by StreamId, Sequence
func sortPacketHeaders(headers []*gatewayv1.PacketHeader) {
	sort.Slice(headers, func(i, j int) bool {
		pi, pj := headers[i], headers[j]
		switch {
		case pi.StreamId != pj.StreamId:
			return pi.StreamId < pj.StreamId
		default:
			return pi.Sequence < pj.Sequence
		}
	})
}

////////////////////////////////////////////////////////////////////////
// Gateway Packet I/O
////////////////////////////////////////////////////////////////////////

////////////////////////////////////
// Gateway Data Input
////////////////////////////////////

// Start gateway proxy to listen for incoming messages
func proxyStart(addrIn string) {
	defer logger.Sugar.Sync()

	// Create a listener for incoming connections
	listener, err := net.Listen("tcp", addrIn)
	if err != nil {
		logger.Sugar.Errorf("[Gateway] >>> Error listening: %v", err)
		return
	}
	defer listener.Close()

	logger.Sugar.Infof("[Gateway] >>> Listening on %s...", addrIn)

	// Accept incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Sugar.Errorf("[Gateway] >>> Error accepting connection: %v", err)
			continue
		}

		go proxyHandleConnection(conn)
	}
}

func proxyHandleConnection(conn net.Conn) {
	defer conn.Close()
	defer logger.Sugar.Sync()

	streamId := getStreamId()
	var packetCounter uint64 = 0

	logger.Sugar.Debugf(
		"[Gateway] >>> Accepted connection from %s id=%d",
		conn.RemoteAddr(),
		streamId,
	)

	// Send a message through the mix-net
	sendMessage := func(h *gatewayv1.PacketHeader, dataOrMessage []byte, packed bool) {
		var err error
		message := dataOrMessage

		if !packed {
			// pack the packet into a static-sized message
			message, err = packetPack(h, dataOrMessage)

			if err != nil {
				// TODO: handle error less fatally
				logger.Sugar.Fatalf("[Gateway] >>> Error creating message from packet: %v", err)
			}
		}

		logger.Sugar.Debugf("[Gateway] >>> [mix-net] Send stream [%d][%d]", h.StreamId, h.Sequence)

		// stage message for a mix-net client to pick it up
		msgQueueIn.Enqueue(message)

		// increment packet sequence counter
		packetCounter++
	}

	// start a new transmission
	header := &gatewayv1.PacketHeader{
		Type:     gatewayv1.PacketType_PACKET_TYPE_START,
		StreamId: streamId,
		Sequence: 0,
	}
	sendMessage(header, nil, false)

	for {
		// send packetized data
		header := &gatewayv1.PacketHeader{
			Type:     gatewayv1.PacketType_PACKET_TYPE_DATA,
			StreamId: streamId,
			Sequence: packetCounter,
		}

		// prepare message buffer, determine space remaining for data
		messageBuffer, space, _ := prepareMessageBuffer(header)

		// read at most "space" bytes from stream, record number ("n") of bytes actually read
		data := make([]byte, space)
		n, err := conn.Read(data)

		if err != nil {
			// end transmission
			if err == io.EOF {
				header.Type = gatewayv1.PacketType_PACKET_TYPE_END
				logger.Sugar.Debugf("[Gateway] >>> Finished receiving data stream id=%d", streamId)

			} else {
				header.Type = gatewayv1.PacketType_PACKET_TYPE_ERROR
				logger.Sugar.Debugf("[Gateway] >>> Error receiving data stream id=%d: %s", streamId, err.Error())
			}

			sendMessage(header, nil, false)
			return
		}

		err = writeMessageBuffer(messageBuffer, space, data[:n])
		sendMessage(header, messageBuffer.Bytes(), true)
	}
}

// Get a unique id for a mix-net transmission stream
// TODO?: use UUID module
func getStreamId() uint64 {
	r := make([]byte, 8)
	rand.Read(r)
	return binary.BigEndian.Uint64(r)
}

////////////////////////////////////
// Gateway Data Output
////////////////////////////////////

// An HTTP server to serve data output
// conceptual placeholder to get mixed data out of the gateway in lieu of more protocol
func httpServerStart(addrOut string) {
	defer logger.Sugar.Sync()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var id uint64 = 0

		for {
			// get a completed data stream id, if there is one
			streamOutStateMu.Lock()
			for k, v := range streamOutState {
				if v == STREAM_OUT_END {
					id = k
				}
				break
			}
			streamOutStateMu.Unlock()

			if id != 0 {
				break
			}

			// if no completed data streams, get a transmitting one
			streamOutStateMu.Lock()
			for k, v := range streamOutState {
				if v == STREAM_OUT_START {
					id = k
				}
				break
			}
			streamOutStateMu.Unlock()

			if id != 0 {
				break
			}

			// wait until there is a stream
			logger.Sugar.Debugf("[Gateway] <<< Waiting for stream...")
			time.Sleep(time.Duration(40) * time.Millisecond)
		}

		logger.Sugar.Debugf(
			"[Gateway] <<< stream leaving gateway id=%d, number of streams=%d",
			id,
			len(streamOutState),
		)

		if id != 0 {
			for {
				// remove data from queue until empty
				data, err := streamOut[id].Dequeue()
				if err != nil {
					streamOutStateMu.Lock()
					state := streamOutState[id]
					streamOutStateMu.Unlock()

					if state == STREAM_OUT_END {
						break
					} else if state == STREAM_OUT_START {
						// if stream has not finished transmitting, wait for more data to exit the mix-net
						logger.Sugar.Debugf("[Gateway] <<< Waiting for stream data to exit mix-net...")
						time.Sleep(time.Duration(10) * time.Millisecond)
						continue
					}
				}

				// send data out the http conection
				fmt.Fprint(w, string(data))
			}

			// when all data for the stream has exited the gateway, remove stream queue and tracking

			streamOutMu.Lock()
			delete(streamOut, id)
			streamOutMu.Unlock()

			streamOutStateMu.Lock()
			delete(streamOutState, id)
			streamOutStateMu.Unlock()

			return
		}

		http.NotFound(w, r)
	})

	// Start the HTTP server
	err := http.ListenAndServe(addrOut, nil)
	if err != nil {
		logger.Sugar.Fatalw("[Gateway] <<< Failed to start proxy out",
			"address", addrOut,
			"error", err,
		)
	}

	logger.Sugar.Infof("[Gateway] <<< Listening on %s...", addrOut)
}
