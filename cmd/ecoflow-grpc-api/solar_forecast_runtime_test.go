package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
)

func TestSolarVerificationLoopUsesSingleLeaseHolderAndHandsOff(t *testing.T) {
	t.Parallel()

	manager := newStatefulSolarVerificationLeaseManager()
	service := &countingSolarVerificationService{}
	manualTicker := func(time.Duration) solarVerificationTicker {
		return solarVerificationTicker{
			C:    make(chan time.Time),
			Stop: func() {},
		}
	}

	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	stopA := startSolarVerificationLoopWithLease(ctxA, slog.Default(), service, solarVerificationLoopDeps{
		leaseManager:   manager,
		leaseSource:    "valkey",
		workerID:       "worker-a",
		interval:       5 * time.Second,
		batchLimit:     3,
		acquireRetry:   10 * time.Millisecond,
		newTicker:      manualTicker,
		nowFn:          time.Now,
		leaseReference: solarVerificationLeaseRef,
	})
	defer stopA()

	manager.waitForAcquireCount(t, 1)
	service.waitForCalls(t, 1)

	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	stopB := startSolarVerificationLoopWithLease(ctxB, slog.Default(), service, solarVerificationLoopDeps{
		leaseManager:   manager,
		leaseSource:    "valkey",
		workerID:       "worker-b",
		interval:       5 * time.Second,
		batchLimit:     3,
		acquireRetry:   10 * time.Millisecond,
		newTicker:      manualTicker,
		nowFn:          time.Now,
		leaseReference: solarVerificationLeaseRef,
	})
	defer stopB()

	manager.waitForAcquireCount(t, 2)
	service.waitForCalls(t, 1)

	cancelA()

	manager.waitForAcquireCount(t, 3)
	service.waitForCalls(t, 2)
}

type countingSolarVerificationService struct {
	calls atomic.Int32
}

func (s *countingSolarVerificationService) VerifyIssuedForecasts(context.Context, time.Time, int) error {
	s.calls.Add(1)
	return nil
}

func (s *countingSolarVerificationService) waitForCalls(t *testing.T, want int32) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if got := s.calls.Load(); got >= want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for verification calls: got=%d want>=%d", s.calls.Load(), want)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

type statefulSolarVerificationLeaseManager struct {
	mu            sync.Mutex
	holderToken   string
	holderWorker  string
	nextFence     int64
	acquireCount  atomic.Int32
	heartbeatDone chan struct{}
}

func newStatefulSolarVerificationLeaseManager() *statefulSolarVerificationLeaseManager {
	return &statefulSolarVerificationLeaseManager{
		heartbeatDone: make(chan struct{}, 1),
	}
}

func (m *statefulSolarVerificationLeaseManager) Acquire(_ context.Context, ref ingestlease.LeaseRef, workerID string, token string, _ ingestlease.CallOptions) (ingestlease.AcquireResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.acquireCount.Add(1)
	if m.holderToken != "" && m.holderWorker != workerID {
		return ingestlease.AcquireResult{
			Acquired:      false,
			ExistingToken: m.holderToken,
			ExistingFence: m.nextFence,
		}, nil
	}
	if token == "" {
		token = fmt.Sprintf("token-%d", m.acquireCount.Load())
	}
	m.nextFence++
	m.holderToken = token
	m.holderWorker = workerID
	return ingestlease.AcquireResult{
		Acquired: true,
		Lease: ingestlease.Lease{
			Ref:      ref,
			Token:    token,
			WorkerID: workerID,
			Fence:    m.nextFence,
			TTL:      45 * time.Second,
		},
	}, nil
}

func (m *statefulSolarVerificationLeaseManager) RunHeartbeat(ctx context.Context, lease ingestlease.Lease, _ ingestlease.HeartbeatOptions) error {
	<-ctx.Done()
	m.mu.Lock()
	if m.holderToken == lease.Token {
		m.holderToken = ""
		m.holderWorker = ""
	}
	m.mu.Unlock()
	select {
	case m.heartbeatDone <- struct{}{}:
	default:
	}
	return ctx.Err()
}

func (m *statefulSolarVerificationLeaseManager) waitForAcquireCount(t *testing.T, want int32) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if got := m.acquireCount.Load(); got >= want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for lease acquires: got=%d want>=%d", m.acquireCount.Load(), want)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}
