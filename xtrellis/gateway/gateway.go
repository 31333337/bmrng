package gateway

import (
	"bytes"
	"container/list"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	gatewayv1 "https://github.com/31333337/trellis/pb/gen/proto/go/gateway/v1"
	"https://github.com/31333337/trellis/xtrellis/utils"
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

	// Create a packet with max data sizes, marshal it, then measure the length minus data size.
	// For message wire type sizes, refer to https://protobuf.dev/programming-guides/encoding/#bools-and-enums

	packet := &gatewayv1.Packet{
		Type:     gatewayv1.PacketType_PACKET_TYPE_DUMMY,
		StreamId: math.MaxUint64,
		Sequence: math.MaxUint64,
		Length:   math.MaxUint32,
		Data:     []byte("----"), // not nil
	}

	packed, err := proto.Marshal(packet)
	if err != nil {
		panic(err)
	}

	protocolSize = int64(len(packed) - len(packet.Data))
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

// Get space in bytes for the packed packet to meet message size
func getPacketSpace(packet *gatewayv1.Packet) (int64, error) {
	packed, err := proto.Marshal(packet)
	if err != nil {
		return 0, err
	}

	// bytes of space to fill
	space := messageSize - int64(len(packed))

	// if packet data is empty
	if space > 0 && bytes.Equal(packet.Data, []byte("")) {
		// account for 2 protocol bytes when data is not empty
		space -= 2
	}

	return space, nil
}

// Pack a packet by filling with space to meet the target message size
func packetPack(packet *gatewayv1.Packet) ([]byte, error) {
	space, err := getPacketSpace(packet)
	if err != nil {
		return nil, err
	}

	if space < 0 {
		return nil, errors.New("packet too large for message size")
	}

	if space > 0 {
		// fill the space used by the data,
		// the packet length enables decoding
		buffer := new(bytes.Buffer)
		buffer.Write(packet.Data)
		buffer.Write(make([]byte, space))
		packet.Data = buffer.Bytes()
	}

	packed, err := proto.Marshal(packet)
	if err != nil {
		return nil, err
	}

	return packed, nil
}

func packetUnpack(packedPacket []byte) (*gatewayv1.Packet, error) {
	packet := &gatewayv1.Packet{}
	err := proto.Unmarshal(packedPacket, packet)
	if err != nil {
		return nil, err
	}

	// deserialize padded packet data based on packet length
	buffer := bytes.NewReader(packet.Data)
	data := make([]byte, packet.Length)
	buffer.Read(data)
	packet.Data = data

	return packet, nil
}

////////////////////////////////////////////////////////////////////////
// Trellis mix-net hooks
// - called within mix-net simulator
// - round start: clients get messages from the In queue
// - round end: final messages are checked and placed in the Out queue
////////////////////////////////////////////////////////////////////////

// Get the next message for a client to send through the mix-net
// If message queue is empty, use a dummy message
func GetMessageForClient(clientId int64) ([]byte, error) {
	message, err := msgQueueIn.Dequeue()

	if err != nil {
		packet := &gatewayv1.Packet{
			Type:     gatewayv1.PacketType_PACKET_TYPE_DUMMY,
			StreamId: uint64(clientId), // use client id for uniqueness within round
			Sequence: 0,
			Length:   0,
			Data:     nil,
		}
		message, err = packetPack(packet)
	}

	return message, err
}

// Final messages from a mix-net round are sent here.
// This replaces `coordinator.Check` testing for message ids.
// Note: There are duplicates (why?), so sort unique.
func CheckFinalMessages(messages [][]byte, numExpected int) bool {
	// count unique messages and extract unique non-dummy packets
	uniqueMessages := make(map[uint64]uint64)
	var uniquePackets []*gatewayv1.Packet
	for _, m := range messages {
		packet, err := packetUnpack(m)
		if err != nil {
			panic(err)
		}

		// For data packets: `StreamId` is expected to be universally unique to the input stream
		// For dummy packets: `StreamId` is only unique within a single round
		uid := packet.StreamId + packet.Sequence

		// if message is unique
		if uniqueMessages[uid] == 0 {
			// mark message as seen
			uniqueMessages[uid] = uid

			// if not a dummy, collect its packet
			if packet.Type != gatewayv1.PacketType_PACKET_TYPE_DUMMY {
				uniquePackets = append(uniquePackets, packet)
			}
		}
	}

	sortPackets(uniquePackets)

	// for unique sorted non-dummy packets, store data as gateway output messages for each stream
	streamOutMu.Lock()
	defer streamOutMu.Unlock()
	for _, p := range uniquePackets {
		utils.DebugLog("[Gateway] data exiting mix-net [%d][%d] += '%s'", p.StreamId, p.Sequence, p.Data)
		id := p.StreamId

		switch p.Type {
		case gatewayv1.PacketType_PACKET_TYPE_START:
			streamOut[id] = NewMessageQueue()
			streamOutStateMu.Lock()
			streamOutState[id] = STREAM_OUT_START
			streamOutStateMu.Unlock()
			break

		case gatewayv1.PacketType_PACKET_TYPE_DATA:
			streamOut[id].Enqueue(p.Data)
			break

		case gatewayv1.PacketType_PACKET_TYPE_END:
			utils.DebugLog("[Gateway] 🟥 END data stream transmission [%d]", p.StreamId)
			streamOutStateMu.Lock()
			streamOutState[id] = STREAM_OUT_END
			streamOutStateMu.Unlock()
			break
		}
	}

	// compare unique messages with number expected
	return len(uniqueMessages) == numExpected
}

// Sort packets by StreamId, Sequence
func sortPackets(packets []*gatewayv1.Packet) {
	sort.Slice(packets, func(i, j int) bool {
		pi, pj := packets[i], packets[j]
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

// Send a message through the mix-net.
func sendPacket(packet *gatewayv1.Packet) {
	// pack the packet into a static-sized message
	message, err := packetPack(packet)
	if err != nil {
		// TODO: handle error less fatally
		panic(fmt.Sprintf("[Gateway] Error creating message from packet: %v\n", err))
	}

	utils.DebugLog("[Gateway] Send packet!\n🔶 %v", packet)

	// stage message for a mix-net client to pick it up
	msgQueueIn.Enqueue(message)
}

// Start gateway proxy to listen for incoming messages
func proxyStart(addrIn string) {
	// Create a listener for incoming connections
	listener, err := net.Listen("tcp", addrIn)
	if err != nil {
		log.Printf("[Gateway] Error listening: %v\n", err)
		return
	}
	defer listener.Close()

	log.Printf("[Gateway] IN: Listening on %s...\n", addrIn)

	// Accept incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[Gateway] Error accepting connection: %v\n", err)
			continue
		}

		go proxyHandleConnection(conn)
	}
}

func proxyHandleConnection(conn net.Conn) {
	defer conn.Close()

	streamId := getStreamId()
	var packetCounter uint64 = 0

	utils.DebugLog("[Gateway] IN: Accepted connection from %s id=%d", conn.RemoteAddr(), streamId)

	// start a new transmission
	packet := &gatewayv1.Packet{
		Type:     gatewayv1.PacketType_PACKET_TYPE_START,
		StreamId: streamId,
		Sequence: 0,
		Length:   0,
		Data:     nil,
	}
	sendPacket(packet)
	packetCounter++

	for {
		// TODO: more data space may be available if protocol uses variant sizes, then measure data size per-packet
		dataSize := GetMaxDataSize()

		bufferData := make([]byte, dataSize)
		n, err := conn.Read(bufferData)

		if err != nil {
			// end transmission
			packetFinal := &gatewayv1.Packet{
				StreamId: streamId,
				Sequence: packetCounter,
			}

			if err == io.EOF {
				packetFinal.Type = gatewayv1.PacketType_PACKET_TYPE_END
				utils.DebugLog("[Gateway] IN: Finished receiving data stream")

			} else {
				packetFinal.Type = gatewayv1.PacketType_PACKET_TYPE_ERROR
				utils.DebugLog("[Gateway] IN: Error receiving data stream: %s", err.Error())
			}

			sendPacket(packetFinal)
			return
		}

		// send packetized data
		packet := &gatewayv1.Packet{
			Type:     gatewayv1.PacketType_PACKET_TYPE_DATA,
			StreamId: streamId,
			Sequence: packetCounter,
			Length:   uint32(n),
			Data:     bufferData[:n],
		}
		sendPacket(packet)
		packetCounter++
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
			utils.DebugLog("[Gateway] OUT: waiting for stream...")
			time.Sleep(time.Duration(40) * time.Millisecond)
		}

		utils.DebugLog("[Gateway] OUT: streamId=%d, number of streams=%d", id, len(streamOutState))

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
						utils.DebugLog("[Gateway] OUT: waiting for stream data to exit mix-net...")
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
	log.Printf("[Gateway] OUT: Listening on %s...\n", addrOut)
	err := http.ListenAndServe(addrOut, nil)
	if err != nil {
		fmt.Println("Error:", err)
	}
}
