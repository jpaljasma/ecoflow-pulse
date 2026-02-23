package ingestlease

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkLeaseManagerAcquireRelease(b *testing.B) {
	_, _, manager := setupTestManager(b, DefaultConfig())
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: fmt.Sprintf("bench-acq-%d", i)}
		token := fmt.Sprintf("token-%d", i)
		result, err := manager.Acquire(ctx, ref, "worker-bench", token, CallOptions{})
		if err != nil {
			b.Fatalf("Acquire: %v", err)
		}
		if !result.Acquired {
			b.Fatalf("Acquire not granted for unique ref")
		}
		release, err := manager.Release(ctx, ref, token, CallOptions{})
		if err != nil {
			b.Fatalf("Release: %v", err)
		}
		if !release.Released {
			b.Fatalf("Release rejected: %s", release.Reason)
		}
	}
}

func BenchmarkLeaseManagerRenew(b *testing.B) {
	_, _, manager := setupTestManager(b, DefaultConfig())
	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "bench-renew"}
	token := "bench-renew-token"

	acquired, err := manager.Acquire(ctx, ref, "worker-bench", token, CallOptions{})
	if err != nil {
		b.Fatalf("Acquire: %v", err)
	}
	if !acquired.Acquired {
		b.Fatalf("Acquire not granted")
	}
	b.Cleanup(func() {
		_, _ = manager.Release(context.Background(), ref, token, CallOptions{})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renew, err := manager.Renew(ctx, acquired.Lease, leaseStateActive, CallOptions{})
		if err != nil {
			b.Fatalf("Renew: %v", err)
		}
		if !renew.Renewed {
			b.Fatalf("Renew rejected: %s", renew.Reason)
		}
	}
}

func BenchmarkLeaseManagerConcurrentContention(b *testing.B) {
	_, _, manager := setupTestManager(b, DefaultConfig())
	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "bench-concurrent"}

	var seq atomic.Uint64

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n := seq.Add(1)
			token := fmt.Sprintf("token-%d", n)
			worker := fmt.Sprintf("worker-%d", n%128)
			result, err := manager.Acquire(ctx, ref, worker, token, CallOptions{})
			if err != nil {
				b.Fatalf("Acquire: %v", err)
			}
			if result.Acquired {
				release, err := manager.Release(ctx, ref, token, CallOptions{})
				if err != nil {
					b.Fatalf("Release: %v", err)
				}
				if !release.Released {
					b.Fatalf("Release rejected: %s", release.Reason)
				}
			}
		}
	})
}

func TestRunHeartbeatNoGoroutineLeak(t *testing.T) {
	t.Parallel()

	_, _, manager := setupTestManager(t, Config{
		LeaseTTL:          500 * time.Millisecond,
		HeartbeatInterval: 120 * time.Millisecond,
		HeartbeatJitter:   0,
	})

	baseline := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: fmt.Sprintf("leak-%d", i)}
		token := fmt.Sprintf("token-%d", i)

		acquired, err := manager.Acquire(context.Background(), ref, "worker-leak", token, CallOptions{})
		if err != nil {
			t.Fatalf("Acquire(%d): %v", i, err)
		}
		if !acquired.Acquired {
			t.Fatalf("expected acquired lease for ref=%v", ref)
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- manager.RunHeartbeat(ctx, acquired.Lease, HeartbeatOptions{
				GracefulDrain: true,
			})
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()
		select {
		case err := <-done:
			if err != nil && err != context.Canceled {
				t.Fatalf("heartbeat error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for heartbeat goroutine")
		}
	}

	// Give runtime a chance to reap goroutines created by timers.
	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
	after := runtime.NumGoroutine()
	if after > baseline+6 {
		t.Fatalf("possible goroutine leak: baseline=%d after=%d", baseline, after)
	}
}
