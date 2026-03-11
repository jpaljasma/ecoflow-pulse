package rollupworker

import (
	"context"
	"errors"
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

func TestProcessDeliverySuccessAcks(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	worker := &Worker{log: slog.Default(), store: store, cfg: DefaultConfig()}
	delivery := newFakeDelivery(t, testWorkerEnvelope(`{"params":{"wattsOutSum":35}}`))

	if err := worker.processDelivery(context.Background(), delivery); err != nil {
		t.Fatalf("processDelivery failed: %v", err)
	}
	if store.calls != 1 || delivery.acked != 1 || delivery.nacked != 0 || delivery.termed != 0 {
		t.Fatalf("unexpected delivery results calls=%d ack=%d nak=%d term=%d", store.calls, delivery.acked, delivery.nacked, delivery.termed)
	}
}

func TestProcessDeliveryInvalidProtoTerms(t *testing.T) {
	t.Parallel()
	worker := &Worker{log: slog.Default(), store: &fakeStore{}, cfg: DefaultConfig()}
	delivery := &fakeDelivery{subject: "pulse.telemetry.ingest.s001", data: []byte("not-proto")}

	if err := worker.processDelivery(context.Background(), delivery); err == nil {
		t.Fatalf("expected invalid proto error")
	}
	if delivery.termed != 1 {
		t.Fatalf("expected term on invalid proto, got=%d", delivery.termed)
	}
}

func TestProcessDeliveryNoMetricsAcks(t *testing.T) {
	t.Parallel()
	store := &fakeStore{err: ErrNoRollupMetrics}
	worker := &Worker{log: slog.Default(), store: store, cfg: DefaultConfig()}
	delivery := newFakeDelivery(t, testWorkerEnvelope(`{"params":{"icoBytes":[1,2,3]}}`))

	if err := worker.processDelivery(context.Background(), delivery); err != nil {
		t.Fatalf("expected no error for no-metrics ack path, got=%v", err)
	}
	if delivery.acked != 1 || delivery.nacked != 0 || delivery.termed != 0 {
		t.Fatalf("unexpected delivery outcome ack=%d nak=%d term=%d", delivery.acked, delivery.nacked, delivery.termed)
	}
}

func TestProcessDeliveryInvalidEnvelopeTerms(t *testing.T) {
	t.Parallel()
	store := &fakeStore{err: ErrInvalidRollupEnvelope}
	worker := &Worker{log: slog.Default(), store: store, cfg: DefaultConfig()}
	delivery := newFakeDelivery(t, testWorkerEnvelope(`{"params":{"wattsOutSum":35}}`))

	if err := worker.processDelivery(context.Background(), delivery); err == nil {
		t.Fatalf("expected invalid envelope error")
	}
	if delivery.termed != 1 {
		t.Fatalf("expected term on invalid envelope, got=%d", delivery.termed)
	}
}

func TestProcessDeliveryStoreFailureNacks(t *testing.T) {
	t.Parallel()
	store := &fakeStore{err: errors.New("db unavailable")}
	worker := &Worker{log: slog.Default(), store: store, cfg: DefaultConfig()}
	delivery := newFakeDelivery(t, testWorkerEnvelope(`{"params":{"wattsOutSum":35}}`))

	if err := worker.processDelivery(context.Background(), delivery); err == nil {
		t.Fatalf("expected store failure")
	}
	if delivery.nacked != 1 {
		t.Fatalf("expected nack on store failure, got=%d", delivery.nacked)
	}
}

type fakeStore struct {
	calls int
	err   error
}

func (f *fakeStore) ApplyEnvelope(_ context.Context, _ *envelopev1.TelemetryEnvelope) error {
	f.calls++
	return f.err
}

func (f *fakeStore) Close() error { return nil }

type fakeDelivery struct {
	subject string
	data    []byte
	acked   int
	nacked  int
	termed  int
}

func newFakeDelivery(t *testing.T, env *envelopev1.TelemetryEnvelope) *fakeDelivery {
	t.Helper()
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return &fakeDelivery{subject: "pulse.telemetry.ingest.s001", data: data}
}

func (d *fakeDelivery) Subject() string { return d.subject }
func (d *fakeDelivery) Data() []byte    { return d.data }
func (d *fakeDelivery) Ack() error {
	d.acked++
	return nil
}
func (d *fakeDelivery) Nak() error {
	d.nacked++
	return nil
}
func (d *fakeDelivery) Term() error {
	d.termed++
	return nil
}

func testWorkerEnvelope(payload string) *envelopev1.TelemetryEnvelope {
	return &envelopev1.TelemetryEnvelope{
		DeviceId:           "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		EcoflowSn:          "DEMOD2M00001057",
		ObservedTimeUnixMs: 1770000000000,
		Payload:            []byte(payload),
		Labels:             map[string]string{"provider": "ecoflow"},
	}
}

var _ = nats.Msg{}
