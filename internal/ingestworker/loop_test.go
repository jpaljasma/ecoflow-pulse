package ingestworker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
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
			ProviderDeviceID:   "DEMOD2M00001057",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 8,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "session start")

	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "DEMOD2M00001057",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "paused",
		},
	})

	waitForAtLeast(t, &runner.stops, 1, time.Second, "session stop on pause")
	waitForAtLeast(t, &leases.heartbeatStarts, 1, time.Second, "heartbeat start")
	waitForAtLeast(t, &leases.heartbeatStops, 1, time.Second, "heartbeat stop")

	cancelAndWait(t, cancel, done)
}

func TestRecommendedStartWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		goMaxProcs int
		want       int
	}{
		{name: "min clamp", goMaxProcs: 1, want: 8},
		{name: "middle", goMaxProcs: 4, want: 16},
		{name: "max clamp", goMaxProcs: 128, want: 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RecommendedStartWorkers(tt.goMaxProcs); got != tt.want {
				t.Fatalf("RecommendedStartWorkers(%d)=%d want=%d", tt.goMaxProcs, got, tt.want)
			}
		})
	}
}

func TestRecommendedStartQueueSize(t *testing.T) {
	t.Parallel()

	if got := RecommendedStartQueueSize(8); got != 64 {
		t.Fatalf("RecommendedStartQueueSize(8)=%d want=64", got)
	}
	if got := RecommendedStartQueueSize(16); got != 128 {
		t.Fatalf("RecommendedStartQueueSize(16)=%d want=128", got)
	}
}

func TestDefaultConfigLeaseMissingAlertDefaults(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("worker-test")
	if cfg.CredentialRejectCooldown != 5*time.Minute {
		t.Fatalf("CredentialRejectCooldown=%v want=5m", cfg.CredentialRejectCooldown)
	}
	if cfg.LeaseMissingAlertWindow != 5*time.Minute {
		t.Fatalf("LeaseMissingAlertWindow=%v want=5m", cfg.LeaseMissingAlertWindow)
	}
	if cfg.LeaseMissingAlertThreshold != 4 {
		t.Fatalf("LeaseMissingAlertThreshold=%d want=4", cfg.LeaseMissingAlertThreshold)
	}
	if cfg.LeaseMissingAlertCooldown != 2*time.Minute {
		t.Fatalf("LeaseMissingAlertCooldown=%v want=2m", cfg.LeaseMissingAlertCooldown)
	}
}

func TestLoopStopsOnCredentialDisable(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "DEMODPU0000294",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 8,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "session start")

	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "DEMODPU0000294",
			DeviceIsActive:     true,
			CredentialIsActive: false,
			IngestDesiredState: "active",
		},
	})
	waitForAtLeast(t, &runner.stops, 1, time.Second, "session stop on credential disable")

	cancelAndWait(t, cancel, done)
}

func TestLoopRestartsWhenCredentialIDChanges(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "DEMODPU0000294",
			CredentialID:       "cred-1",
			AccessKey:          "ak-1",
			SecretKey:          "sk-1",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 8,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "initial session start")

	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "DEMODPU0000294",
			CredentialID:       "cred-2",
			AccessKey:          "ak-2",
			SecretKey:          "sk-2",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})

	waitForAtLeast(t, &runner.stops, 1, time.Second, "session stop on credential id change")
	waitForAtLeast(t, &runner.starts, 2, time.Second, "session restart on credential id change")
	cancelAndWait(t, cancel, done)
}

func TestLoopRestartsWhenCredentialMaterialChanges(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "DEMOD2M00001057",
			CredentialID:       "cred-1",
			AccessKey:          "ak-1",
			SecretKey:          "sk-1",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 8,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "initial session start")

	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "DEMOD2M00001057",
			CredentialID:       "cred-1",
			AccessKey:          "ak-rotated",
			SecretKey:          "sk-rotated",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})

	waitForAtLeast(t, &runner.stops, 1, time.Second, "session stop on credential material change")
	waitForAtLeast(t, &runner.starts, 2, time.Second, "session restart on credential material change")
	cancelAndWait(t, cancel, done)
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

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 20 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 4,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	time.Sleep(160 * time.Millisecond)
	if got := runner.starts.Load(); got != 0 {
		t.Fatalf("expected zero session starts when lease denied, got=%d", got)
	}

	cancelAndWait(t, cancel, done)
}

