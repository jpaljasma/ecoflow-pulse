package projectionworker

import (
	"context"
	"testing"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func BenchmarkValkeySnapshotStoreApplyEnvelope(b *testing.B) {
	store := setupSnapshotStore(b)
	env := &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-bench",
		DeviceId:           "dev-bench",
		EcoflowSn:          "DEMOD2M00001057",
		IngestedTimeUnixMs: 1234,
		Payload:            []byte(`{"params":{"wattsOutSum":35,"soc":54.2}}`),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.EnvelopeId = "env-bench"
		env.IngestedTimeUnixMs = int64(i + 1)
		if _, err := store.ApplyEnvelope(context.Background(), env); err != nil {
			b.Fatalf("ApplyEnvelope failed: %v", err)
		}
	}
}
