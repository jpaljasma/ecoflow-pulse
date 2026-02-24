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
