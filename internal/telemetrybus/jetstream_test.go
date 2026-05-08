package telemetrybus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestEnsureJetStreamIngestStreamAddsWhenMissing(t *testing.T) {
	t.Parallel()

	mgr := &fakeJetStreamManager{
		streamInfoErr: nats.ErrStreamNotFound,
	}
	err := ensureJetStreamIngestStreamWithManager(
		context.Background(),
		mgr,
		SubjectConfig{Prefix: "pulse", ShardCount: 128},
		JetStreamIngestBootstrapConfig{
			Enabled:    true,
			StreamName: "PULSE_TELEMETRY_INGEST",
			Replicas:   3,
			MaxAge:     24 * time.Hour,
			Storage:    nats.FileStorage,
		},
	)
	if err != nil {
		t.Fatalf("ensureJetStreamIngestStreamWithManager() error = %v", err)
	}
	if mgr.addCalls != 1 {
		t.Fatalf("expected one add call, got=%d", mgr.addCalls)
	}
	if mgr.lastAdded == nil {
		t.Fatalf("expected stream config to be added")
	}
	if got := mgr.lastAdded.Subjects; len(got) != 1 || got[0] != "pulse.telemetry.ingest.*" {
		t.Fatalf("unexpected subjects=%v", got)
	}
}

func TestEnsureJetStreamIngestStreamUpdatesWhenConfigDiffers(t *testing.T) {
	t.Parallel()

	mgr := &fakeJetStreamManager{
		streamInfo: &nats.StreamInfo{
			Config: nats.StreamConfig{
				Name:      "PULSE_TELEMETRY_INGEST",
				Subjects:  []string{"pulse.telemetry.ingest.*"},
				Retention: nats.LimitsPolicy,
				MaxAge:    12 * time.Hour,
				Storage:   nats.FileStorage,
				Replicas:  1,
			},
		},
	}

	err := ensureJetStreamIngestStreamWithManager(
		context.Background(),
		mgr,
		SubjectConfig{Prefix: "pulse", ShardCount: 128},
		JetStreamIngestBootstrapConfig{
			Enabled:    true,
			StreamName: "PULSE_TELEMETRY_INGEST",
			Replicas:   3,
			MaxAge:     24 * time.Hour,
			Storage:    nats.FileStorage,
		},
	)
	if err != nil {
		t.Fatalf("ensureJetStreamIngestStreamWithManager() error = %v", err)
	}
	if mgr.updateCalls != 1 {
		t.Fatalf("expected one update call, got=%d", mgr.updateCalls)
	}
}

func TestEnsureJetStreamIngestStreamNoopWhenConfigMatches(t *testing.T) {
	t.Parallel()

	mgr := &fakeJetStreamManager{
		streamInfo: &nats.StreamInfo{
			Config: nats.StreamConfig{
				Name:      "PULSE_TELEMETRY_INGEST",
				Subjects:  []string{"pulse.telemetry.ingest.*"},
				Retention: nats.LimitsPolicy,
				MaxAge:    12 * time.Hour,
				Storage:   nats.FileStorage,
				Replicas:  3,
			},
		},
	}

	err := ensureJetStreamIngestStreamWithManager(
		context.Background(),
		mgr,
		SubjectConfig{Prefix: "pulse", ShardCount: 128},
		DefaultJetStreamIngestBootstrapConfig(),
	)
	if err != nil {
		t.Fatalf("ensureJetStreamIngestStreamWithManager() error = %v", err)
	}
	if mgr.updateCalls != 0 {
		t.Fatalf("expected no update call, got=%d", mgr.updateCalls)
	}
	if mgr.addCalls != 0 {
		t.Fatalf("expected no add call, got=%d", mgr.addCalls)
	}
}

type fakeJetStreamManager struct {
	streamInfo    *nats.StreamInfo
	streamInfoErr error

	addCalls  int
	lastAdded *nats.StreamConfig
	addErr    error

	updateCalls int
	lastUpdated *nats.StreamConfig
	updateErr   error
}

func (f *fakeJetStreamManager) StreamInfo(_ string, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	if f.streamInfoErr != nil {
		return nil, f.streamInfoErr
	}
	if f.streamInfo == nil {
		return nil, errors.New("no stream info configured")
	}
	return f.streamInfo, nil
}

func (f *fakeJetStreamManager) AddStream(cfg *nats.StreamConfig, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.addCalls++
	f.lastAdded = cfg
	if f.addErr != nil {
		return nil, f.addErr
	}
	return &nats.StreamInfo{Config: *cfg}, nil
}

func (f *fakeJetStreamManager) UpdateStream(cfg *nats.StreamConfig, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.updateCalls++
	f.lastUpdated = cfg
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &nats.StreamInfo{Config: *cfg}, nil
}