func TestLoopDedupesDuplicateAssignments(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{Provider: "ecoflow", ProviderDeviceID: " demod2m00001057 ", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
		{Provider: "ECOFLOW", ProviderDeviceID: "DEMOD2M00001057", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 20 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 4,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "session start")
	time.Sleep(100 * time.Millisecond)
	if got := runner.starts.Load(); got != 1 {
		t.Fatalf("expected deduped starts=1, got=%d", got)
	}

	cancelAndWait(t, cancel, done)
}

func TestLoopStopsWhenAssignmentRemoved(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{Provider: "ecoflow", ProviderDeviceID: "R1", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 20 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 4,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "session start")
	store.set(nil)
	waitForAtLeast(t, &runner.stops, 1, time.Second, "session stop on assignment removal")

	cancelAndWait(t, cancel, done)
}

func TestLoopHonorsProviderFilter(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{Provider: "ecoflow", ProviderDeviceID: "R1", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
		{Provider: "victron", ProviderDeviceID: "V1", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:       "worker-test",
		ProviderFilter: "ecoflow",
		PollInterval:   20 * time.Millisecond,
		PollJitter:     0,
		StopTimeout:    2 * time.Second,
		StartWorkers:   4,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "session start")
	time.Sleep(100 * time.Millisecond)
	if got := runner.starts.Load(); got != 1 {
		t.Fatalf("expected only provider-filtered start, got=%d", got)
	}

	cancelAndWait(t, cancel, done)
}

func TestLoopRestartsAfterRunnerError(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{Provider: "ecoflow", ProviderDeviceID: "R1", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{failFirst: true}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 20 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 4,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 2, time.Second, "session restart after runner error")
	cancelAndWait(t, cancel, done)
}

func TestLoopRestartsAfterHeartbeatError(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{Provider: "ecoflow", ProviderDeviceID: "R1", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
	})
	leases := &fakeLeaseManager{heartbeatErrOnce: true, heartbeatErr: errors.New("heartbeat failed")}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 20 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 4,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 2, time.Second, "session restart after heartbeat error")
	cancelAndWait(t, cancel, done)
}

func TestLoopBacksOffAfterCredentialRejected(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{Provider: "ecoflow", ProviderDeviceID: "R1", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{failErrOnce: ErrEcoFlowCredentialRejected}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:                 "worker-test",
		PollInterval:             20 * time.Millisecond,
		PollJitter:               0,
		StopTimeout:              2 * time.Second,
		StartWorkers:             4,
		CredentialRejectCooldown: 120 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "initial session start")
	time.Sleep(80 * time.Millisecond)
	if got := runner.starts.Load(); got != 1 {
		t.Fatalf("expected credential rejection cooldown to suppress immediate restart, got starts=%d", got)
	}

	waitForAtLeast(t, &runner.starts, 2, time.Second, "session restart after credential cooldown")
	cancelAndWait(t, cancel, done)
}

func TestLoopConcurrentStartsManyAssignments(t *testing.T) {
	t.Parallel()

	const n = 300
	assignments := make([]controlplane.IngestAssignment, 0, n)
	for i := 0; i < n; i++ {
		assignments = append(assignments, controlplane.IngestAssignment{
			Provider:           "ecoflow",
			ProviderDeviceID:   fmt.Sprintf("SN-%d", i),
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		})
	}
	store := &fakeStore{}
	store.set(assignments)
	leases := &fakeLeaseManager{acquireDelay: 2 * time.Millisecond}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 200 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  4 * time.Second,
		StartWorkers: 48,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, n, 4*time.Second, "all sessions start")
	if got := leases.maxConcurrentAcquire.Load(); got < 8 {
		t.Fatalf("expected concurrent acquires >= 8, got=%d", got)
	}

	cancelAndWait(t, cancel, done)
}

func TestLoopNoGoroutineLeakOnShutdown(t *testing.T) {
	baseline := runtime.NumGoroutine()

	const n = 80
	assignments := make([]controlplane.IngestAssignment, 0, n)
	for i := 0; i < n; i++ {
		assignments = append(assignments, controlplane.IngestAssignment{
			Provider:           "ecoflow",
			ProviderDeviceID:   fmt.Sprintf("SN-%d", i),
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		})
	}

	store := &fakeStore{}
	store.set(assignments)
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 25 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  5 * time.Second,
		StartWorkers: 32,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)
	waitForAtLeast(t, &runner.starts, n, 2*time.Second, "session start")

	cancelAndWait(t, cancel, done)

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(25 * time.Millisecond)
		if runtime.NumGoroutine() <= baseline+24 {
			return
		}
	}
	t.Fatalf("possible goroutine leak: baseline=%d current=%d", baseline, runtime.NumGoroutine())
}

func TestLoopIgnoresStaleTerminationEvent(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{Provider: "ecoflow", ProviderDeviceID: "R1", DeviceIsActive: true, CredentialIsActive: true, IngestDesiredState: "active"},
	})
	leases := &fakeLeaseManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 20 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 4,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "session start")
	key := assignmentKey("ecoflow", "R1")
	token := waitForRunningToken(t, loop, key, time.Second)
	if token == "" {
		t.Fatalf("expected running token")
	}

	loop.handleTerminationEvent(terminationEvent{
		key:    key,
		token:  token + "-stale",
		source: "session",
		err:    nil,
	})
	if got := runningCount(loop); got != 1 {
		t.Fatalf("stale termination event unexpectedly stopped running session: running=%d", got)
	}

	cancelAndWait(t, cancel, done)
}

