package ingestworker

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

func TestLoopChaosReacquiresAfterLeaseLoss(t *testing.T) {
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
	leases := &leaseLossChaosManager{}
	runner := &fakeSessionRunner{}

	loop, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-chaos",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 4,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, loop)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "initial session start")
	key := assignmentKey(controlplane.ProviderEcoFlow, "DEMOD2M00001057")
	firstToken := waitForRunningToken(t, loop, key, time.Second)
	if firstToken == "" {
		t.Fatal("expected running session token")
	}

	waitForCondition(t, 2*time.Second, "lease-loss restart with a new token", func() bool {
		return runner.starts.Load() >= 2 && runningToken(t, loop, key) != "" && runningToken(t, loop, key) != firstToken
	})

	cancelAndWait(t, cancel, done)
}

func TestLoopChaosWorkerCrashHandsLeaseToPeer(t *testing.T) {
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
	leases := &singleOwnerChaosLeaseManager{}
	runner := &activeSessionRunner{}

	loopA, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-a",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 1,
	})
	if err != nil {
		t.Fatalf("NewLoop worker-a error: %v", err)
	}
	loopB, err := NewLoop(testLogger(), store, leases, runner, Config{
		WorkerID:     "worker-b",
		PollInterval: 15 * time.Millisecond,
		PollJitter:   0,
		StopTimeout:  2 * time.Second,
		StartWorkers: 1,
	})
	if err != nil {
		t.Fatalf("NewLoop worker-b error: %v", err)
	}

	ctxA, cancelA := context.WithCancel(context.Background())
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelA()
	defer cancelB()
	doneA := runLoop(ctxA, loopA)
	doneB := runLoop(ctxB, loopB)

	waitForAtLeast(t, &runner.starts, 1, time.Second, "initial session start")
	waitForCondition(t, time.Second, "single worker lease owner", func() bool {
		return leases.currentHolderWorker() != ""
	})
	time.Sleep(80 * time.Millisecond)
	if got := runner.starts.Load(); got != 1 {
		t.Fatalf("expected exactly one active start before crash handoff, got=%d", got)
	}

	originalHolder := leases.currentHolderWorker()
	if originalHolder == "" {
		t.Fatal("expected a lease holder before crash handoff")
	}

	if originalHolder == "worker-a" {
		cancelAndWait(t, cancelA, doneA)
	} else {
		cancelAndWait(t, cancelB, doneB)
	}

	waitForCondition(t, 2*time.Second, "lease handoff to peer worker", func() bool {
		holder := leases.currentHolderWorker()
		return holder != "" && holder != originalHolder && runner.starts.Load() >= 2
	})
	if got := runner.maxActive.Load(); got != 1 {
		t.Fatalf("expected lease handoff to keep max concurrent sessions at 1, got=%d", got)
	}
	if got := leases.denied.Load(); got == 0 {
		t.Fatal("expected peer worker to observe at least one denied acquire while lease was held")
	}

	if originalHolder == "worker-a" {
		cancelAndWait(t, cancelB, doneB)
	} else {
		cancelAndWait(t, cancelA, doneA)
	}
}

func TestEcoFlowSessionRunnerChaosSerializesReconnectStorm(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	publisher := &fakeEnvelopePublisher{}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
		ReconnectInitialBackoff: time.Millisecond,
		ReconnectMaxBackoff:     2 * time.Millisecond,
		ReconnectJitter:         0,
		QuotaRefreshInterval:    time.Hour,
	})
	if err != nil {
		t.Fatalf("NewEcoFlowSessionRunner() error = %v", err)
	}

	resolver := &fakeCertResolver{
		cert: ecoflow.GeneralInfoMQTTCertification{
			CertificateAccount:  "open-account",
			CertificatePassword: "secret",
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
		},
	}
	const stormAttempts = 5
	subs := make([]mqttSubscriber, 0, stormAttempts+1)
	for i := 0; i < stormAttempts; i++ {
		subs = append(subs, &fakeMQTTSubscriber{
			reads: []fakeReadResult{{err: io.EOF}},
		})
	}
	subs = append(subs, &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{
				msg: ecoflowmqtt.Message{
					Topic:   "/open/open-account/DEMODPU0000294/quota",
					Payload: []byte(`{"id":1,"typeCode":"kitInfo"}`),
				},
			},
			{err: context.Canceled},
		},
	})
	factory := &trackingSubscriberFactory{subscribers: subs}
	runner.adapter = resolver
	runner.newSubscriber = factory.new

	var sleepMu sync.Mutex
	sleeps := make([]time.Duration, 0, stormAttempts)
	runner.sleepFn = func(_ context.Context, duration time.Duration) error {
		if duration > 10*time.Millisecond {
			return context.Canceled
		}
		sleepMu.Lock()
		sleeps = append(sleeps, duration)
		sleepMu.Unlock()
		return nil
	}

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMODPU0000294",
		DeviceID:           "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:       "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
		AccessKey:          "ak",
		SecretKey:          "sk",
		CredentialIsActive: true,
	}

	if err := runner.Run(ctx, assignment); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := resolver.calls.Load(); got != stormAttempts+1 {
		t.Fatalf("expected %d certification attempts across reconnect storm, got=%d", stormAttempts+1, got)
	}
	if got := len(factory.configs); got != stormAttempts+1 {
		t.Fatalf("expected %d subscriber initializations, got=%d", stormAttempts+1, got)
	}
	sleepMu.Lock()
	sleepCount := len(sleeps)
	sleepSnapshot := append([]time.Duration(nil), sleeps...)
	sleepMu.Unlock()
	if sleepCount != stormAttempts {
		t.Fatalf("expected %d reconnect sleeps, got=%d", stormAttempts, sleepCount)
	}
	for i, duration := range sleepSnapshot {
		if duration != time.Millisecond {
			t.Fatalf("expected storm sleep %d to reset to 1ms after a connected EOF, got=%s", i, duration)
		}
	}
	if got := factory.maxActive.Load(); got != 1 {
		t.Fatalf("expected reconnect storm to keep one active subscriber at a time, got=%d", got)
	}
	if got := factory.active.Load(); got != 0 {
		t.Fatalf("expected all storm subscribers to close, active=%d", got)
	}
}

