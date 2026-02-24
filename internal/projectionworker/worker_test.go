package projectionworker

import (
	"context"
	"log/slog"
	"testing"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func TestNewWorkerValidation(t *testing.T) {
	t.Parallel()

	if _, err := New(nil, nil, nil, Config{}); err == nil {
		t.Fatalf("expected error for nil nats connection")
	}
}

func TestHandleMessageValidEnvelopeAppliesStore(t *testing.T) {
	t.Parallel()

	store := &fakeSnapshotStore{}
	worker := &Worker{
		log:   slog.Default(),
		store: store,
		cfg:   DefaultConfig(),
	}
	env := &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-1",
		DeviceId:           "dev-1",
		EcoflowSn:          "R351ZABAPH331057",
		IngestedTimeUnixMs: 1234,
		Payload:            []byte(`{"params":{"wattsOutSum":35}}`),
	}
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	worker.handleMessage(&nats.Msg{Subject: "pulse.telemetry.ingest.s001", Data: data})
	if store.calls != 1 {
		t.Fatalf("expected one store apply call, got=%d", store.calls)
	}
	if store.last == nil || store.last.GetEnvelopeId() != "env-1" {
		t.Fatalf("expected store apply envelope env-1")
	}
}

func TestHandleMessageInvalidEnvelopeSkipsStore(t *testing.T) {
	t.Parallel()

	store := &fakeSnapshotStore{}
	worker := &Worker{
		log:   slog.Default(),
		store: store,
		cfg:   DefaultConfig(),
	}
	worker.handleMessage(&nats.Msg{Subject: "pulse.telemetry.ingest.s001", Data: []byte("not-proto")})
	if store.calls != 0 {
		t.Fatalf("expected zero store apply calls, got=%d", store.calls)
	}
}

func TestHandleMessageCheckpointRecoveryAcrossWorkerRestart(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	worker := &Worker{
		log:   slog.Default(),
		store: store,
		cfg:   DefaultConfig(),
	}

	push := func(t *testing.T, env *envelopev1.TelemetryEnvelope) {
		t.Helper()
		data, err := proto.Marshal(env)
		if err != nil {
			t.Fatalf("marshal envelope %q: %v", env.GetEnvelopeId(), err)
		}
		worker.handleMessage(&nats.Msg{
			Subject: "pulse.telemetry.ingest.s007",
			Data:    data,
		})
	}

	identity := SnapshotIdentity{DeviceID: "dev-recovery", EcoflowSN: "R351ZABAPH331057"}

	push(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-r1",
		DeviceId:           identity.DeviceID,
		EcoflowSn:          identity.EcoflowSN,
		IngestedTimeUnixMs: 1000,
		Payload:            []byte(`{"params":{"load":10}}`),
	})
	push(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-r2",
		DeviceId:           identity.DeviceID,
		EcoflowSn:          identity.EcoflowSN,
		IngestedTimeUnixMs: 2000,
		Payload:            []byte(`{"params":{"load":20}}`),
	})

	beforeRestart, err := store.ReadSnapshot(context.Background(), identity)
	if err != nil {
		t.Fatalf("read snapshot before restart: %v", err)
	}
	if beforeRestart.Cursor.Seq != 2 {
		t.Fatalf("cursor seq before restart mismatch: got=%d want=2", beforeRestart.Cursor.Seq)
	}

	// Simulate worker restart: new worker instance, same store.
	worker = &Worker{
		log:   slog.Default(),
		store: store,
		cfg:   DefaultConfig(),
	}

	push(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-r-old",
		DeviceId:           identity.DeviceID,
		EcoflowSn:          identity.EcoflowSN,
		IngestedTimeUnixMs: 1500,
		Payload:            []byte(`{"params":{"load":999}}`),
	})
	afterStaleReplay, err := store.ReadSnapshot(context.Background(), identity)
	if err != nil {
		t.Fatalf("read snapshot after stale replay: %v", err)
	}
	if afterStaleReplay.Cursor.Seq != 2 {
		t.Fatalf("stale replay advanced cursor seq: got=%d want=2", afterStaleReplay.Cursor.Seq)
	}
	if got := afterStaleReplay.Metrics["params.load"]; got != 20 {
		t.Fatalf("stale replay mutated metric: got=%v want=20", got)
	}

	push(t, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-r3",
		DeviceId:           identity.DeviceID,
		EcoflowSn:          identity.EcoflowSN,
		IngestedTimeUnixMs: 3000,
		Payload:            []byte(`{"params":{"load":30}}`),
	})
	afterFresh, err := store.ReadSnapshot(context.Background(), identity)
	if err != nil {
		t.Fatalf("read snapshot after fresh envelope: %v", err)
	}
	if afterFresh.Cursor.Seq != 3 {
		t.Fatalf("fresh envelope did not advance cursor seq: got=%d want=3", afterFresh.Cursor.Seq)
	}
	if got := afterFresh.Metrics["params.load"]; got != 30 {
		t.Fatalf("fresh envelope metric mismatch: got=%v want=30", got)
	}
}

type fakeSnapshotStore struct {
	calls int
	last  *envelopev1.TelemetryEnvelope
}

func (f *fakeSnapshotStore) ApplyEnvelope(_ context.Context, env *envelopev1.TelemetryEnvelope) (*LiveSnapshot, error) {
	f.calls++
	f.last = env
	return &LiveSnapshot{}, nil
}

func (f *fakeSnapshotStore) GetSnapshot(context.Context, string, string) (*LiveSnapshot, error) {
	return nil, nil
}
