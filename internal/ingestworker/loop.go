package ingestworker

import (
	"context"
	"errors"
	"log/slog"
	mathrand "math/rand"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
)

const (
	defaultPollInterval               = 5 * time.Second
	defaultPollJitter                 = 0.20
	defaultStopTimeout                = 8 * time.Second
	defaultLeaseMissingAlertWindow    = 5 * time.Minute
	defaultLeaseMissingAlertThreshold = 4
	defaultLeaseMissingAlertCooldown  = 2 * time.Minute
	minStartWorkers                   = 8
	maxStartWorkers                   = 64
	startWorkersPerP                  = 4
	startQueueFactor                  = 8
	defaultTermQueueCap               = 4096
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
	WorkerID                   string
	ProviderFilter             string
	PollInterval               time.Duration
	PollJitter                 float64
	StopTimeout                time.Duration
	StartWorkers               int
	StartQueueSize             int
	LeaseMissingAlertWindow    time.Duration
	LeaseMissingAlertThreshold int
	LeaseMissingAlertCooldown  time.Duration
	AutoscaleMetrics           *AutoscaleMetrics
}

func DefaultConfig(workerID string) Config {
	workers := RecommendedStartWorkers(runtime.GOMAXPROCS(0))
	return Config{
		WorkerID:                   strings.TrimSpace(workerID),
		PollInterval:               defaultPollInterval,
		PollJitter:                 defaultPollJitter,
		StopTimeout:                defaultStopTimeout,
		StartWorkers:               workers,
		StartQueueSize:             RecommendedStartQueueSize(workers),
		LeaseMissingAlertWindow:    defaultLeaseMissingAlertWindow,
		LeaseMissingAlertThreshold: defaultLeaseMissingAlertThreshold,
		LeaseMissingAlertCooldown:  defaultLeaseMissingAlertCooldown,
	}
}

// RecommendedStartWorkers returns the default startup worker pool size.
// Policy: 4*GOMAXPROCS, clamped to [8,64].
func RecommendedStartWorkers(goMaxProcs int) int {
	if goMaxProcs <= 0 {
		goMaxProcs = runtime.GOMAXPROCS(0)
	}
	workers := startWorkersPerP * goMaxProcs
	if workers < minStartWorkers {
		return minStartWorkers
	}
	if workers > maxStartWorkers {
		return maxStartWorkers
	}
	return workers
}

// RecommendedStartQueueSize returns the default bounded startup queue size.
// Policy: workers*8.
func RecommendedStartQueueSize(workers int) int {
	if workers <= 0 {
		workers = RecommendedStartWorkers(0)
	}
	return workers * startQueueFactor
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
	tokenID atomic.Uint64
	tokenNS string
	termCh  chan terminationEvent
	runDone chan struct{}

	leaseMissingTracker *reconnectRateTracker
	autoscaleMetrics    *AutoscaleMetrics
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
	if cfg.StartWorkers <= 0 {
		cfg.StartWorkers = RecommendedStartWorkers(0)
	}
	if cfg.StartQueueSize < 0 {
		cfg.StartQueueSize = 0
	}
	if cfg.StartQueueSize == 0 {
		cfg.StartQueueSize = RecommendedStartQueueSize(cfg.StartWorkers)
	}
	if cfg.LeaseMissingAlertWindow <= 0 {
		cfg.LeaseMissingAlertWindow = defaultLeaseMissingAlertWindow
	}
	if cfg.LeaseMissingAlertThreshold <= 0 {
		cfg.LeaseMissingAlertThreshold = defaultLeaseMissingAlertThreshold
	}
	if cfg.LeaseMissingAlertCooldown <= 0 {
		cfg.LeaseMissingAlertCooldown = defaultLeaseMissingAlertCooldown
	}

	return &Loop{
		log:     log,
		store:   store,
		leases:  leases,
		runner:  runner,
		cfg:     cfg,
		rng:     mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
		running: make(map[string]*runningSession),
		tokenNS: cfg.WorkerID + "-",
		termCh:  make(chan terminationEvent, defaultTermQueueCap),
		runDone: make(chan struct{}),
		leaseMissingTracker: newReconnectRateTracker(
			cfg.LeaseMissingAlertWindow,
			cfg.LeaseMissingAlertThreshold,
			cfg.LeaseMissingAlertCooldown,
		),
		autoscaleMetrics: cfg.AutoscaleMetrics,
	}, nil
}

