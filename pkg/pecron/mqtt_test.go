package pecron

import (
	"bytes"
	"testing"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

func TestPecronConnectPacketMatchesPythonPahoCredentialShape(t *testing.T) {
	t.Parallel()

	packet := newPecronConnectPacket(MQTTConfig{
		Token:     "access-token",
		ClientID:  "qu_uid_1234",
		KeepAlive: 90 * time.Second,
	})

	if packet.ProtocolName != "MQTT" || packet.ProtocolVersion != 4 {
		t.Fatalf("protocol = %q/%d, want MQTT/4", packet.ProtocolName, packet.ProtocolVersion)
	}
	if !packet.CleanSession {
		t.Fatal("clean session = false, want true")
	}
	if packet.ClientIdentifier != "qu_uid_1234" {
		t.Fatalf("client id = %q", packet.ClientIdentifier)
	}
	if !packet.UsernameFlag || packet.Username != "" {
		t.Fatalf("username flag/value = %v/%q, want true/empty", packet.UsernameFlag, packet.Username)
	}
	if !packet.PasswordFlag || string(packet.Password) != "access-token" {
		t.Fatalf("password flag/value = %v/%q, want true/token", packet.PasswordFlag, string(packet.Password))
	}
	if packet.Keepalive != 90 {
		t.Fatalf("keepalive = %d, want 90", packet.Keepalive)
	}

	var encoded bytes.Buffer
	if err := packet.Write(&encoded); err != nil {
		t.Fatalf("write packet: %v", err)
	}
	decoded, err := packets.ReadPacket(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("read encoded packet: %v", err)
	}
	connect, ok := decoded.(*packets.ConnectPacket)
	if !ok {
		t.Fatalf("decoded packet = %T, want ConnectPacket", decoded)
	}
	if !connect.UsernameFlag || connect.Username != "" || !connect.PasswordFlag || string(connect.Password) != "access-token" {
		t.Fatalf("decoded credential flags/value = u:%v/%q p:%v/%q", connect.UsernameFlag, connect.Username, connect.PasswordFlag, string(connect.Password))
	}
}

func TestPecronMQTTTopicsPreserveProviderCase(t *testing.T) {
	t.Parallel()

	ref := DeviceRef{ProductKey: "p11u2Q", DeviceKey: "AABBCCDD944C"}
	wantSubscribe := []string{
		"q/2/d/qdp11u2QAABBCCDD944C/bus_",
		"q/2/d/qdp11u2QAABBCCDD944C/ack_",
		"q/2/d/qdp11u2QAABBCCDD944C/onl_",
	}
	gotSubscribe := MQTTSubscribeTopics(ref)
	if len(gotSubscribe) != len(wantSubscribe) {
		t.Fatalf("subscribe topics = %#v, want %#v", gotSubscribe, wantSubscribe)
	}
	for i := range wantSubscribe {
		if gotSubscribe[i] != wantSubscribe[i] {
			t.Fatalf("subscribe topics = %#v, want %#v", gotSubscribe, wantSubscribe)
		}
	}
	if got, want := MQTTPublishTopic(ref), "q/1/d/qdp11u2QAABBCCDD944C/bus"; got != want {
		t.Fatalf("publish topic = %q, want %q", got, want)
	}
}