type activeSessionRunner struct {
	starts    atomic.Int64
	stops     atomic.Int64
	active    atomic.Int64
	maxActive atomic.Int64
}

func (r *activeSessionRunner) Run(ctx context.Context, _ controlplane.IngestAssignment) error {
	r.starts.Add(1)
	active := r.active.Add(1)
	for {
		max := r.maxActive.Load()
		if active <= max {
			break
		}
		if r.maxActive.CompareAndSwap(max, active) {
			break
		}
	}
	defer func() {
		r.active.Add(-1)
		r.stops.Add(1)
	}()
	<-ctx.Done()
	return nil
}

type leaseLossChaosManager struct {
	heartbeatStarts atomic.Int64
	lostOnce        atomic.Bool
}

func (m *leaseLossChaosManager) Acquire(_ context.Context, ref ingestlease.LeaseRef, workerID string, token string, _ ingestlease.CallOptions) (ingestlease.AcquireResult, error) {
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

func (m *leaseLossChaosManager) RunHeartbeat(ctx context.Context, lease ingestlease.Lease, _ ingestlease.HeartbeatOptions) error {
	m.heartbeatStarts.Add(1)
	if m.lostOnce.CompareAndSwap(false, true) {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(20 * time.Millisecond):
			return ingestlease.NewLeaseRejectedError("renew", "missing")
		}
	}
	<-ctx.Done()
	return nil
}

type singleOwnerChaosLeaseManager struct {
	mu           sync.Mutex
	holderWorker string
	holderToken  string
	denied       atomic.Int64
}

func (m *singleOwnerChaosLeaseManager) Acquire(_ context.Context, ref ingestlease.LeaseRef, workerID string, token string, _ ingestlease.CallOptions) (ingestlease.AcquireResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.holderToken != "" {
		m.denied.Add(1)
		return ingestlease.AcquireResult{Acquired: false}, nil
	}
	m.holderWorker = workerID
	m.holderToken = token
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

func (m *singleOwnerChaosLeaseManager) RunHeartbeat(ctx context.Context, lease ingestlease.Lease, _ ingestlease.HeartbeatOptions) error {
	<-ctx.Done()
	time.Sleep(5 * time.Millisecond)
	m.mu.Lock()
	if m.holderToken == lease.Token {
		m.holderToken = ""
		m.holderWorker = ""
	}
	m.mu.Unlock()
	return nil
}

func (m *singleOwnerChaosLeaseManager) currentHolderWorker() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.holderWorker
}

type trackingSubscriberFactory struct {
	mu          sync.Mutex
	subscribers []mqttSubscriber
	configs     []ecoflowmqtt.Config
	active      atomic.Int64
	maxActive   atomic.Int64
}

func (f *trackingSubscriberFactory) new(cfg ecoflowmqtt.Config) (mqttSubscriber, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs = append(f.configs, cfg)
	if len(f.subscribers) == 0 {
		return nil, io.EOF
	}
	sub := f.subscribers[0]
	f.subscribers = f.subscribers[1:]
	active := f.active.Add(1)
	for {
		max := f.maxActive.Load()
		if active <= max {
			break
		}
		if f.maxActive.CompareAndSwap(max, active) {
			break
		}
	}
	return &trackedMQTTSubscriber{
		inner: sub,
		onClose: func() {
			f.active.Add(-1)
		},
	}, nil
}

type trackedMQTTSubscriber struct {
	inner   mqttSubscriber
	onClose func()
	once    sync.Once
}

func (s *trackedMQTTSubscriber) Connect(ctx context.Context) error {
	return s.inner.Connect(ctx)
}

func (s *trackedMQTTSubscriber) Subscribe(ctx context.Context, topic string, qos byte) error {
	return s.inner.Subscribe(ctx, topic, qos)
}

func (s *trackedMQTTSubscriber) ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error) {
	return s.inner.ReadMessage(ctx)
}

func (s *trackedMQTTSubscriber) Disconnect() error {
	return s.inner.Disconnect()
}

func (s *trackedMQTTSubscriber) Close() error {
	err := s.inner.Close()
	s.once.Do(func() {
		if s.onClose != nil {
			s.onClose()
		}
	})
	return err
}

func waitForCondition(t *testing.T, timeout time.Duration, name string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", name)
}