func TestLoopCancelDuringBurstStart(t *testing.T) {
	t.Parallel()

	baseline := runtime.NumGoroutine()

	const n = 500
	assignments := make([]controlplane.IngestAssignment, 0, n)
	for i := 0; i < n; i++ {
		assignments = append(assignments, controlplane.IngestAssignment{
			Provider:           "ecoflow",
			ProviderDeviceID:   fmt.Sprintf("SN-%d", i),
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		})
	}
	store := &fakeStore{}
	store.set(assignments)
	leases := &fakeLeaseManager{acquireDelay: 15 * time.Millisecond}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-test",
		PollInterval: 3 * time.Second,
		PollJitter:   0,
		StopTimeout:  5 * time.Second,
		StartWorkers: 64,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	time.Sleep(100 * time.Millisecond)
	cancelAndWait(t, cancel, done)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(25 * time.Millisecond)
		if runtime.NumGoroutine() <= baseline+32 {
			return
		}
	}
	t.Fatalf("possible goroutine leak after burst cancel: baseline=%d current=%d", baseline, runtime.NumGoroutine())
}

func runLoop(ctx context.Context, loop *Loop) chan error {
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()
	return done
}

func cancelAndWait(t *testing.T, cancel context.CancelFunc, done chan error) {
	t.Helper()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("loop returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
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

func runningCount(loop *Loop) int {
	loop.mu.Lock()
	defer loop.mu.Unlock()
	return len(loop.running)
}

func runningToken(t *testing.T, loop *Loop, key string) string {
	t.Helper()
	loop.mu.Lock()
	defer loop.mu.Unlock()
	rs, ok := loop.running[key]
	if !ok {
		return ""
	}
	return rs.lease.Token
}

func waitForRunningToken(t *testing.T, loop *Loop, key string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if token := runningToken(t, loop, key); token != "" {
			return token
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ""
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
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

func (s *fakeStore) ListIngestAssignments(_ context.Context, in controlplane.ListIngestAssignmentsInput) ([]controlplane.IngestAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]controlplane.IngestAssignment, 0, len(s.items))
	for i := range s.items {
		item := s.items[i]
		if in.Provider != "" && controlplane.NormalizeProvider(item.Provider) != controlplane.NormalizeProvider(in.Provider) {
			continue
		}
		if in.ActiveOnly {
			if !item.DeviceIsActive {
				continue
			}
			if item.IngestDesiredState != "active" {
				continue
			}
		}
		out = append(out, item)
	}
	return out, nil
}

type fakeLeaseManager struct {
	denyAcquire  bool
	acquireErr   error
	acquireDelay time.Duration

	heartbeatErr         error
	heartbeatErrOnce     bool
	heartbeatErrSent     atomic.Bool
	heartbeatStarts      atomic.Int64
	heartbeatStops       atomic.Int64
	inflightAcquire      atomic.Int64
	maxConcurrentAcquire atomic.Int64
}

func (m *fakeLeaseManager) Acquire(_ context.Context, ref ingestlease.LeaseRef, workerID string, token string, _ ingestlease.CallOptions) (ingestlease.AcquireResult, error) {
	inflight := m.inflightAcquire.Add(1)
	for {
		max := m.maxConcurrentAcquire.Load()
		if inflight <= max {
			break
		}
		if m.maxConcurrentAcquire.CompareAndSwap(max, inflight) {
			break
		}
	}
	defer m.inflightAcquire.Add(-1)

	if m.acquireDelay > 0 {
		time.Sleep(m.acquireDelay)
	}
	if m.acquireErr != nil {
		return ingestlease.AcquireResult{}, m.acquireErr
	}
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
	defer m.heartbeatStops.Add(1)
	if m.heartbeatErr != nil {
		if !m.heartbeatErrOnce || m.heartbeatErrSent.CompareAndSwap(false, true) {
			return m.heartbeatErr
		}
	}
	<-ctx.Done()
	return nil
}

type fakeSessionRunner struct {
	starts      atomic.Int64
	stops       atomic.Int64
	failFirst   bool
	failErrOnce error
	failed      atomic.Bool
}

func (r *fakeSessionRunner) Run(ctx context.Context, _ controlplane.IngestAssignment) error {
	r.starts.Add(1)
	if r.failErrOnce != nil && r.failed.CompareAndSwap(false, true) {
		r.stops.Add(1)
		return r.failErrOnce
	}
	if r.failFirst && r.failed.CompareAndSwap(false, true) {
		r.stops.Add(1)
		return errors.New("runner failed")
	}
	<-ctx.Done()
	r.stops.Add(1)
	return nil
}
