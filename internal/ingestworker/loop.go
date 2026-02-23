package ingestworker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"strings"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
)

const (
	defaultPollInterval = 5 * time.Second
	defaultPollJitter   = 0.20
	defaultStopTimeout  = 8 * time.Second
)

type AssignmentStore interface {
	ListIngestAssignments(ctx context.Context, in controlplane.ListIngestAssignmentsInput) ([]controlplane.IngestAssignment, error)
}

type LeaseManager interface {
	Acquire(ctx context.Context, ref ingestlease.LeaseRef, workerID string, token string, opts ingestlease.CallOptions) (ingestlease.AcquireResult, error)
	RunHeartbeat(ctx context.Context, lease ingestlease.Lease, options ingestlease.HeartbeatOptions) error
}

type SessionRunner interface {
	Run(ctx context.Context, assignment controlplane.IngestAssignment) error
}

type Config struct {
	WorkerID       string
	ProviderFilter string
	PollInterval   time.Duration
	PollJitter     float64
	StopTimeout    time.Duration
}

func DefaultConfig(workerID string) Config {
	return Config{
		WorkerID:     strings.TrimSpace(workerID),
		PollInterval: defaultPollInterval,
		PollJitter:   defaultPollJitter,
		StopTimeout:  defaultStopTimeout,
	}
}

type Loop struct {
	log    *slog.Logger
	store  AssignmentStore
	leases LeaseManager
	runner SessionRunner
	cfg    Config

	mu      sync.Mutex
	rng     *mathrand.Rand
	running map[string]*runningSession
}

type runningSession struct {
	assignment controlplane.IngestAssignment
	lease      ingestlease.Lease
	cancel     context.CancelFunc
	done       chan error
	heartbeat  chan error
}

func NewLoop(log *slog.Logger, store AssignmentStore, leases LeaseManager, runner SessionRunner, cfg Config) (*Loop, error) {
	if log == nil {
		log = slog.Default()
	}
	if store == nil {
		return nil, errors.New("assignment store is required")
	}
	if leases == nil {
		return nil, errors.New("lease manager is required")
	}
	if runner == nil {
		return nil, errors.New("session runner is required")
	}
	cfg.WorkerID = strings.TrimSpace(cfg.WorkerID)
	if cfg.WorkerID == "" {
		return nil, errors.New("worker_id is required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.PollJitter < 0 {
		cfg.PollJitter = 0
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = defaultStopTimeout
	}

	return &Loop{
		log:     log,
		store:   store,
		leases:  leases,
		runner:  runner,
		cfg:     cfg,
		rng:     mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
		running: make(map[string]*runningSession),
	}, nil
}

func (l *Loop) Run(ctx context.Context) error {
	if err := l.reconcile(ctx); err != nil {
		l.log.Warn("ingest worker initial reconcile failed", slog.String("error", err.Error()))
	}

	timer := time.NewTimer(l.nextPollInterval())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			l.stopAll()
			return nil
		case <-timer.C:
			if err := l.reconcile(ctx); err != nil {
				l.log.Warn("ingest worker reconcile failed", slog.String("error", err.Error()))
			}
			timer.Reset(l.nextPollInterval())
		}
	}
}

func (l *Loop) reconcile(ctx context.Context) error {
	l.reapTerminated()

	assignments, err := l.store.ListIngestAssignments(ctx, controlplane.ListIngestAssignmentsInput{
		Provider:   l.cfg.ProviderFilter,
		ActiveOnly: true,
	})
	if err != nil {
		return err
	}

	latest := make(map[string]controlplane.IngestAssignment, len(assignments))
	for i := range assignments {
		a := sanitizeAssignment(assignments[i])
		if a.Provider == "" || a.ProviderDeviceID == "" {
			continue
		}
		key := assignmentKey(a.Provider, a.ProviderDeviceID)
		if _, exists := latest[key]; exists {
			continue
		}
		latest[key] = a
	}

	l.mu.Lock()
	for key, running := range l.running {
		a, exists := latest[key]
		if !exists {
			l.stopSessionLocked(key, running, "assignment_missing")
			continue
		}
		if !shouldRun(a) {
			l.stopSessionLocked(key, running, stopReason(a))
			continue
		}
	}
	l.mu.Unlock()

	for key, a := range latest {
		if !shouldRun(a) {
			continue
		}
		l.mu.Lock()
		_, exists := l.running[key]
		l.mu.Unlock()
		if exists {
			continue
		}
		l.startSession(ctx, a)
	}

	l.reapTerminated()
	return nil
}

