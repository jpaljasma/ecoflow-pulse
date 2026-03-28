package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
	solarstore "github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd/store"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

type solarVerificationLeaseManager interface {
	Acquire(ctx context.Context, ref ingestlease.LeaseRef, workerID string, token string, opts ingestlease.CallOptions) (ingestlease.AcquireResult, error)
	RunHeartbeat(ctx context.Context, lease ingestlease.Lease, options ingestlease.HeartbeatOptions) error
}

type solarVerificationService interface {
	VerifyIssuedForecasts(ctx context.Context, before time.Time, limit int) error
}

type solarVerificationTicker struct {
	C    <-chan time.Time
	Stop func()
}

type solarVerificationTickerFactory func(time.Duration) solarVerificationTicker

type solarVerificationLoopDeps struct {
	leaseManager   solarVerificationLeaseManager
	leaseCleanup   func()
	leaseSource    string
	workerID       string
	interval       time.Duration
	batchLimit     int
	acquireRetry   time.Duration
	newTicker      solarVerificationTickerFactory
	nowFn          func() time.Time
	leaseReference ingestlease.LeaseRef
}

var solarVerificationLeaseRef = ingestlease.LeaseRef{
	Provider:         "pulse",
	ProviderDeviceID: "solar-forecast-verification",
}

func newSolarVerificationQueryReaderFromEnv(log *slog.Logger) (telemetryquery.Reader, func(), error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		log.Info("solar verifier telemetry query reader disabled", "reason", "CONTROL_PLANE_DB_DSN not set")
		return nil, func() {}, nil
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, nil, err
	}
	reader, err := telemetryquery.NewPostgresReader(dsn)
	if err != nil {
		return nil, nil, err
	}
	log.Info("solar verifier telemetry query reader enabled", "source", "postgres")
	return reader, func() { _ = reader.Close() }, nil
}

func newSolarVerificationTrainingStoreFromEnv(log *slog.Logger) (solarforecastd.TrainingStore, func(), error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		log.Info("solar verifier training store disabled", "reason", "CONTROL_PLANE_DB_DSN not set")
		return nil, func() {}, nil
	}
	store, err := solarstore.NewPostgresStore(dsn)
	if err != nil {
		return nil, nil, err
	}
	log.Info("solar verifier training store enabled", "source", "postgres")
	return store, func() { _ = store.Close() }, nil
}

func startSolarVerificationLoop(ctx context.Context, log *slog.Logger, svc solarVerificationService) func() {
	if svc == nil {
		return func() {}
	}
	interval := runtimecfg.DurationNonNegative("SOLAR_FORECAST_VERIFICATION_INTERVAL", 15*time.Minute)
	if interval <= 0 {
		log.Info("solar forecast verification loop disabled", "reason", "SOLAR_FORECAST_VERIFICATION_INTERVAL <= 0")
		return func() {}
	}
	limit := runtimecfg.IntMin("SOLAR_FORECAST_VERIFICATION_BATCH_LIMIT", 24*64, 1)
	mode := strings.ToLower(strings.TrimSpace(runtimecfg.EnvOrDefault("SOLAR_FORECAST_VERIFICATION_COORDINATION_MODE", "claim")))
	if mode == "" || mode == "claim" || mode == "parallel" || mode == "shared" {
		log.Info("solar forecast verification claim coordination enabled", "mode", mode)
		return startSolarVerificationLoopWithoutLease(ctx, log, svc, interval, limit)
	}
	leaseManager, cleanupLeaseManager, leaseSource, err := newSolarVerificationLeaseManagerFromEnv(log)
	if err != nil {
		log.Warn("solar forecast verification lease coordination unavailable, falling back to uncoordinated loop", "error", err.Error())
		return startSolarVerificationLoopWithoutLease(ctx, log, svc, interval, limit)
	}
	if leaseManager == nil {
		return startSolarVerificationLoopWithoutLease(ctx, log, svc, interval, limit)
	}
	return startSolarVerificationLoopWithLease(ctx, log, svc, solarVerificationLoopDeps{
		leaseManager:   leaseManager,
		leaseCleanup:   cleanupLeaseManager,
		leaseSource:    leaseSource,
		workerID:       solarVerificationWorkerID(),
		interval:       interval,
		batchLimit:     limit,
		acquireRetry:   5 * time.Second,
		newTicker:      defaultSolarVerificationTickerFactory,
		nowFn:          time.Now,
		leaseReference: solarVerificationLeaseRef,
	})
}

func newSolarVerificationLeaseManagerFromEnv(log *slog.Logger) (solarVerificationLeaseManager, func(), string, error) {
	valkeyAddrs := runtimecfg.SplitNonEmpty(strings.TrimSpace(os.Getenv("VALKEY_ADDRS")))
	if len(valkeyAddrs) == 0 {
		log.Info("solar forecast verification lease coordination disabled", "reason", "VALKEY_ADDRS not set")
		return nil, func() {}, "disabled", nil
	}
	cfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	cfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	cfg.Password = os.Getenv("VALKEY_PASSWORD")
	ingestlease.ConfigureSentinelFromEnv(&cfg)

	client, err := ingestlease.NewValkeyClient(cfg)
	if err != nil {
		return nil, nil, "", fmt.Errorf("create solar verifier valkey client: %w", err)
	}
	manager, err := ingestlease.NewManager(client, ingestlease.DefaultConfig())
	if err != nil {
		client.Close()
		return nil, nil, "", fmt.Errorf("create solar verifier lease manager: %w", err)
	}
	log.Info("solar forecast verification lease coordination enabled", "source", "valkey", "valkey_addrs", strings.Join(valkeyAddrs, ","))
	return manager, func() { client.Close() }, "valkey", nil
}

