package ecoflowmqtt

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"testing"
)

func TestRemainingLengthCodec(t *testing.T) {
	cases := []int{0, 1, 42, 127, 128, 321, 16_384, 2_097_151, 268_435_455}
	for _, want := range cases {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		if err := writeRemainingLength(w, want); err != nil {
			t.Fatalf("writeRemainingLength(%d) error: %v", want, err)
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("flush error: %v", err)
		}
		r := bufio.NewReader(&b)
		got, err := readRemainingLength(r)
		if err != nil {
			t.Fatalf("readRemainingLength(%d) error: %v", want, err)
		}
		if got != want {
			t.Fatalf("remaining length mismatch: got=%d want=%d", got, want)
		}
	}
}

func TestParsePublishQoS1(t *testing.T) {
	topic := "a/b/c"
	payload := []byte(`{"x":1}`)
	body := make([]byte, 0, 2+len(topic)+2+len(payload))
	body = appendMQTTString(body, topic)
	packetID := uint16(77)
	pid := make([]byte, 2)
	binary.BigEndian.PutUint16(pid, packetID)
	body = append(body, pid...)
	body = append(body, payload...)

	msg, gotPacketID, err := parsePublish(0x02, body)
	if err != nil {
		t.Fatalf("parsePublish error: %v", err)
	}
	if gotPacketID != packetID {
		t.Fatalf("packet id mismatch: got=%d want=%d", gotPacketID, packetID)
	}
	if msg.Topic != topic {
		t.Fatalf("topic mismatch: got=%q want=%q", msg.Topic, topic)
	}
	if string(msg.Payload) != string(payload) {
		t.Fatalf("payload mismatch: got=%q want=%q", string(msg.Payload), string(payload))
	}
	if msg.QoS != 1 {
		t.Fatalf("qos mismatch: got=%d want=1", msg.QoS)
	}
}

func TestBuildConnectPacket(t *testing.T) {
	packet := buildConnectPacket(Config{
		ClientID:  "cid-1",
		Username:  "user",
		Password:  "pass",
		KeepAlive: 30,
	})
	if len(packet) == 0 {
		t.Fatal("buildConnectPacket returned empty packet")
	}
	if packet[0] != 0 || packet[1] != 4 {
		t.Fatalf("unexpected protocol name length bytes: %v", packet[:2])
	}
}
