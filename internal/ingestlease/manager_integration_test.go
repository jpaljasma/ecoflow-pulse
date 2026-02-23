//go:build integration

package ingestlease

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkLeaseManagerAcquireReleaseIntegration(b *testing.B) {
	manager := requireIntegrationManager(b)
	ctx := context.Background()
	var seq atomic.Uint64

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		n := seq.Add(1)
		ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: fmt.Sprintf("int-acq-%d", n)}
		token := fmt.Sprintf("token-%d", n)
		result, err := manager.Acquire(ctx, ref, "worker-int", token, CallOptions{})
		if err != nil {
			b.Fatalf("Acquire: %v", err)
		}
		if !result.Acquired {
			b.Fatalf("Acquire denied for unique key")
		}
		released, err := manager.Release(ctx, ref, token, CallOptions{})
		if err != nil {
			b.Fatalf("Release: %v", err)
		}
		if !released.Released {
			b.Fatalf("Release rejected: %s", released.Reason)
		}
	}
}

func BenchmarkLeaseManagerRenewIntegration(b *testing.B) {
	manager := requireIntegrationManager(b)
	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "int-renew"}
	token := "int-renew-token"

	acquired, err := manager.Acquire(ctx, ref, "worker-int", token, CallOptions{})
	if err != nil {
		b.Fatalf("Acquire: %v", err)
	}
	if !acquired.Acquired {
		b.Fatalf("Acquire denied for renew benchmark")
	}
	b.Cleanup(func() {
		_, _ = manager.Release(context.Background(), ref, token, CallOptions{})
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		renewed, err := manager.Renew(ctx, acquired.Lease, leaseStateActive, CallOptions{})
		if err != nil {
			b.Fatalf("Renew: %v", err)
		}
		if !renewed.Renewed {
			b.Fatalf("Renew rejected: %s", renewed.Reason)
		}
	}
}

func BenchmarkLeaseManagerConcurrentContentionIntegration(b *testing.B) {
	manager := requireIntegrationManager(b)
	ctx := context.Background()
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "int-concurrent"}
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
				released, err := manager.Release(ctx, ref, token, CallOptions{})
				if err != nil {
					b.Fatalf("Release: %v", err)
				}
				if !released.Released {
					b.Fatalf("Release rejected: %s", released.Reason)
				}
			}
		}
	})
}

func TestRunHeartbeatNoGoroutineLeakIntegration(t *testing.T) {
	manager := requireIntegrationManager(t)
	baseline := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: fmt.Sprintf("int-leak-%d", i)}
		token := fmt.Sprintf("token-%d", i)

		acquired, err := manager.Acquire(context.Background(), ref, "worker-int", token, CallOptions{})
		if err != nil {
			t.Fatalf("Acquire(%d): %v", i, err)
		}
		if !acquired.Acquired {
			t.Fatalf("Acquire denied for ref=%+v", ref)
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func(lease Lease) {
			done <- manager.RunHeartbeat(ctx, lease, HeartbeatOptions{
				GracefulDrain: true,
			})
		}(acquired.Lease)

		time.Sleep(120 * time.Millisecond)
		cancel()

		select {
		case err := <-done:
			if err != nil && err != context.Canceled {
				t.Fatalf("heartbeat error: %v", err)
			}
		case <-time.After(4 * time.Second):
			t.Fatalf("timeout waiting for heartbeat goroutine")
		}
	}

	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(25 * time.Millisecond)
	}
	after := runtime.NumGoroutine()
	if after > baseline+10 {
		t.Fatalf("possible goroutine leak: baseline=%d after=%d", baseline, after)
	}
}

func TestRunHeartbeatReleasesOnCancelIntegration(t *testing.T) {
	manager := requireIntegrationManager(t)
	ref := LeaseRef{Provider: "ecoflow", ProviderDeviceID: "int-cancel-release"}
	token := "token-int-cancel-release"

	acquired, err := manager.Acquire(context.Background(), ref, "worker-int", token, CallOptions{})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !acquired.Acquired {
		t.Fatalf("expected acquire success")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func(lease Lease) {
		done <- manager.RunHeartbeat(ctx, lease, HeartbeatOptions{
			GracefulDrain: false,
		})
	}(acquired.Lease)

	time.Sleep(120 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Fatalf("heartbeat error: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatalf("timeout waiting for heartbeat goroutine")
	}

	recheck, err := manager.Acquire(context.Background(), ref, "worker-int-2", "token-int-cancel-release-2", CallOptions{})
	if err != nil {
		t.Fatalf("re-acquire after cancel cleanup failed: %v", err)
	}
	if !recheck.Acquired {
		t.Fatalf("expected immediate re-acquire after cancel cleanup")
	}
	_, _ = manager.Release(context.Background(), ref, recheck.Lease.Token, CallOptions{})
}

var (
	integrationMgrOnce sync.Once
	integrationMgr     *Manager
	integrationMgrErr  error
)

func requireIntegrationManager(tb testing.TB) *Manager {
	tb.Helper()

	integrationMgrOnce.Do(func() {
		cfg := DefaultValkeyClientConfig(integrationValkeyAddresses())
		client, err := NewValkeyClient(cfg)
		if err != nil {
			integrationMgrErr = fmt.Errorf("create valkey client: %w", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
			integrationMgrErr = fmt.Errorf("ping valkey: %w", err)
			client.Close()
			return
		}

		manager, err := NewManager(client, DefaultConfig())
		if err != nil {
			integrationMgrErr = fmt.Errorf("create manager: %w", err)
			client.Close()
			return
		}
		integrationMgr = manager
	})

	if integrationMgrErr != nil || integrationMgr == nil {
		tb.Skipf("integration valkey benchmark unavailable: %v", integrationMgrErr)
	}

	return integrationMgr
}

func integrationValkeyAddresses() []string {
	if raw := strings.TrimSpace(os.Getenv("VALKEY_INTEGRATION_ADDRS")); raw != "" {
		parts := strings.Split(raw, ",")
		addresses := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				addresses = append(addresses, trimmed)
			}
		}
		if len(addresses) > 0 {
			return addresses
		}
	}
	return []string{"127.0.0.1:6379"}
}
