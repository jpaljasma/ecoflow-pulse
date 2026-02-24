package telemetrybus

import (
	"context"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/nats-io/nats.go"
)

func BenchmarkPublishEnvelope(b *testing.B) {
	b.ReportAllocs()

	envelope := &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "0190f4b2-a336-7aa8-b9fa-98c7d5aa8e6b",
		EnvelopeVersion:    1,
		DeviceId:           "device-1",
		EcoflowSn:          "R351ZABAPH331057",
		Shard:              7,
		ShardCount:         128,
		MessageId:          "8222878",
		DeviceTimeUnixMs:   1771119926522,
		ObservedTimeUnixMs: time.Now().UnixMilli(),
		IngestedTimeUnixMs: time.Now().UnixMilli(),
		TypeCode:           "pdStatus",
		PayloadType:        "ecoflow.mqtt.raw",
		PayloadVersion:     1,
		PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		Payload:            []byte(`{"cmdId":1,"cmdFunc":2,"typeCode":"pdStatus","params":{"wattsInSum":100}}`),
		Labels: map[string]string{
			"provider":           "ecoflow",
			"provider_device_id": "R351ZABAPH331057",
			"credential_id":      "cred-1",
			"device_id":          "device-1",
			"cmd_id":             "1",
			"cmd_func":           "2",
		},
	}

	cases := []struct {
		name        string
		stripLabels bool
	}{
		{name: "labels_on", stripLabels: false},
		{name: "labels_off", stripLabels: true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			cfg := SubjectConfig{
				Prefix:     "pulse",
				ShardCount: 128,
			}
			publisher := newNATSEnvelopePublisherForTest(
				cfg,
				func(_ *nats.Msg) error { return nil },
			)
			publisher.options.StripLabels = tc.stripLabels

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := publisher.PublishEnvelope(ctx, envelope); err != nil {
					b.Fatalf("publish failed: %v", err)
				}
			}
		})
	}
}