func (l *Loop) Run(ctx context.Context) error {
	if l.autoscaleMetrics != nil {
		l.autoscaleMetrics.SetPollInterval(l.cfg.PollInterval)
	}
	defer close(l.runDone)
	if err := l.reconcile(ctx); err != nil {
		l.log.Warn("ingest worker initial reconcile failed", slog.String("error", err.Error()))
	}
	l.drainTerminated()

	timer := time.NewTimer(l.nextPollInterval())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			l.stopAll()
			return nil
		case evt := <-l.termCh:
			l.handleTerminationEvent(evt)
			l.drainTerminated()
		case <-timer.C:
			if err := l.reconcile(ctx); err != nil {
				l.log.Warn("ingest worker reconcile failed", slog.String("error", err.Error()))
			}
			timer.Reset(l.nextPollInterval())
		}
	}
}

func (l *Loop) reconcile(ctx context.Context) error {
	startedAt := time.Now()
	defer func() {
		if l.autoscaleMetrics != nil {
			l.autoscaleMetrics.ObserveReconcileDuration(time.Since(startedAt))
		}
	}()
	l.drainTerminated()

	assignments, err := l.store.ListIngestAssignments(ctx, controlplane.ListIngestAssignmentsInput{
		Provider:   l.cfg.ProviderFilter,
		ActiveOnly: true,
	})
	if err != nil {
		return err
	}

	latest := make(map[string]int, len(assignments))
	for i := range assignments {
		assignments[i] = sanitizeAssignment(assignments[i])
		a := assignments[i]
		if a.Provider == "" || a.ProviderDeviceID == "" {
			continue
		}
		key := assignmentKey(a.Provider, a.ProviderDeviceID)
		if _, exists := latest[key]; exists {
			continue
		}
		latest[key] = i
	}

	stopEvents := make([]stopEvent, 0, 4)
	l.mu.Lock()
	for key, running := range l.running {
		idx, exists := latest[key]
		if !exists {
			l.stopSessionLocked(key, running)
			stopEvents = append(stopEvents, stopEvent{key: key, reason: "assignment_missing"})
			continue
		}
		a := assignments[idx]
		if !shouldRun(a) {
			reason := stopReason(a)
			l.stopSessionLocked(key, running)
			stopEvents = append(stopEvents, stopEvent{key: key, reason: reason})
			continue
		}
		if !sameRuntimeAssignment(running.assignment, a) {
			l.stopSessionLocked(key, running)
			stopEvents = append(stopEvents, stopEvent{key: key, reason: "assignment_updated"})
			continue
		}
		// Already running and still valid; remove from latest so we only attempt
		// starts for new sessions.
		delete(latest, key)
	}
	l.mu.Unlock()
	for i := range stopEvents {
		l.log.Info("ingest session stop requested",
			slog.String("key", stopEvents[i].key),
			slog.String("reason", stopEvents[i].reason),
		)
	}

	toStart := make([]controlplane.IngestAssignment, 0, len(latest))
	for _, idx := range latest {
		a := assignments[idx]
		if !shouldRun(a) {
			continue
		}
		toStart = append(toStart, a)
	}
	if l.autoscaleMetrics != nil {
		l.autoscaleMetrics.SetUnassignedActiveDevices(len(toStart))
	}
	l.startSessions(ctx, toStart)

	l.drainTerminated()
	return nil
}

type stopEvent struct {
	key    string
	reason string
}

type terminationEvent struct {
	key    string
	token  string
	source string
	err    error
}

func (l *Loop) startSessions(ctx context.Context, assignments []controlplane.IngestAssignment) {
	if len(assignments) == 0 {
		return
	}
	if len(assignments) == 1 {
		l.startSession(ctx, assignments[0])
		return
	}
	workers := l.cfg.StartWorkers
	if workers <= 0 {
		workers = RecommendedStartWorkers(0)
	}
	if workers > len(assignments) {
		workers = len(assignments)
	}
	queueSize := l.cfg.StartQueueSize
	if queueSize <= 0 {
		queueSize = RecommendedStartQueueSize(workers)
	}
	if queueSize > len(assignments) {
		queueSize = len(assignments)
	}
	if queueSize <= 0 {
		queueSize = 1
	}

	jobs := make(chan controlplane.IngestAssignment, queueSize)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for a := range jobs {
				l.startSession(ctx, a)
			}
		}()
	}
	for i := range assignments {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case jobs <- assignments[i]:
		}
	}
	close(jobs)
	wg.Wait()
}

