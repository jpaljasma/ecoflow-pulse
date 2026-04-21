package telemetrybus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type fakeQueueSubscription struct {
	mu           sync.Mutex
	valid        bool
	unsubscribed int
}

func (f *fakeQueueSubscription) Unsubscribe() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.valid = false
	f.unsubscribed++
	return nil
}

func (f *fakeQueueSubscription) IsValid() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.valid
}

type fakeConsumerInfoProvider struct {
	mu     sync.Mutex
	errors []error
	calls  int
}

func (f *fakeConsumerInfoProvider) ConsumerInfo(string, string, ...nats.JSOpt) (*nats.ConsumerInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var err error
	if f.calls < len(f.errors) {
		err = f.errors[f.calls]
	}
	f.calls++
	if err != nil {
		return nil, err
	}
	return &nats.ConsumerInfo{}, nil
}

func TestRunConsumerSupervisorResubscribesWhenConsumerDisappears(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu            sync.Mutex
		subs          []*fakeQueueSubscription
		subscribeCall int
	)
	subscribe := func(nats.MsgHandler) (QueueSubscription, error) {
		mu.Lock()
		defer mu.Unlock()
		subscribeCall++
		sub := &fakeQueueSubscription{valid: true}
		subs = append(subs, sub)
		if subscribeCall == 2 {
			cancel()
		}
		return sub, nil
	}
	provider := &fakeConsumerInfoProvider{
		errors: []error{nil, nats.ErrConsumerNotFound},
	}

	if err := RunConsumerSupervisor(
		ctx,
		nil,
		provider,
		subscribe,
		func(*nats.Msg) {},
		NewMsgHandlerTracker(),
		ConsumerSupervisorConfig{
			StreamName:      "PULSE_TELEMETRY_INGEST",
			Durable:         "projection-live-v1",
			MonitorInterval: 10 * time.Millisecond,
			RetryBase:       10 * time.Millisecond,
			RetryMax:        20 * time.Millisecond,
			DrainTimeout:    10 * time.Millisecond,
		},
	); err != nil {
		t.Fatalf("RunConsumerSupervisor() error = %v", err)
	}

	if subscribeCall < 2 {
		t.Fatalf("expected resubscribe after consumer loss, got %d subscribe calls", subscribeCall)
	}
	if len(subs) == 0 || subs[0].unsubscribed == 0 {
		t.Fatal("expected first subscription to be cleaned up before resubscribe")
	}
}

func TestRunConsumerSupervisorRetriesInitialSubscribeFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempts := 0
	subscribe := func(nats.MsgHandler) (QueueSubscription, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("boom")
		}
		cancel()
		return &fakeQueueSubscription{valid: true}, nil
	}

	if err := RunConsumerSupervisor(
		ctx,
		nil,
		&fakeConsumerInfoProvider{},
		subscribe,
		func(*nats.Msg) {},
		NewMsgHandlerTracker(),
		ConsumerSupervisorConfig{
			StreamName:      "PULSE_TELEMETRY_INGEST",
			Durable:         "archive-raw-v1",
			MonitorInterval: 10 * time.Millisecond,
			RetryBase:       10 * time.Millisecond,
			RetryMax:        20 * time.Millisecond,
			DrainTimeout:    10 * time.Millisecond,
		},
	); err != nil {
		t.Fatalf("RunConsumerSupervisor() error = %v", err)
	}

	if attempts < 2 {
		t.Fatalf("expected subscribe retry, got %d attempts", attempts)
	}
}

func TestRunConsumerSupervisorKeepsSubscriptionOnTransientConsumerCheckError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()

	var (
		mu            sync.Mutex
		subs          []*fakeQueueSubscription
		subscribeCall int
	)
	subscribe := func(nats.MsgHandler) (QueueSubscription, error) {
		mu.Lock()
		defer mu.Unlock()
		subscribeCall++
		sub := &fakeQueueSubscription{valid: true}
		subs = append(subs, sub)
		return sub, nil
	}
	provider := &fakeConsumerInfoProvider{
		errors: []error{errors.New("temporary jetstream api error"), nil, nil},
	}

	if err := RunConsumerSupervisor(
		ctx,
		nil,
		provider,
		subscribe,
		func(*nats.Msg) {},
		NewMsgHandlerTracker(),
		ConsumerSupervisorConfig{
			StreamName:      "PULSE_TELEMETRY_INGEST",
			Durable:         "rollup-timeseries-v1",
			MonitorInterval: 10 * time.Millisecond,
			RetryBase:       10 * time.Millisecond,
			RetryMax:        20 * time.Millisecond,
			DrainTimeout:    10 * time.Millisecond,
		},
	); err != nil {
		t.Fatalf("RunConsumerSupervisor() error = %v", err)
	}

	if subscribeCall != 1 {
		t.Fatalf("expected to keep the original subscription on transient check errors, got %d subscribe calls", subscribeCall)
	}
	if provider.calls < 2 {
		t.Fatalf("expected repeated consumer health checks, got %d calls", provider.calls)
	}
	if len(subs) != 1 {
		t.Fatalf("expected one live subscription, got %d", len(subs))
	}
}
