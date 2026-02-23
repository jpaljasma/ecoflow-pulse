package ingestworker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
)

func TestLoopStartsAndStopsOnPause(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "R351ZABAPH331057",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(slog.Default(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()

	waitForAtLeast(t, &runner.starts, 1, time.Second, "session start")

	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "R351ZABAPH331057",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "paused",
		},
	})

	waitForAtLeast(t, &runner.stops, 1, time.Second, "session stop on pause")
	waitForAtLeast(t, &leases.heartbeatStarts, 1, time.Second, "heartbeat start")
	waitForAtLeast(t, &leases.heartbeatStops, 1, time.Second, "heartbeat stop")

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("loop returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for loop stop")
	}
}

func TestLoopStopsOnCredentialDisable(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "Y711ZABA9H2P0294",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(slog.Default(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()

	waitForAtLeast(t, &runner.starts, 1, time.Second, "session start")

	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "Y711ZABA9H2P0294",
			DeviceIsActive:     true,
			CredentialIsActive: false,
			IngestDesiredState: "active",
		},
	})
	waitForAtLeast(t, &runner.stops, 1, time.Second, "session stop on credential disable")

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("loop returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for loop stop")
	}
}

func TestLoopSkipsWhenLeaseDenied(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "R634ZABAWH2G1008",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})
	leases := &fakeLeaseManager{denyAcquire: true}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(slog.Default(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 20 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()

	time.Sleep(140 * time.Millisecond)
	if got := runner.starts.Load(); got != 0 {
		t.Fatalf("expected zero session starts when lease denied, got=%d", got)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("loop returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for loop stop")
	}
}

func waitForAtLeast(t *testing.T, counter *atomic.Int64, want int64, timeout time.Duration, name string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if counter.Load() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s >= %d (got=%d)", name, want, counter.Load())
}

type fakeStore struct {
	mu    sync.RWMutex
	items []controlplane.IngestAssignment
}

func (s *fakeStore) set(items []controlplane.IngestAssignment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append([]controlplane.IngestAssignment(nil), items...)
}

func (s *fakeStore) ListIngestAssignments(_ context.Context, _ controlplane.ListIngestAssignmentsInput) ([]controlplane.IngestAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]controlplane.IngestAssignment(nil), s.items...), nil
}

type fakeLeaseManager struct {
	denyAcquire     bool
	heartbeatStarts atomic.Int64
	heartbeatStops  atomic.Int64
}

func (m *fakeLeaseManager) Acquire(_ context.Context, ref ingestlease.LeaseRef, workerID string, token string, _ ingestlease.CallOptions) (ingestlease.AcquireResult, error) {
	if m.denyAcquire {
		return ingestlease.AcquireResult{Acquired: false}, nil
	}
	return ingestlease.AcquireResult{
		Acquired: true,
		Lease: ingestlease.Lease{
			Ref:      ref,
			Token:    token,
			WorkerID: workerID,
			Fence:    1,
			TTL:      45 * time.Second,
		},
	}, nil
}

func (m *fakeLeaseManager) RunHeartbeat(ctx context.Context, _ ingestlease.Lease, _ ingestlease.HeartbeatOptions) error {
	m.heartbeatStarts.Add(1)
	<-ctx.Done()
	m.heartbeatStops.Add(1)
	return nil
}

type fakeSessionRunner struct {
	starts atomic.Int64
	stops  atomic.Int64
}

func (r *fakeSessionRunner) Run(ctx context.Context, _ controlplane.IngestAssignment) error {
	r.starts.Add(1)
	<-ctx.Done()
	r.stops.Add(1)
	return nil
}
