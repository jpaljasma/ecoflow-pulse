package gaprepair

import (
	"io"
	"log/slog"
	"testing"

	replayv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/replay/v1"
	"google.golang.org/protobuf/proto"
)

func BenchmarkWorkerHandleDeliverySuccess(b *testing.B) {
	runner := &fakeReplayRunner{}
	worker := newTestWorker(runner, DefaultWorkerConfig())
	worker.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	req := &replayv1.GapRepairRequest{
		Provider:         "ecoflow",
		ProviderDeviceId: "DEMOD2M00001057",
		FromUnixMs:       1000,
		ToUnixMs:         2000,
		MaxObjects:       17,
	}
	data, err := proto.Marshal(req)
	if err != nil {
		b.Fatalf("marshal request failed: %v", err)
	}
	delivery := &fakeGapDelivery{data: data}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		delivery.acked, delivery.nacked, delivery.termed = 0, 0, 0
		worker.handleDelivery(delivery)
	}
}