func (l *Loop) startSession(ctx context.Context, a controlplane.IngestAssignment) {
	token, err := randomToken()
	if err != nil {
		l.log.Warn("generate lease token failed",
			slog.String("provider", a.Provider),
			slog.String("provider_device_id", a.ProviderDeviceID),
			slog.String("error", err.Error()),
		)
		return
	}

	result, err := l.leases.Acquire(ctx, ingestlease.LeaseRef{
		Provider:         a.Provider,
		ProviderDeviceID: a.ProviderDeviceID,
	}, l.cfg.WorkerID, token, ingestlease.CallOptions{})
	if err != nil {
		l.log.Warn("lease acquire failed",
			slog.String("provider", a.Provider),
			slog.String("provider_device_id", a.ProviderDeviceID),
			slog.String("error", err.Error()),
		)
		return
	}
	if !result.Acquired {
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	rs := &runningSession{
		assignment: a,
		lease:      result.Lease,
		cancel:     cancel,
		done:       make(chan error, 1),
		heartbeat:  make(chan error, 1),
	}
	go func() {
		rs.done <- l.runner.Run(runCtx, a)
	}()
	go func() {
		rs.heartbeat <- l.leases.RunHeartbeat(runCtx, result.Lease, ingestlease.HeartbeatOptions{
			GracefulDrain: true,
		})
	}()

	l.mu.Lock()
	key := assignmentKey(a.Provider, a.ProviderDeviceID)
	if existing, exists := l.running[key]; exists {
		l.stopSessionLocked(key, existing, "duplicate_session_start")
	}
	l.running[key] = rs
	l.mu.Unlock()

	l.log.Info("ingest session started",
		slog.String("provider", a.Provider),
		slog.String("provider_device_id", a.ProviderDeviceID),
		slog.String("worker_id", l.cfg.WorkerID),
	)
}

func (l *Loop) reapTerminated() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, rs := range l.running {
		var (
			terminated bool
			err        error
			source     string
		)

		select {
		case err = <-rs.heartbeat:
			terminated = true
			source = "heartbeat"
		default:
		}
		if !terminated {
			select {
			case err = <-rs.done:
				terminated = true
				source = "session"
			default:
			}
		}
		if !terminated {
			continue
		}

		rs.cancel()
		delete(l.running, key)
		if err != nil && !errors.Is(err, context.Canceled) {
			l.log.Warn("ingest session terminated with error",
				slog.String("key", key),
				slog.String("source", source),
				slog.String("error", err.Error()),
			)
		} else {
			l.log.Info("ingest session stopped",
				slog.String("key", key),
				slog.String("source", source),
			)
		}
	}
}

func (l *Loop) stopAll() {
	l.mu.Lock()
	running := make([]*runningSession, 0, len(l.running))
	for key, rs := range l.running {
		rs.cancel()
		running = append(running, rs)
		delete(l.running, key)
	}
	l.mu.Unlock()

	deadline := time.NewTimer(l.cfg.StopTimeout)
	defer deadline.Stop()

	for _, rs := range running {
		select {
		case err := <-rs.heartbeat:
			if err != nil && !errors.Is(err, context.Canceled) {
				l.log.Warn("heartbeat exit error on shutdown", slog.String("error", err.Error()))
			}
		case <-deadline.C:
			l.log.Warn("heartbeat shutdown timeout reached")
			return
		}
	}
}

func (l *Loop) stopSessionLocked(key string, rs *runningSession, reason string) {
	rs.cancel()
	delete(l.running, key)
	l.log.Info("ingest session stop requested", slog.String("key", key), slog.String("reason", reason))
}

func (l *Loop) nextPollInterval() time.Duration {
	base := l.cfg.PollInterval
	if l.cfg.PollJitter <= 0 {
		return base
	}
	shift := ((l.rng.Float64() * 2) - 1) * l.cfg.PollJitter
	delay := float64(base) * (1 + shift)
	if delay < float64(time.Millisecond) {
		return time.Millisecond
	}
	return time.Duration(delay)
}

func assignmentKey(provider, providerDeviceID string) string {
	return sanitizeProvider(provider) + "|" + strings.ToUpper(strings.TrimSpace(providerDeviceID))
}

func sanitizeProvider(provider string) string {
	return controlplane.NormalizeProvider(provider)
}

func sanitizeAssignment(a controlplane.IngestAssignment) controlplane.IngestAssignment {
	a.Provider = sanitizeProvider(a.Provider)
	a.ProviderDeviceID = strings.ToUpper(strings.TrimSpace(a.ProviderDeviceID))
	a.IngestDesiredState = strings.ToLower(strings.TrimSpace(a.IngestDesiredState))
	return a
}

func shouldRun(a controlplane.IngestAssignment) bool {
	return a.DeviceIsActive && a.CredentialIsActive && a.IngestDesiredState == "active"
}

func stopReason(a controlplane.IngestAssignment) string {
	if !a.DeviceIsActive {
		return "device_inactive"
	}
	if !a.CredentialIsActive {
		return "credential_inactive"
	}
	switch a.IngestDesiredState {
	case "paused":
		return "ingest_paused"
	case "draining":
		return "ingest_draining"
	default:
		return "ingest_not_active"
	}
}

func randomToken() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("read random token bytes: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}
