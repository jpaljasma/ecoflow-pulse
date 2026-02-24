package telemetrybus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func TestPublishEnvelopeRoutesToIngestShardSubject(t *testing.T) {
	t.Parallel()

	var publishedSubject string
	var publishedPayload []byte
	var publishedMsgID string
	publisher := newNATSEnvelopePublisherForTest(
		SubjectConfig{Prefix: "pulse", ShardCount: 128},
		func(msg *nats.Msg) error {
			publishedSubject = msg.Subject
			publishedPayload = append([]byte(nil), msg.Data...)
			publishedMsgID = msg.Header.Get("Nats-Msg-Id")
			return nil
		},
	)

	envelope := &envelopev1.TelemetryEnvelope{
		EnvelopeId:      "018f08b2-6ad2-7f9f-90d9-fb4ef732a3af",
		EnvelopeVersion: 1,
		Shard:           7,
		TypeCode:        "pdStatus",
		Payload:         []byte(`{"id":1}`),
	}

	if err := PublishEnvelope(context.Background(), publisher, envelope); err != nil {
		t.Fatalf("PublishEnvelope() error = %v", err)
	}
	if publishedSubject != "pulse.telemetry.ingest.s007" {
		t.Fatalf("subject mismatch: got=%q want=%q", publishedSubject, "pulse.telemetry.ingest.s007")
	}
	if publishedMsgID != envelope.EnvelopeId {
		t.Fatalf("msg-id mismatch: got=%q want=%q", publishedMsgID, envelope.EnvelopeId)
	}

	var decoded envelopev1.TelemetryEnvelope
	if err := proto.Unmarshal(publishedPayload, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded.GetTypeCode() != "pdStatus" {
		t.Fatalf("decoded envelope type mismatch: got=%q", decoded.GetTypeCode())
	}
}

func TestPublishEnvelopeHelperErrorsOnNilPublisher(t *testing.T) {
	t.Parallel()

	err := PublishEnvelope(context.Background(), nil, &envelopev1.TelemetryEnvelope{})
	if err == nil || !strings.Contains(err.Error(), "publisher is required") {
		t.Fatalf("expected nil publisher error, got=%v", err)
	}
}

func TestPublishEnvelopeErrorsOnNilEnvelope(t *testing.T) {
	t.Parallel()

	publisher := newNATSEnvelopePublisherForTest(SubjectConfig{}, func(_ *nats.Msg) error { return nil })
	err := publisher.PublishEnvelope(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "envelope is required") {
		t.Fatalf("expected nil envelope error, got=%v", err)
	}
}

func TestPublishEnvelopePropagatesPublishError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("nats down")
	publisher := newNATSEnvelopePublisherForTest(SubjectConfig{}, func(_ *nats.Msg) error { return wantErr })
	err := publisher.PublishEnvelope(context.Background(), &envelopev1.TelemetryEnvelope{
		EnvelopeVersion: 1,
		Shard:           1,
	})
	if err == nil || !strings.Contains(err.Error(), "publish envelope to nats") {
		t.Fatalf("expected publish failure, got=%v", err)
	}
}

func TestPublishEnvelopeWithOptionsStripsLabelsBeforeMarshal(t *testing.T) {
	t.Parallel()

	var publishedPayload []byte
	publisher := &NATSEnvelopePublisher{
		cfg:            SubjectConfig{Prefix: "pulse", ShardCount: 128}.Normalized(),
		subjectByShard: buildIngestSubjectCache(SubjectConfig{Prefix: "pulse", ShardCount: 128}.Normalized()),
		options:        NATSEnvelopePublisherOptions{StripLabels: true},
		publish: func(_ context.Context, msg *nats.Msg) error {
			publishedPayload = append([]byte(nil), msg.Data...)
			return nil
		},
		closeFn: func() error { return nil },
	}
	in := &envelopev1.TelemetryEnvelope{
		EnvelopeId: "env-1",
		Shard:      3,
		Labels: map[string]string{
			"provider":      "ecoflow",
			"credential_id": "cred-1",
		},
	}

	if err := publisher.PublishEnvelope(context.Background(), in); err != nil {
		t.Fatalf("PublishEnvelope() error = %v", err)
	}

	// input envelope is not mutated
	if len(in.GetLabels()) != 2 {
		t.Fatalf("expected input labels to remain intact, got=%v", in.GetLabels())
	}

	var decoded envelopev1.TelemetryEnvelope
	if err := proto.Unmarshal(publishedPayload, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got := decoded.GetLabels(); got != nil {
		t.Fatalf("expected published labels to be stripped, got=%v", got)
	}
}

func TestPublishEnvelopeRetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	publisher := newRetryTestPublisher(func(_ context.Context, _ *nats.Msg) error {
		attempts++
		if attempts <= 2 {
			return errors.New("temporary nats failure")
		}
		return nil
	})

	err := publisher.PublishEnvelope(context.Background(), &envelopev1.TelemetryEnvelope{
		EnvelopeVersion: 1,
		Shard:           2,
	})
	if err != nil {
		t.Fatalf("PublishEnvelope() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempt mismatch: got=%d want=3", attempts)
	}
}

func TestPublishEnvelopeRetryExhausted(t *testing.T) {
	t.Parallel()

	attempts := 0
	publisher := newRetryTestPublisher(func(_ context.Context, _ *nats.Msg) error {
		attempts++
		return errors.New("temporary nats failure")
	})

	err := publisher.PublishEnvelope(context.Background(), &envelopev1.TelemetryEnvelope{
		EnvelopeVersion: 1,
		Shard:           2,
	})
	if err == nil {
		t.Fatalf("expected error when retries exhausted")
	}
	if attempts != 3 {
		t.Fatalf("attempt mismatch: got=%d want=3", attempts)
	}
}

func TestPublishEnvelopeDoesNotRetryNonRetryableError(t *testing.T) {
	t.Parallel()

	attempts := 0
	publisher := newRetryTestPublisher(func(_ context.Context, _ *nats.Msg) error {
		attempts++
		return context.DeadlineExceeded
	})

	err := publisher.PublishEnvelope(context.Background(), &envelopev1.TelemetryEnvelope{
		EnvelopeVersion: 1,
		Shard:           2,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got=%v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempt mismatch: got=%d want=1", attempts)
	}
}

func newRetryTestPublisher(publishFn func(ctx context.Context, msg *nats.Msg) error) *NATSEnvelopePublisher {
	cfg := SubjectConfig{Prefix: "pulse", ShardCount: 128}.Normalized()
	return &NATSEnvelopePublisher{
		cfg:            cfg,
		subjectByShard: buildIngestSubjectCache(cfg),
		options: NATSEnvelopePublisherOptions{
			PublishMaxRetries:          2,
			PublishRetryInitialBackoff: time.Millisecond,
			PublishRetryMaxBackoff:     time.Millisecond,
		}.normalized(),
		publish:       publishFn,
		sleepFn:       func(context.Context, time.Duration) error { return nil },
		randFloat64Fn: func() float64 { return 0 },
		closeFn:       func() error { return nil },
	}
}
