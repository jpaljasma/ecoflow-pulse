package ecoflowmqtt

import (
	"encoding/binary"
	"testing"
)

func BenchmarkParsePublish(b *testing.B) {
	cases := []struct {
		name  string
		flags byte
		body  []byte
	}{
		{
			name:  "qos0_small_payload",
			flags: 0x00,
			body:  benchmarkPublishBody("open/account/device/quota", []byte(`{"id":8222878,"params":{"wattsInSum":100}}`), 0),
		},
		{
			name:  "qos1_large_payload",
			flags: 0x02,
			body: benchmarkPublishBody(
				"dt/anker_power/A1783/SN-C2000/param_info",
				[]byte(`{"payload":"{\"pn\":\"A1783\",\"sn\":\"SN-C2000\",\"data\":\"/wkAAAABDwQhAqJTTi1DMjAwpQGXpQ==\"}","head":{"device_pn":"A1783","device_sn":"SN-C2000"}}`),
				77,
			),
		},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				msg, _, err := parsePublish(tc.flags, tc.body)
				if err != nil {
					b.Fatalf("parsePublish() error = %v", err)
				}
				if len(msg.Payload) == 0 {
					b.Fatal("empty payload")
				}
			}
		})
	}
}

func benchmarkPublishBody(topic string, payload []byte, packetID uint16) []byte {
	body := make([]byte, 0, 2+len(topic)+2+len(payload))
	body = appendMQTTString(body, topic)
	if packetID != 0 {
		pid := make([]byte, 2)
		binary.BigEndian.PutUint16(pid, packetID)
		body = append(body, pid...)
	}
	body = append(body, payload...)
	return body
}
