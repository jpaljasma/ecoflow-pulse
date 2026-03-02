package ingestworker

import (
	"context"
	"log/slog"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
)

const defaultQuotaMetricsInterval = 30 * time.Second

type quotaRefreshReason string

const (
	quotaRefreshReasonBootstrap quotaRefreshReason = "bootstrap"
	quotaRefreshReasonPeriodic  quotaRefreshReason = "periodic"
	quotaRefreshReasonStale     quotaRefreshReason = "stale_reconnect"
)

type QuotaMetrics struct {
	bootstrapAttempts atomic.Uint64
	bootstrapApplied  atomic.Uint64
	bootstrapFailures atomic.Uint64

	periodicAttempts atomic.Uint64
	periodicApplied  atomic.Uint64
	periodicFailures atomic.Uint64

	staleAttempts atomic.Uint64
	staleApplied  atomic.Uint64
	staleFailures atomic.Uint64

	fetchSuccessTotal          atomic.Uint64
	fetchFailureTotal          atomic.Uint64
	metadataUpsertFailureTotal atomic.Uint64
	buildFailureTotal          atomic.Uint64
	publishFailureTotal        atomic.Uint64
	emptyParamsTotal           atomic.Uint64
	appliedTotal               atomic.Uint64

	lastSuccessUnixMs      atomic.Int64
	lastFailureUnixMs      atomic.Int64
	lastParamsCount        atomic.Int64
	lastMetadataGroupCount atomic.Int64
	lastCapabilityCount    atomic.Int64
}

type QuotaMetricsSnapshot struct {
	BootstrapAttempts uint64
	BootstrapApplied  uint64
	BootstrapFailures uint64

	PeriodicAttempts uint64
	PeriodicApplied  uint64
	PeriodicFailures uint64

	StaleAttempts uint64
	StaleApplied  uint64
	StaleFailures uint64

	FetchSuccessTotal          uint64
	FetchFailureTotal          uint64
	MetadataUpsertFailureTotal uint64
	BuildFailureTotal          uint64
	PublishFailureTotal        uint64
	EmptyParamsTotal           uint64
	AppliedTotal               uint64

	LastSuccessAt      time.Time
	LastFailureAt      time.Time
	LastParamsCount    int64
	LastMetadataGroups int64
	LastCapabilityKeys int64
}

func DefaultQuotaMetricsInterval() time.Duration {
	return defaultQuotaMetricsInterval
}

func (m *QuotaMetrics) Snapshot() QuotaMetricsSnapshot {
	if m == nil {
		return QuotaMetricsSnapshot{}
	}
	return QuotaMetricsSnapshot{
		BootstrapAttempts:          m.bootstrapAttempts.Load(),
		BootstrapApplied:           m.bootstrapApplied.Load(),
		BootstrapFailures:          m.bootstrapFailures.Load(),
		PeriodicAttempts:           m.periodicAttempts.Load(),
		PeriodicApplied:            m.periodicApplied.Load(),
		PeriodicFailures:           m.periodicFailures.Load(),
		StaleAttempts:              m.staleAttempts.Load(),
		StaleApplied:               m.staleApplied.Load(),
		StaleFailures:              m.staleFailures.Load(),
		FetchSuccessTotal:          m.fetchSuccessTotal.Load(),
		FetchFailureTotal:          m.fetchFailureTotal.Load(),
		MetadataUpsertFailureTotal: m.metadataUpsertFailureTotal.Load(),
		BuildFailureTotal:          m.buildFailureTotal.Load(),
		PublishFailureTotal:        m.publishFailureTotal.Load(),
		EmptyParamsTotal:           m.emptyParamsTotal.Load(),
		AppliedTotal:               m.appliedTotal.Load(),
		LastSuccessAt:              unixMillisToTime(m.lastSuccessUnixMs.Load()),
		LastFailureAt:              unixMillisToTime(m.lastFailureUnixMs.Load()),
		LastParamsCount:            m.lastParamsCount.Load(),
		LastMetadataGroups:         m.lastMetadataGroupCount.Load(),
		LastCapabilityKeys:         m.lastCapabilityCount.Load(),
	}
}

