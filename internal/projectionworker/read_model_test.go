package projectionworker

import (
	"context"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestValkeySnapshotStoreReadSnapshotContract(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	ctx := context.Background()

	_, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-read-1",
		DeviceId:           "dev-read-1",
		EcoflowSn:          "demod2m00001057",
		IngestedTimeUnixMs: 1000,
		Payload:            []byte(`{"params":{"load":35,"pv":20}}`),
	})
	if err != nil {
		t.Fatalf("apply envelope: %v", err)
	}

	snap, err := store.ReadSnapshot(ctx, SnapshotIdentity{DeviceID: "dev-read-1"})
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snap == nil {
		t.Fatalf("expected read-model snapshot")
		return
	}
	if snap.DeviceID != "dev-read-1" {
		t.Fatalf("device id mismatch: got=%q want=%q", snap.DeviceID, "dev-read-1")
	}
	if got := snap.Metrics["params.load"]; got != 35 {
		t.Fatalf("load metric mismatch: got=%v want=35", got)
	}
	if got := snap.Metrics["params.pv"]; got != 20 {
		t.Fatalf("pv metric mismatch: got=%v want=20", got)
	}
	if snap.Cursor.Seq == 0 {
		t.Fatalf("expected non-zero cursor seq")
	}

	// Read contract must provide caller-isolated metric maps.
	snap.Metrics["params.load"] = 999
	reloaded, err := store.ReadSnapshot(ctx, SnapshotIdentity{DeviceID: "dev-read-1"})
	if err != nil {
		t.Fatalf("read snapshot again: %v", err)
	}
	if got := reloaded.Metrics["params.load"]; got != 35 {
		t.Fatalf("expected persisted metric unchanged after caller mutation: got=%v want=35", got)
	}
}

func TestValkeySnapshotStoreCheckpointRecoveryAcrossStoreRestart(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	ctx := context.Background()

	identity := SnapshotIdentity{DeviceID: "dev-checkpoint", EcoflowSN: "DEMOD2M00001057"}

	_, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-checkpoint-1",
		DeviceId:           identity.DeviceID,
		EcoflowSn:          identity.EcoflowSN,
		IngestedTimeUnixMs: 1000,
		Payload:            []byte(`{"params":{"load":10}}`),
	})
	if err != nil {
		t.Fatalf("apply envelope #1: %v", err)
	}
	_, err = store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-checkpoint-2",
		DeviceId:           identity.DeviceID,
		EcoflowSn:          identity.EcoflowSN,
		IngestedTimeUnixMs: 2000,
		Payload:            []byte(`{"params":{"load":20}}`),
	})
	if err != nil {
		t.Fatalf("apply envelope #2: %v", err)
	}

	// Simulate worker restart by re-instantiating store over same Valkey state.
	restarted, err := NewValkeySnapshotStore(store.client, DefaultValkeySnapshotStoreConfig())
	if err != nil {
		t.Fatalf("new restarted store: %v", err)
	}

	before, err := restarted.ReadSnapshot(ctx, identity)
	if err != nil {
		t.Fatalf("read checkpoint before replay: %v", err)
	}
	if before.Cursor.Seq != 2 {
		t.Fatalf("cursor seq mismatch before replay: got=%d want=2", before.Cursor.Seq)
	}
	if before.Cursor.TsUnixMs != 2000 {
		t.Fatalf("cursor ts mismatch before replay: got=%d want=2000", before.Cursor.TsUnixMs)
	}

	// Replay older envelope should be idempotently skipped.
	_, err = restarted.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-checkpoint-replay-old",
		DeviceId:           identity.DeviceID,
		EcoflowSn:          identity.EcoflowSN,
		IngestedTimeUnixMs: 1500,
		Payload:            []byte(`{"params":{"load":999}}`),
	})
	if err != nil {
		t.Fatalf("apply stale replay envelope: %v", err)
	}
	afterOld, err := restarted.ReadSnapshot(ctx, identity)
	if err != nil {
		t.Fatalf("read checkpoint after stale replay: %v", err)
	}
	if afterOld.Cursor.Seq != 2 {
		t.Fatalf("stale replay advanced seq: got=%d want=2", afterOld.Cursor.Seq)
	}
	if got := afterOld.Metrics["params.load"]; got != 20 {
		t.Fatalf("stale replay mutated metric: got=%v want=20", got)
	}

	// Newer envelope should advance cursor and update metrics.
	_, err = restarted.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-checkpoint-3",
		DeviceId:           identity.DeviceID,
		EcoflowSn:          identity.EcoflowSN,
		IngestedTimeUnixMs: 3000,
		Payload:            []byte(`{"params":{"load":33}}`),
	})
	if err != nil {
		t.Fatalf("apply envelope #3: %v", err)
	}
	afterNew, err := restarted.ReadSnapshot(ctx, identity)
	if err != nil {
		t.Fatalf("read checkpoint after new envelope: %v", err)
	}
	if afterNew.Cursor.Seq != 3 {
		t.Fatalf("new envelope did not advance seq: got=%d want=3", afterNew.Cursor.Seq)
	}
	if afterNew.Cursor.TsUnixMs != 3000 {
		t.Fatalf("new envelope did not update cursor ts: got=%d want=3000", afterNew.Cursor.TsUnixMs)
	}
	if got := afterNew.Metrics["params.load"]; got != 33 {
		t.Fatalf("new envelope metric mismatch: got=%v want=33", got)
	}
}

