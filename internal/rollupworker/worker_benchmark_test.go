package rollupworker

import (
	"context"
	"log/slog"
	"testing"

	"google.golang.org/protobuf/proto"
)

func BenchmarkProcessDeliverySuccess(b *testing.B) {
	store := &fakeStore{}
	worker := &Worker{log: slog.Default(), store: store, cfg: DefaultConfig()}
	data, err := proto.Marshal(testWorkerEnvelope(`{"params":{"wattsOutSum":35,"soc":54.2}}`))
	if err != nil {
		b.Fatalf("marshal envelope: %v", err)
	}
	delivery := &fakeDelivery{subject: "pulse.telemetry.ingest.s001", data: data}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		delivery.acked, delivery.nacked, delivery.termed = 0, 0, 0
		if err := worker.processDelivery(context.Background(), delivery); err != nil {
			b.Fatalf("processDelivery failed: %v", err)
		}
	}
}
