package ingestworker

import (
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

func BenchmarkBuildTelemetryEnvelope(b *testing.B) {
	b.ReportAllocs()

	assignment := controlplane.IngestAssignment{
		Provider:         "ecoflow",
		ProviderDeviceID: "DEMOD2M00001057",
		CredentialID:     "cred-1",
		DeviceID:         "device-1",
	}
	message := ecoflowmqtt.Message{
		Topic: "/open/open-demo/DEMOD2M00001057/quota",
		Payload: []byte(`{
			"moduleType":1,
			"needAck":0,
			"id":8222878,
			"time":17108342,
			"params":{"XT150Watts2":-100,"wattsInSum":100,"icoBytes":[0,8,136,0,128,0,0,0,0,0,0,0,0,0]},
			"version":"1.0",
			"typeCode":"pdStatus",
			"addr":"hs_yj751_pd_appshow_addr",
			"cmdId":1,
			"cmdFunc":2
		}`),
	}
	cfg := EcoFlowSessionConfig{ShardCount: 128}
	observedAt := time.UnixMilli(1771119926522).UTC()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := buildTelemetryEnvelope(assignment, message, observedAt, cfg)
		if err != nil {
			b.Fatalf("buildTelemetryEnvelope failed: %v", err)
		}
	}
}
