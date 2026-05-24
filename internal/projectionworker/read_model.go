package projectionworker

import (
	"context"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/currenttelemetry"
)

// SnapshotIdentity identifies one live snapshot read target.
// device_id is preferred when available; ecoflow_sn is a fallback identity.
type SnapshotIdentity struct {
	DeviceID  string
	EcoflowSN string
}

func (s SnapshotIdentity) normalized() SnapshotIdentity {
	return SnapshotIdentity{
		DeviceID:  strings.TrimSpace(s.DeviceID),
		EcoflowSN: strings.ToUpper(strings.TrimSpace(s.EcoflowSN)),
	}
}

// SnapshotCursor is the read-model cursor consumed by downstream query/realtime
// consumers. Seq is monotonic per projected device stream.
type SnapshotCursor struct {
	Seq      uint64
	TsUnixMs int64
}

// SnapshotReadModel is the downstream snapshot contract consumed by realtime
// and query paths.
type SnapshotReadModel struct {
	DeviceID               string
	EcoflowSN              string
	Cursor                 SnapshotCursor
	Metrics                map[string]float64
	MetricObservedAtUnixMs map[string]int64
	MetricChangedAtUnixMs  map[string]int64
}

// SnapshotReader is consumed by downstream services (for example gRPC query
// APIs) that need latest live metrics without coupling to projection internals.
type SnapshotReader interface {
	ReadSnapshot(ctx context.Context, identity SnapshotIdentity) (*SnapshotReadModel, error)
}

// ReadSnapshot returns the downstream read-model contract from the persisted
// live snapshot state.
func (s *ValkeySnapshotStore) ReadSnapshot(ctx context.Context, identity SnapshotIdentity) (*SnapshotReadModel, error) {
	identity = identity.normalized()
	snapshot, err := s.GetSnapshot(ctx, identity.DeviceID, identity.EcoflowSN)
	if err != nil || snapshot == nil {
		return nil, err
	}
	return toSnapshotReadModel(snapshot, s.nowFn().UTC().UnixMilli(), s.metricFreshnessWindow, s.metricFlatlineWindow), nil
}

func toSnapshotReadModel(snapshot *LiveSnapshot, nowUnixMs int64, freshnessWindow time.Duration, flatlineWindow time.Duration) *SnapshotReadModel {
	if snapshot == nil {
		return nil
	}
	freshness := buildMetricFreshnessContext(snapshot, flatlineWindow)
	out := &SnapshotReadModel{
		DeviceID:  snapshot.DeviceID,
		EcoflowSN: snapshot.EcoflowSN,
		Cursor: SnapshotCursor{
			Seq:      snapshot.CursorSeq,
			TsUnixMs: snapshot.CursorTsUnixMs,
		},
		Metrics:                make(map[string]float64, len(snapshot.Metrics)),
		MetricObservedAtUnixMs: make(map[string]int64, len(snapshot.Metrics)),
		MetricChangedAtUnixMs:  make(map[string]int64, len(snapshot.Metrics)),
	}
	for k, v := range snapshot.Metrics {
		observedAt := snapshot.MetricObservedAtUnixMs[k]
		if observedAt <= 0 {
			observedAt = snapshot.CursorTsUnixMs
		}
		changedAt := snapshot.MetricChangedAtUnixMs[k]
		if changedAt <= 0 {
			changedAt = observedAt
		}
		if metricExpired(k, observedAt, changedAt, nowUnixMs, freshnessWindow, freshness) {
			continue
		}
		out.Metrics[k] = v
		if observedAt > 0 {
			out.MetricObservedAtUnixMs[k] = observedAt
		}
		if changedAt > 0 {
			out.MetricChangedAtUnixMs[k] = changedAt
		}
	}
	return out
}

type metricFreshnessContext struct {
	currentTelemetryFlatlined bool
	currentTelemetryIdleStale bool
}

func buildMetricFreshnessContext(snapshot *LiveSnapshot, flatlineWindow time.Duration) metricFreshnessContext {
	if snapshot == nil || flatlineWindow <= 0 {
		return metricFreshnessContext{}
	}
	var newestObservedAt int64
	var newestChangedAt int64
	for key := range snapshot.Metrics {
		if !currenttelemetry.IsCurrentMetricKey(key) {
			continue
		}
		observedAt := snapshot.MetricObservedAtUnixMs[key]
		if observedAt <= 0 {
			observedAt = snapshot.CursorTsUnixMs
		}
		changedAt := snapshot.MetricChangedAtUnixMs[key]
		if changedAt <= 0 {
			changedAt = observedAt
		}
		if observedAt > newestObservedAt {
			newestObservedAt = observedAt
		}
		if changedAt > newestChangedAt {
			newestChangedAt = changedAt
		}
	}
	return metricFreshnessContext{
		currentTelemetryFlatlined: newestObservedAt > 0 &&
			newestChangedAt > 0 &&
			newestObservedAt-newestChangedAt > flatlineWindow.Milliseconds(),
		currentTelemetryIdleStale: currenttelemetry.IdleStale(snapshot.Metrics),
	}
}

func metricExpired(
	key string,
	observedAtUnixMs int64,
	changedAtUnixMs int64,
	nowUnixMs int64,
	freshnessWindow time.Duration,
	freshness metricFreshnessContext,
) bool {
	if freshnessWindow <= 0 || !isVolatileMetricKey(key) || observedAtUnixMs <= 0 || nowUnixMs <= 0 {
		return false
	}
	if nowUnixMs-observedAtUnixMs > freshnessWindow.Milliseconds() {
		return true
	}
	if !currenttelemetry.IsCurrentMetricKey(key) {
		return false
	}
	return (freshness.currentTelemetryFlatlined && changedAtUnixMs > 0) || freshness.currentTelemetryIdleStale
}

func isVolatileMetricKey(key string) bool {
	clean := strings.ToLower(strings.TrimSpace(key))
	if clean == "" {
		return false
	}
	switch clean {
	case "pvw", "acw", "dcw", "loadw", "batteryw", "tempc", "temp", "temperature":
		return true
	}
	if strings.Contains(clean, "remaintime") {
		return true
	}
	if strings.Contains(clean, "watt") || strings.Contains(clean, "pwr") {
		return true
	}
	if strings.Contains(clean, "mppt") {
		return true
	}
	if strings.Contains(clean, ".pv") && (strings.Contains(clean, "invol") ||
		strings.Contains(clean, "inamp") ||
		strings.Contains(clean, "inwatt") ||
		strings.Contains(clean, "chargewatt") ||
		strings.Contains(clean, "chgstate")) {
		return true
	}
	if strings.HasSuffix(clean, ".invol") ||
		strings.HasSuffix(clean, ".inamp") ||
		strings.HasSuffix(clean, ".chgstate") ||
		strings.HasSuffix(clean, ".dcoutstate") ||
		strings.HasSuffix(clean, ".cfgacenabled") {
		return true
	}
	if strings.Contains(clean, ".out") && (strings.Contains(clean, "vol") ||
		strings.Contains(clean, "amp") ||
		strings.Contains(clean, "state")) {
		return true
	}
	if strings.HasSuffix(clean, ".batvol") || strings.HasSuffix(clean, ".batamp") {
		return true
	}
	return false
}