func startSolarVerificationLoopWithoutLease(ctx context.Context, log *slog.Logger, svc solarVerificationService, interval time.Duration, limit int) func() {
	childCtx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ticker.Stop()
		if err := svc.VerifyIssuedForecasts(childCtx, time.Now().UTC(), limit); err != nil {
			log.Warn("solar forecast verification loop failed", "error", err.Error())
		}
		for {
			select {
			case <-childCtx.Done():
				return
			case <-ticker.C:
				if err := svc.VerifyIssuedForecasts(childCtx, time.Now().UTC(), limit); err != nil {
					log.Warn("solar forecast verification loop failed", "error", err.Error())
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func startSolarVerificationLoopWithLease(ctx context.Context, log *slog.Logger, svc solarVerificationService, deps solarVerificationLoopDeps) func() {
	if svc == nil {
		return func() {}
	}
	if deps.interval <= 0 {
		return func() {}
	}
	if deps.batchLimit <= 0 {
		deps.batchLimit = 24 * 64
	}
	if deps.acquireRetry <= 0 {
		deps.acquireRetry = 5 * time.Second
	}
	if deps.newTicker == nil {
		deps.newTicker = defaultSolarVerificationTickerFactory
	}
	if strings.TrimSpace(deps.workerID) == "" {
		deps.workerID = solarVerificationWorkerID()
	}
	if deps.leaseReference == (ingestlease.LeaseRef{}) {
		deps.leaseReference = solarVerificationLeaseRef
	}

	childCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if deps.leaseCleanup != nil {
				deps.leaseCleanup()
			}
		}()

		for {
			if err := childCtx.Err(); err != nil {
				return
			}
			lease, acquired, err := acquireSolarVerificationLease(childCtx, deps.leaseManager, deps.leaseReference, deps.workerID)
			if err != nil {
				if childCtx.Err() != nil {
					return
				}
				log.Warn("solar forecast verification lease acquire failed", "error", err.Error(), "lease_source", deps.leaseSource, "worker_id", deps.workerID)
				if !waitForSolarVerificationRetry(childCtx, deps.acquireRetry) {
					return
				}
				continue
			}
			if !acquired {
				if !waitForSolarVerificationRetry(childCtx, deps.acquireRetry) {
					return
				}
				continue
			}

			log.Info("solar forecast verification lease acquired", "lease_source", deps.leaseSource, "worker_id", deps.workerID, "fence", lease.Fence)
			if !runSolarVerificationLeaseHolder(childCtx, log, svc, lease, deps) {
				return
			}
			if !waitForSolarVerificationRetry(childCtx, deps.acquireRetry) {
				return
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func acquireSolarVerificationLease(ctx context.Context, manager solarVerificationLeaseManager, ref ingestlease.LeaseRef, workerID string) (ingestlease.Lease, bool, error) {
	if manager == nil {
		return ingestlease.Lease{}, false, nil
	}
	result, err := manager.Acquire(ctx, ref, workerID, "", ingestlease.CallOptions{})
	if err != nil {
		return ingestlease.Lease{}, false, err
	}
	if !result.Acquired {
		return ingestlease.Lease{}, false, nil
	}
	return result.Lease, true, nil
}

func runSolarVerificationLeaseHolder(ctx context.Context, log *slog.Logger, svc solarVerificationService, lease ingestlease.Lease, deps solarVerificationLoopDeps) bool {
	holderCtx, holderCancel := context.WithCancel(ctx)
	defer holderCancel()

	heartbeatErrCh := make(chan error, 1)
	go func() {
		heartbeatErrCh <- deps.leaseManager.RunHeartbeat(holderCtx, lease, ingestlease.HeartbeatOptions{
			GracefulDrain:    true,
			StateDuringRenew: "active",
		})
	}()

	ticker := deps.newTicker(deps.interval)
	defer ticker.Stop()

	if err := svc.VerifyIssuedForecasts(holderCtx, deps.nowFn().UTC(), deps.batchLimit); err != nil && holderCtx.Err() == nil {
		log.Warn("solar forecast verification run failed", "error", err.Error(), "lease_source", deps.leaseSource, "worker_id", deps.workerID)
	}

	for {
		select {
		case <-ctx.Done():
			return false
		case err := <-heartbeatErrCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Warn("solar forecast verification lease heartbeat stopped", "error", err.Error(), "lease_source", deps.leaseSource, "worker_id", deps.workerID)
			}
			return true
		case <-ticker.C:
			if err := svc.VerifyIssuedForecasts(holderCtx, deps.nowFn().UTC(), deps.batchLimit); err != nil && holderCtx.Err() == nil {
				log.Warn("solar forecast verification run failed", "error", err.Error(), "lease_source", deps.leaseSource, "worker_id", deps.workerID)
			}
		}
	}
}

func waitForSolarVerificationRetry(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		delay = time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func defaultSolarVerificationTickerFactory(interval time.Duration) solarVerificationTicker {
	ticker := time.NewTicker(interval)
	return solarVerificationTicker{
		C: ticker.C,
		Stop: func() {
			ticker.Stop()
		},
	}
}

func solarVerificationWorkerID() string {
	if podName := strings.TrimSpace(os.Getenv("POD_NAME")); podName != "" {
		return podName
	}
	if hostName := strings.TrimSpace(os.Getenv("HOSTNAME")); hostName != "" {
		return hostName
	}
	if hostName, err := os.Hostname(); err == nil && strings.TrimSpace(hostName) != "" {
		return hostName
	}
	return fmt.Sprintf("solar-verifier-%d", os.Getpid())
}
