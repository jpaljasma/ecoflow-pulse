package projectionworker

import (
	"context"
	"testing"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestValkeySnapshotStoreReadSnapshotContract(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	ctx := context.Background()

	_, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-read-1",
		DeviceId:           "dev-read-1",
		EcoflowSn:          "r351zabaph331057",
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

	identity := SnapshotIdentity{DeviceID: "dev-checkpoint", EcoflowSN: "R351ZABAPH331057"}

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