func TestValkeySnapshotStoreReadSnapshotExpiresStaleVolatileMetrics(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	store.metricFreshnessWindow = time.Minute
	ctx := context.Background()

	_, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-freshness-1",
		DeviceId:           "dev-freshness",
		EcoflowSn:          "SN-FRESHNESS",
		IngestedTimeUnixMs: 1000,
		Payload:            []byte(`{"params":{"pv1ChargeWatts":46,"wattsOutSum":12,"f32ShowSoc":81}}`),
	})
	if err != nil {
		t.Fatalf("apply first envelope: %v", err)
	}
	_, err = store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-freshness-2",
		DeviceId:           "dev-freshness",
		EcoflowSn:          "SN-FRESHNESS",
		IngestedTimeUnixMs: 70_000,
		Payload:            []byte(`{"params":{"f32ShowSoc":80}}`),
	})
	if err != nil {
		t.Fatalf("apply second envelope: %v", err)
	}

	store.nowFn = func() time.Time { return time.UnixMilli(130_001).UTC() }
	snap, err := store.ReadSnapshot(ctx, SnapshotIdentity{DeviceID: "dev-freshness"})
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snap == nil {
		t.Fatalf("expected snapshot")
	}
	if _, exists := snap.Metrics["params.pv1ChargeWatts"]; exists {
		t.Fatalf("stale PV metric should be filtered from read model: %v", snap.Metrics)
	}
	if _, exists := snap.Metrics["params.wattsOutSum"]; exists {
		t.Fatalf("stale load metric should be filtered from read model: %v", snap.Metrics)
	}
	if got := snap.Metrics["params.f32ShowSoc"]; got != 80 {
		t.Fatalf("non-volatile/fresh SOC should remain: got=%v want=80", got)
	}
	if _, exists := snap.MetricObservedAtUnixMs["params.pv1ChargeWatts"]; exists {
		t.Fatalf("filtered metric timestamp should not be exposed: %v", snap.MetricObservedAtUnixMs)
	}
}

func TestValkeySnapshotStoreReadSnapshotKeepsFreshFlatlinedMetrics(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	store.metricFreshnessWindow = time.Minute
	store.metricFlatlineWindow = 30 * time.Minute
	ctx := context.Background()

	_, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-flatline-1",
		DeviceId:           "dev-flatline",
		EcoflowSn:          "SN-FLATLINE",
		IngestedTimeUnixMs: 1000,
		Payload:            []byte(`{"params":{"pv1ChargeWatts":2,"wattsOutSum":0,"f32ShowSoc":44}}`),
	})
	if err != nil {
		t.Fatalf("apply first envelope: %v", err)
	}
	_, err = store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-flatline-2",
		DeviceId:           "dev-flatline",
		EcoflowSn:          "SN-FLATLINE",
		IngestedTimeUnixMs: 100_000,
		Payload:            []byte(`{"params":{"pv1ChargeWatts":2,"wattsOutSum":0,"f32ShowSoc":44}}`),
	})
	if err != nil {
		t.Fatalf("apply second envelope: %v", err)
	}

	store.nowFn = func() time.Time { return time.UnixMilli(130_000).UTC() }
	snap, err := store.ReadSnapshot(ctx, SnapshotIdentity{DeviceID: "dev-flatline"})
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if got := snap.Metrics["params.pv1ChargeWatts"]; got != 2 {
		t.Fatalf("fresh unchanged PV metric should remain: got=%v want=2", got)
	}
	if got := snap.MetricObservedAtUnixMs["params.pv1ChargeWatts"]; got != 100_000 {
		t.Fatalf("PV metric observed-at mismatch: got=%d want=100000", got)
	}
}

