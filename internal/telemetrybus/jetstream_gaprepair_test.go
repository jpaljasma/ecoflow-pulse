package telemetrybus

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestEnsureJetStreamGapRepairStreamAddsWhenMissing(t *testing.T) {
	t.Parallel()

	mgr := &fakeJetStreamManager{streamInfoErr: nats.ErrStreamNotFound}
	err := ensureJetStreamGapRepairStreamWithManager(
		context.Background(),
		mgr,
		SubjectConfig{Prefix: "pulse", ShardCount: 128},
		JetStreamGapRepairBootstrapConfig{
			Enabled:    true,
			StreamName: "PULSE_TELEMETRY_GAPREPAIR",
			Replicas:   3,
			MaxAge:     24 * time.Hour,
			Storage:    nats.FileStorage,
			Retention:  nats.WorkQueuePolicy,
		},
	)
	if err != nil {
		t.Fatalf("ensureJetStreamGapRepairStreamWithManager() error = %v", err)
	}
	if mgr.addCalls != 1 {
		t.Fatalf("expected one add call, got=%d", mgr.addCalls)
	}
	if mgr.lastAdded == nil {
		t.Fatalf("expected stream config to be added")
	}
	if got := mgr.lastAdded.Subjects; len(got) != 1 || got[0] != "pulse.telemetry.gaprepair.s*" {
		t.Fatalf("unexpected subjects=%v", got)
	}
}

func TestEnsureJetStreamGapRepairStreamUpdatesWhenConfigDiffers(t *testing.T) {
	t.Parallel()

	mgr := &fakeJetStreamManager{
		streamInfo: &nats.StreamInfo{Config: nats.StreamConfig{
			Name:      "PULSE_TELEMETRY_GAPREPAIR",
			Subjects:  []string{"pulse.telemetry.gaprepair.s*"},
			Retention: nats.WorkQueuePolicy,
			MaxAge:    12 * time.Hour,
			Storage:   nats.FileStorage,
			Replicas:  1,
		}},
	}
	err := ensureJetStreamGapRepairStreamWithManager(
		context.Background(),
		mgr,
		SubjectConfig{Prefix: "pulse", ShardCount: 128},
		JetStreamGapRepairBootstrapConfig{
			Enabled:    true,
			StreamName: "PULSE_TELEMETRY_GAPREPAIR",
			Replicas:   3,
			MaxAge:     24 * time.Hour,
			Storage:    nats.FileStorage,
			Retention:  nats.WorkQueuePolicy,
		},
	)
	if err != nil {
		t.Fatalf("ensureJetStreamGapRepairStreamWithManager() error = %v", err)
	}
	if mgr.updateCalls != 1 {
		t.Fatalf("expected one update call, got=%d", mgr.updateCalls)
	}
}

func TestEnsureJetStreamGapRepairStreamNoopWhenConfigMatches(t *testing.T) {
	t.Parallel()

	mgr := &fakeJetStreamManager{
		streamInfo: &nats.StreamInfo{Config: nats.StreamConfig{
			Name:      "PULSE_TELEMETRY_GAPREPAIR",
			Subjects:  []string{"pulse.telemetry.gaprepair.s*"},
			Retention: nats.WorkQueuePolicy,
			MaxAge:    24 * time.Hour,
			Storage:   nats.FileStorage,
			Replicas:  3,
		}},
	}
	err := ensureJetStreamGapRepairStreamWithManager(
		context.Background(),
		mgr,
		SubjectConfig{Prefix: "pulse", ShardCount: 128},
		DefaultJetStreamGapRepairBootstrapConfig(),
	)
	if err != nil {
		t.Fatalf("ensureJetStreamGapRepairStreamWithManager() error = %v", err)
	}
	if mgr.addCalls != 0 {
		t.Fatalf("expected no add calls, got=%d", mgr.addCalls)
	}
	if mgr.updateCalls != 0 {
		t.Fatalf("expected no update calls, got=%d", mgr.updateCalls)
	}
}