func LogQuotaMetricsSnapshot(log *slog.Logger, component string, snapshot QuotaMetricsSnapshot) {
	if log == nil {
		return
	}
	log.Info("ingest_quota_metrics",
		slog.String("component", strings.TrimSpace(component)),
		slog.Uint64("bootstrap_attempts", snapshot.BootstrapAttempts),
		slog.Uint64("bootstrap_applied", snapshot.BootstrapApplied),
		slog.Uint64("bootstrap_failures", snapshot.BootstrapFailures),
		slog.Uint64("periodic_attempts", snapshot.PeriodicAttempts),
		slog.Uint64("periodic_applied", snapshot.PeriodicApplied),
		slog.Uint64("periodic_failures", snapshot.PeriodicFailures),
		slog.Uint64("stale_attempts", snapshot.StaleAttempts),
		slog.Uint64("stale_applied", snapshot.StaleApplied),
		slog.Uint64("stale_failures", snapshot.StaleFailures),
		slog.Uint64("fetch_success_total", snapshot.FetchSuccessTotal),
		slog.Uint64("fetch_failure_total", snapshot.FetchFailureTotal),
		slog.Uint64("metadata_upsert_failure_total", snapshot.MetadataUpsertFailureTotal),
		slog.Uint64("build_failure_total", snapshot.BuildFailureTotal),
		slog.Uint64("publish_failure_total", snapshot.PublishFailureTotal),
		slog.Uint64("empty_params_total", snapshot.EmptyParamsTotal),
		slog.Uint64("applied_total", snapshot.AppliedTotal),
		slog.Int64("last_params_count", snapshot.LastParamsCount),
		slog.Int64("last_metadata_groups", snapshot.LastMetadataGroups),
		slog.Int64("last_capability_keys", snapshot.LastCapabilityKeys),
		slog.Time("last_success_at", snapshot.LastSuccessAt),
		slog.Time("last_failure_at", snapshot.LastFailureAt),
	)
}

func StartQuotaMetricsReporter(
	ctx context.Context,
	log *slog.Logger,
	component string,
	metrics *QuotaMetrics,
	interval time.Duration,
) func() {
	if ctx == nil || log == nil || metrics == nil || interval <= 0 {
		return func() {}
	}
	reportCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	jitterRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	go func() {
		defer close(done)
		maxJitter := int64(interval) / 4
		if maxJitter < 1 {
			maxJitter = 1
		}
		initialJitter := time.Duration(jitterRand.Int63n(maxJitter))
		if initialJitter > 0 {
			timer := time.NewTimer(initialJitter)
			select {
			case <-reportCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			LogQuotaMetricsSnapshot(log, component, metrics.Snapshot())
			select {
			case <-reportCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (m *QuotaMetrics) recordAttempt(reason quotaRefreshReason) {
	if m == nil {
		return
	}
	switch reason {
	case quotaRefreshReasonBootstrap:
		m.bootstrapAttempts.Add(1)
	case quotaRefreshReasonPeriodic:
		m.periodicAttempts.Add(1)
	case quotaRefreshReasonStale:
		m.staleAttempts.Add(1)
	}
}

func (m *QuotaMetrics) recordFailure(reason quotaRefreshReason, observedAt time.Time) {
	if m == nil {
		return
	}
	switch reason {
	case quotaRefreshReasonBootstrap:
		m.bootstrapFailures.Add(1)
	case quotaRefreshReasonPeriodic:
		m.periodicFailures.Add(1)
	case quotaRefreshReasonStale:
		m.staleFailures.Add(1)
	}
	m.fetchFailureTotal.Add(1)
	m.lastFailureUnixMs.Store(observedAt.UTC().UnixMilli())
}

func (m *QuotaMetrics) recordMetadataUpsertFailure() {
	if m == nil {
		return
	}
	m.metadataUpsertFailureTotal.Add(1)
}

func (m *QuotaMetrics) recordBuildFailure() {
	if m == nil {
		return
	}
	m.buildFailureTotal.Add(1)
}

func (m *QuotaMetrics) recordPublishFailure() {
	if m == nil {
		return
	}
	m.publishFailureTotal.Add(1)
}

func (m *QuotaMetrics) recordEmptyParams() {
	if m == nil {
		return
	}
	m.emptyParamsTotal.Add(1)
}

func (m *QuotaMetrics) recordApplied(reason quotaRefreshReason, observedAt time.Time, paramsCount, metadataGroups, capabilityCount int) {
	if m == nil {
		return
	}
	switch reason {
	case quotaRefreshReasonBootstrap:
		m.bootstrapApplied.Add(1)
	case quotaRefreshReasonPeriodic:
		m.periodicApplied.Add(1)
	case quotaRefreshReasonStale:
		m.staleApplied.Add(1)
	}
	m.fetchSuccessTotal.Add(1)
	m.appliedTotal.Add(1)
	m.lastSuccessUnixMs.Store(observedAt.UTC().UnixMilli())
	m.lastParamsCount.Store(int64(paramsCount))
	m.lastMetadataGroupCount.Store(int64(metadataGroups))
	m.lastCapabilityCount.Store(int64(capabilityCount))
}

func unixMillisToTime(unixMs int64) time.Time {
	if unixMs <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(unixMs).UTC()
}