func TestValkeySnapshotStoreReadSnapshotExpiresFlatlinedCurrentMetrics(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	store.metricFreshnessWindow = time.Minute
	store.metricFlatlineWindow = 5 * time.Minute
	ctx := context.Background()

	for index, item := range []struct {
		id string
		ts int64
	}{
		{id: "env-stuck-current-1", ts: 1_000},
		{id: "env-stuck-current-2", ts: 6 * 60_000},
		{id: "env-stuck-current-3", ts: 12 * 60_000},
	} {
		_, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
			EnvelopeId:         item.id,
			DeviceId:           "dev-stuck-current",
			EcoflowSn:          "SN-STUCK-CURRENT",
			IngestedTimeUnixMs: item.ts,
			Payload:            []byte(`{"params":{"pv1ChargeWatts":46,"wattsInSum":46,"wattsOutSum":0,"remainTime":5999,"f32ShowSoc":77.5}}`),
		})
		if err != nil {
			t.Fatalf("apply stuck envelope %d: %v", index+1, err)
		}
	}

	store.nowFn = func() time.Time { return time.UnixMilli(12*60_000 + 10_000).UTC() }
	snap, err := store.ReadSnapshot(ctx, SnapshotIdentity{DeviceID: "dev-stuck-current"})
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	for _, key := range []string{"params.pv1ChargeWatts", "params.wattsInSum", "params.wattsOutSum", "params.remainTime"} {
		if _, exists := snap.Metrics[key]; exists {
			t.Fatalf("flatlined current metric %q should be filtered from read model: %v", key, snap.Metrics)
		}
		if _, exists := snap.MetricChangedAtUnixMs[key]; exists {
			t.Fatalf("filtered metric changed-at should not be exposed: %v", snap.MetricChangedAtUnixMs)
		}
	}
	if got := snap.Metrics["params.f32ShowSoc"]; got != 77.5 {
		t.Fatalf("stable SOC should remain: got=%v want=77.5", got)
	}
}

func TestValkeySnapshotStoreReadSnapshotKeepsCurrentMetricsWhenCohortMoves(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	store.metricFreshnessWindow = time.Minute
	store.metricFlatlineWindow = 5 * time.Minute
	ctx := context.Background()

	_, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-moving-current-1",
		DeviceId:           "dev-moving-current",
		EcoflowSn:          "SN-MOVING-CURRENT",
		IngestedTimeUnixMs: 1_000,
		Payload:            []byte(`{"params":{"pv1ChargeWatts":2,"wattsInSum":2,"pv1InVol":18.0,"f32ShowSoc":44}}`),
	})
	if err != nil {
		t.Fatalf("apply first envelope: %v", err)
	}
	_, err = store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-moving-current-2",
		DeviceId:           "dev-moving-current",
		EcoflowSn:          "SN-MOVING-CURRENT",
		IngestedTimeUnixMs: 7 * 60_000,
		Payload:            []byte(`{"params":{"pv1ChargeWatts":2,"wattsInSum":2,"pv1InVol":18.2,"f32ShowSoc":44}}`),
	})
	if err != nil {
		t.Fatalf("apply second envelope: %v", err)
	}

	store.nowFn = func() time.Time { return time.UnixMilli(7*60_000 + 30_000).UTC() }
	snap, err := store.ReadSnapshot(ctx, SnapshotIdentity{DeviceID: "dev-moving-current"})
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if got := snap.Metrics["params.pv1ChargeWatts"]; got != 2 {
		t.Fatalf("unchanged trickle PV should remain while current cohort is moving: got=%v want=2", got)
	}
	if got := snap.MetricChangedAtUnixMs["params.pv1ChargeWatts"]; got != 1_000 {
		t.Fatalf("unchanged PV changed-at should preserve original change time: got=%d want=1000", got)
	}
	if got := snap.MetricChangedAtUnixMs["params.pv1InVol"]; got != 7*60_000 {
		t.Fatalf("moving witness changed-at mismatch: got=%d want=%d", got, 7*60_000)
	}
}