func (l *Loop) startSession(ctx context.Context, a controlplane.IngestAssignment) {
	token := l.nextToken()

	acquireStarted := time.Now()
	result, err := l.leases.Acquire(ctx, ingestlease.LeaseRef{
		Provider:         a.Provider,
		ProviderDeviceID: a.ProviderDeviceID,
	}, l.cfg.WorkerID, token, ingestlease.CallOptions{})
	if l.autoscaleMetrics != nil {
		l.autoscaleMetrics.ObserveLeaseAcquireLatency(time.Since(acquireStarted))
	}
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
		err := l.runner.Run(runCtx, a)
		rs.done <- err
		l.emitTermination(terminationEvent{
			key:    assignmentKey(a.Provider, a.ProviderDeviceID),
			token:  result.Lease.Token,
			source: "session",
			err:    err,
		})
	}()
	go func() {
		err := l.leases.RunHeartbeat(runCtx, result.Lease, ingestlease.HeartbeatOptions{
			GracefulDrain: true,
		})
		rs.heartbeat <- err
		l.emitTermination(terminationEvent{
			key:    assignmentKey(a.Provider, a.ProviderDeviceID),
			token:  result.Lease.Token,
			source: "heartbeat",
			err:    err,
		})
	}()

	l.mu.Lock()
	key := assignmentKey(a.Provider, a.ProviderDeviceID)
	if existing, exists := l.running[key]; exists {
		l.stopSessionLocked(key, existing)
	}
	l.running[key] = rs
	l.mu.Unlock()

	l.log.Info("ingest session started",
		slog.String("provider", a.Provider),
		slog.String("provider_device_id", a.ProviderDeviceID),
		slog.String("worker_id", l.cfg.WorkerID),
	)
}

func (l *Loop) emitTermination(evt terminationEvent) {
	select {
	case l.termCh <- evt:
	case <-l.runDone:
	}
}

func (l *Loop) drainTerminated() {
	for {
		select {
		case evt := <-l.termCh:
			l.handleTerminationEvent(evt)
		default:
			return
		}
	}
}

func (l *Loop) handleTerminationEvent(evt terminationEvent) {
	var (
		handled bool
		err     error
		source  string
	)

	l.mu.Lock()
	rs, ok := l.running[evt.key]
	if ok && rs.lease.Token == evt.token {
		rs.cancel()
		delete(l.running, evt.key)
		handled = true
		err = evt.err
		source = evt.source
	}
	l.mu.Unlock()
	if !handled {
		return
	}

	if err != nil && !errors.Is(err, context.Canceled) {
		if source == "heartbeat" && ingestlease.IsLeaseRenewMissing(err) {
			count, perMinute, spike := l.leaseMissingTracker.Record(time.Now().UTC())
			l.log.Warn("ingest session terminated with lease-missing heartbeat error",
				slog.String("key", evt.key),
				slog.String("source", source),
				slog.String("error", err.Error()),
				slog.Int("lease_missing_events_in_window", count),
				slog.Float64("lease_missing_events_per_min", perMinute),
				slog.Duration("lease_missing_window", l.cfg.LeaseMissingAlertWindow),
			)
			if spike {
				l.log.Warn("ingest lease-missing heartbeat-rate spike detected",
					slog.Int("lease_missing_events_in_window", count),
					slog.Float64("lease_missing_events_per_min", perMinute),
					slog.Duration("window", l.cfg.LeaseMissingAlertWindow),
					slog.Int("threshold", l.cfg.LeaseMissingAlertThreshold),
					slog.Duration("cooldown", l.cfg.LeaseMissingAlertCooldown),
				)
			}
			return
		}
		l.log.Warn("ingest session terminated with error",
			slog.String("key", evt.key),
			slog.String("source", source),
			slog.String("error", err.Error()),
		)
		return
	}
	l.log.Info("ingest session stopped",
		slog.String("key", evt.key),
		slog.String("source", source),
	)
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
		select {
		case err := <-rs.done:
			if err != nil && !errors.Is(err, context.Canceled) {
				l.log.Warn("session exit error on shutdown", slog.String("error", err.Error()))
			}
		case <-deadline.C:
			l.log.Warn("session shutdown timeout reached")
			return
		}
	}
}

func (l *Loop) stopSessionLocked(key string, rs *runningSession) {
	rs.cancel()
	delete(l.running, key)
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
	a.DeviceID = strings.TrimSpace(a.DeviceID)
	a.CredentialID = strings.TrimSpace(a.CredentialID)
	a.ProductName = strings.TrimSpace(a.ProductName)
	a.Model = strings.TrimSpace(a.Model)
	a.AccessKey = strings.TrimSpace(a.AccessKey)
	a.SecretKey = strings.TrimSpace(a.SecretKey)
	a.IngestDesiredState = strings.ToLower(strings.TrimSpace(a.IngestDesiredState))
	return a
}

func sameRuntimeAssignment(current, next controlplane.IngestAssignment) bool {
	return current.Provider == next.Provider &&
		current.ProviderDeviceID == next.ProviderDeviceID &&
		current.CredentialID == next.CredentialID &&
		current.AccessKey == next.AccessKey &&
		current.SecretKey == next.SecretKey
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

func (l *Loop) nextToken() string {
	seq := l.tokenID.Add(1)
	return l.tokenNS + strconv.FormatUint(seq, 36)
}
