package ingestworker

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

func BenchmarkLoopReconcileStartBatch(b *testing.B) {
	benchmarks := []struct {
		name         string
		assignments  int
		startWorkers int
		acquireDelay time.Duration
	}{
		{name: "5k_workers1_delay100us", assignments: 5000, startWorkers: 1, acquireDelay: 100 * time.Microsecond},
		{name: "5k_workers32_delay100us", assignments: 5000, startWorkers: 32, acquireDelay: 100 * time.Microsecond},
		{name: "10k_workers1_delay50us", assignments: 10000, startWorkers: 1, acquireDelay: 50 * time.Microsecond},
		{name: "10k_workers48_delay50us", assignments: 10000, startWorkers: 48, acquireDelay: 50 * time.Microsecond},
	}

	for _, tc := range benchmarks {
		b.Run(tc.name, func(b *testing.B) {
			store := &fakeStore{}
			store.set(makeAssignments(tc.assignments))
			leases := &fakeLeaseManager{
				denyAcquire:  true,
				acquireDelay: tc.acquireDelay,
			}
			runner := &fakeSessionRunner{}
			loop, err := NewLoop(slog.Default(), store, leases, runner, Config{
				WorkerID:     "worker-bench",
				PollInterval: time.Second,
				PollJitter:   0,
				StopTimeout:  time.Second,
				StartWorkers: tc.startWorkers,
			})
			if err != nil {
				b.Fatalf("NewLoop error: %v", err)
			}

			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := loop.reconcile(ctx); err != nil {
					b.Fatalf("reconcile failed: %v", err)
				}
			}
		})
	}
}

func makeAssignments(n int) []controlplane.IngestAssignment {
	out := make([]controlplane.IngestAssignment, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, controlplane.IngestAssignment{
			Provider:           "ecoflow",
			ProviderDeviceID:   fmt.Sprintf("SN-%d", i),
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		})
	}
	return out
}
