package telemetrybus

import (
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type fakeIngestQueueSubscriber struct {
	errors []error
	calls  int
}

func (f *fakeIngestQueueSubscriber) QueueSubscribe(string, string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) {
	f.calls++
	if f.calls <= len(f.errors) && f.errors[f.calls-1] != nil {
		return nil, f.errors[f.calls-1]
	}
	return &nats.Subscription{}, nil
}

func TestQueueSubscribeIngestFallsBackToExistingDurableBindOnDeliverPolicyDrift(t *testing.T) {
	t.Parallel()

	js := &fakeIngestQueueSubscriber{
		errors: []error{errors.New("nats: configuration requests deliver policy to be 2, but consumer's value is 0")},
	}

	sub, err := QueueSubscribeIngest(js, IngestQueueSubscribeConfig{
		SubjectConfig: SubjectConfig{Prefix: "pulse", ShardCount: 128},
		StreamName:    "PULSE_TELEMETRY_INGEST",
		Durable:       "rollup-timeseries-v1",
		QueueGroup:    "rollup-timeseries",
		AckWait:       30 * time.Second,
		MaxAckPending: 4096,
	}, func(*nats.Msg) {})
	if err != nil {
		t.Fatalf("QueueSubscribeIngest() error = %v", err)
	}
	if sub == nil {
		t.Fatal("QueueSubscribeIngest() returned nil subscription")
	}
	if js.calls != 2 {
		t.Fatalf("expected create-or-bind attempt plus fallback bind, got %d calls", js.calls)
	}
}

func TestQueueSubscribeIngestDoesNotFallbackOnUnrelatedSubscribeError(t *testing.T) {
	t.Parallel()

	js := &fakeIngestQueueSubscriber{
		errors: []error{errors.New("nats: permissions violation")},
	}

	_, err := QueueSubscribeIngest(js, IngestQueueSubscribeConfig{
		SubjectConfig: SubjectConfig{Prefix: "pulse", ShardCount: 128},
		StreamName:    "PULSE_TELEMETRY_INGEST",
		Durable:       "archive-raw-v1",
		QueueGroup:    "archive-raw",
		AckWait:       60 * time.Second,
		MaxAckPending: 4096,
	}, func(*nats.Msg) {})
	if err == nil {
		t.Fatal("expected subscribe error")
	}
	if js.calls != 1 {
		t.Fatalf("expected one subscribe attempt for unrelated error, got %d calls", js.calls)
	}
}
