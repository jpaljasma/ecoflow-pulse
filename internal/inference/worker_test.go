package inference

import (
	"context"
	"log/slog"
	"testing"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type fakeResolver struct {
	ctx DeviceContext
	err error
}

func (f *fakeResolver) ResolveDeviceContext(context.Context, string) (DeviceContext, error) {
	if f.err != nil {
		return DeviceContext{}, f.err
	}
	return f.ctx, nil
}

type fakeStore struct {
	calls int
	err   error
}

func (f *fakeStore) ApplyEnvelope(_ context.Context, _ *envelopev1.TelemetryEnvelope, _ DeviceContext) (*ReadModel, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &ReadModel{}, nil
}

func (f *fakeStore) GetDeviceInsights(context.Context, string, Filter) (DeviceInsights, error) {
	return DeviceInsights{}, nil
}

func (f *fakeStore) ListFleetInsights(context.Context, []string, Filter) ([]DeviceInsights, error) {
	return nil, nil
}

func (f *fakeStore) GetReadModel(context.Context, Identity) (*ReadModel, error) {
	return nil, nil
}

func TestHandleMessageSuccessAcks(t *testing.T) {
	store := &fakeStore{}
	worker := &Worker{
		log:      slog.Default(),
		store:    store,
		resolver: &fakeResolver{ctx: DeviceContext{DeviceID: "dev-1"}},
		cfg:      DefaultWorkerConfig(),
	}
	env := &envelopev1.TelemetryEnvelope{DeviceId: "dev-1", EcoflowSn: "R351ZABAPH331057", Payload: []byte(`{"params":{"soc":54.2}}`)}
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	msg := &nats.Msg{Subject: "pulse.telemetry.ingest.s001", Data: data}
	worker.handleMessage(msg)
	if store.calls != 1 {
		t.Fatalf("store calls=%d want=1", store.calls)
	}
}

func TestHandleMessageResolverNotFoundTerms(t *testing.T) {
	worker := &Worker{
		log:      slog.Default(),
		store:    &fakeStore{},
		resolver: &fakeResolver{err: controlplane.ErrDeviceNotFound},
		cfg:      DefaultWorkerConfig(),
	}
	env := &envelopev1.TelemetryEnvelope{DeviceId: "dev-1", Payload: []byte(`{"params":{"soc":54.2}}`)}
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	msg := &nats.Msg{Subject: "pulse.telemetry.ingest.s001", Data: data}
	worker.handleMessage(msg)
}

var _ Store = (*fakeStore)(nil)
var _ DeviceContextResolver = (*fakeResolver)(nil)
