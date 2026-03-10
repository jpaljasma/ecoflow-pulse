package ingestworker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestAsyncEnvelopePublisherPublishesAndCloses(t *testing.T) {
	t.Parallel()

	var published atomic.Int64
	pub := &fakeEnvelopePublisher{
		onPublish: func(*envelopev1.TelemetryEnvelope) error {
			published.Add(1)
			return nil
		},
	}

	ap := newAsyncEnvelopePublisher(context.Background(), pub, 8, 2, 250*time.Millisecond)
	for i := 0; i < 20; i++ {
		if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id"}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}
	if err := ap.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got := published.Load(); got != 20 {
		t.Fatalf("published count mismatch: got=%d want=20", got)
	}
}

func TestAsyncEnvelopePublisherEmitsWorkerErrorAndContinues(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	pub := &fakeEnvelopePublisher{
		onPublish: func(*envelopev1.TelemetryEnvelope) error {
			if calls.Add(1) == 1 {
				return errors.New("publish failed")
			}
			return nil
		},
	}

	ap := newAsyncEnvelopePublisher(context.Background(), pub, 4, 1, 250*time.Millisecond)
	if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id"}); err != nil {
		t.Fatalf("initial Publish() error = %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("expected async error to surface")
		case err := <-ap.Errors():
			if err == nil || err.Error() != "publish failed" {
				t.Fatalf("unexpected async error: %v", err)
			}
			goto verify
		}
	}

verify:
	if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id-2"}); err != nil {
		t.Fatalf("expected Publish() to keep accepting envelopes after worker error, got=%v", err)
	}
	if err := ap.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestAsyncEnvelopePublisherTimeoutWhenQueueIsFull(t *testing.T) {
	t.Parallel()

	hold := make(chan struct{})
	pub := &fakeEnvelopePublisher{
		onPublish: func(*envelopev1.TelemetryEnvelope) error {
			<-hold
			return nil
		},
	}
	ap := newAsyncEnvelopePublisher(context.Background(), pub, 1, 1, 30*time.Millisecond)
	defer func() {
		close(hold)
		_ = ap.Close()
	}()

	if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id-1"}); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}
	if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id-2"}); err != nil {
		t.Fatalf("second Publish() should fill queue, got=%v", err)
	}
	if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id-3"}); err == nil {
		t.Fatal("expected timeout when queue is full")
	}
}

func TestAsyncEnvelopePublisherCloseCancelsBlockedWorker(t *testing.T) {
	t.Parallel()

	pub := &blockingContextPublisher{started: make(chan struct{}, 1)}

	ap := newAsyncEnvelopePublisher(context.Background(), pub, 1, 1, time.Second)
	if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id-1"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	select {
	case <-pub.started:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected worker publish to start")
	}

	done := make(chan error, 1)
	go func() {
		done <- ap.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected Close() to return after canceling blocked worker")
	}
}

func TestAsyncEnvelopePublisherPublishReturnsClosedDuringConcurrentClose(t *testing.T) {
	t.Parallel()

	pub := &blockingContextPublisher{started: make(chan struct{}, 1)}
	ap := newAsyncEnvelopePublisher(context.Background(), pub, 1, 1, time.Second)
	defer func() {
		_ = ap.Close()
	}()

	if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id-1"}); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}
	select {
	case <-pub.started:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected worker publish to start")
	}
	if err := ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id-2"}); err != nil {
		t.Fatalf("second Publish() error = %v", err)
	}

	result := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				result <- errors.New("publish panicked during close")
			}
		}()
		result <- ap.Publish(context.Background(), &envelopev1.TelemetryEnvelope{EnvelopeId: "id-3"})
	}()

	time.Sleep(20 * time.Millisecond)
	if err := ap.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err := <-result:
		if !errors.Is(err, errAsyncPublisherClosed) {
			t.Fatalf("expected errAsyncPublisherClosed, got %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected concurrent Publish() to return after Close()")
	}
}

type blockingContextPublisher struct {
	started chan struct{}
}

func (p *blockingContextPublisher) PublishEnvelope(ctx context.Context, _ *envelopev1.TelemetryEnvelope) error {
	select {
	case p.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func (p *blockingContextPublisher) Close() error { return nil }
