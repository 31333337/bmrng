package gateway

import (
	"bytes"
	"reflect"
	"testing"

	gatewayv1 "https://github.com/31333337/trellis/pb/gen/proto/go/gateway/v1"
)

func TestMessageQueue(t *testing.T) {
	v1 := []byte("test1")
	v2 := []byte("test2")

	q := NewMessageQueue()

	q.Enqueue(v1)
	q.Enqueue(v2)

	m1, err := q.Dequeue()
	if !bytes.Equal(m1, v1) {
		t.Log("bytes not equal")
		t.FailNow()
	}

	m2, err := q.Dequeue()
	if !bytes.Equal(m2, v2) {
		t.Log("bytes not equal")
		t.FailNow()
	}

	_, err = q.Dequeue()
	if err == nil {
		t.Log("expected queue to be empty")
		t.FailNow()
	}
}

func tPacketPack(t *testing.T, success bool, id uint64, sequence uint64, data []byte) {
	dataLength := uint32(len(data))

	packet := &gatewayv1.Packet{
		Type:     gatewayv1.PacketType_PACKET_TYPE_DATA,
		StreamId: id,
		Sequence: sequence,
		Length:   dataLength,
		Data:     data,
	}

	packed, err := packetPack(packet)

	if !success {
		if err == nil {
			t.Log("should fail")
			t.FailNow()
		}
		return
	}

	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	if len(packed) != int(messageSize) {
		t.Log(err)
		t.Log("packed packet not equal to message size")
		t.FailNow()
	}
}

func TestPacketPack(t *testing.T) {
	messageSize = 64

	// should pass
	tPacketPack(t, true, 0, 0, nil)
	tPacketPack(t, true, 1, 1, []byte("1"))
	tPacketPack(t, true, 1000000000, 1000000000, []byte("1234567890"))

	// should fail
	messageSize = 32
	tPacketPack(t, false, 1000000000, 1000000000, []byte("1234567890"))
}

func TestPacketUnpack(t *testing.T) {
	messageSize = 64

	ptype := gatewayv1.PacketType_PACKET_TYPE_START
	streamid := uint64(100)
	sequence := uint64(100)
	data := []byte("1234")
	length := uint32(len(data))

	p1 := &gatewayv1.Packet{
		Type:     ptype,
		StreamId: streamid,
		Sequence: sequence,
		Length:   length,
		Data:     data,
	}

	packed, err := packetPack(p1)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	p2, err := packetUnpack(packed)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	if ptype != p2.Type {
		t.Log("packet Type not equal")
		t.FailNow()
	}

	if streamid != p2.StreamId {
		t.Log("packet StreamId not equal")
		t.FailNow()
	}

	if sequence != p2.Sequence {
		t.Log("packet Sequence not equal")
		t.FailNow()
	}

	if length != p2.Length {
		t.Log("packet Length not equal")
		t.FailNow()
	}

	if !bytes.Equal(data, p2.Data) {
		t.Log("packet Data not equal")
		t.FailNow()
	}
}

func TestPacketsSort(t *testing.T) {
	var packets = []*gatewayv1.Packet{
		{
			StreamId: 20,
			Sequence: 0,
		},
		{
			StreamId: 10,
			Sequence: 2,
		},
		{
			StreamId: 10,
			Sequence: 1,
		},
		{
			StreamId: 20,
			Sequence: 1,
		},
	}

	var sorted = []*gatewayv1.Packet{
		{
			StreamId: 10,
			Sequence: 1,
		},
		{
			StreamId: 10,
			Sequence: 2,
		},
		{
			StreamId: 20,
			Sequence: 0,
		},
		{
			StreamId: 20,
			Sequence: 1,
		},
	}

	sortPackets(packets)

	if !reflect.DeepEqual(packets, sorted) {
		t.Log("packets not sorted")
		t.FailNow()
	}
}
