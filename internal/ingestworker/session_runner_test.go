package ingestworker

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

func TestNewEcoFlowSessionRunnerRequiresPublisher(t *testing.T) {
	t.Parallel()

	_, err := NewEcoFlowSessionRunner(testLogger(), nil, nil, EcoFlowSessionConfig{})
	if err == nil {
		t.Fatalf("expected constructor to fail without publisher")
	}
}

func TestNewEcoFlowSessionRunnerRejectsUnorderedPublishWithoutOptIn(t *testing.T) {
	t.Parallel()

	_, err := NewEcoFlowSessionRunner(testLogger(), nil, &fakeEnvelopePublisher{}, EcoFlowSessionConfig{
		PublishWorkers: 4,
	})
	if err == nil {
		t.Fatalf("expected constructor to fail when publish_workers>1 without allow_unordered_publish")
	}
}

func TestNewEcoFlowSessionRunnerAllowsUnorderedPublishWithOptIn(t *testing.T) {
	t.Parallel()

	_, err := NewEcoFlowSessionRunner(testLogger(), nil, &fakeEnvelopePublisher{}, EcoFlowSessionConfig{
		PublishWorkers:        4,
		AllowUnorderedPublish: true,
	})
	if err != nil {
		t.Fatalf("expected constructor to succeed with unordered publish opt-in, got=%v", err)
	}
}

func TestEcoFlowSessionRunnerRunPublishesEnvelope(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := &fakeEnvelopePublisher{
		onPublish: func(*envelopev1.TelemetryEnvelope) error {
			cancel()
			return nil
		},
	}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, EcoFlowSessionConfig{
		ShardCount:          64,
		ReconnectJitter:     0,
		ReconnectMaxBackoff: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewEcoFlowSessionRunner() error = %v", err)
	}

	resolver := &fakeCertResolver{
		cert: ecoflow.GeneralInfoMQTTCertification{
			CertificateAccount:  "open-account",
			CertificatePassword: "secret",
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
		},
	}
	subscriber := &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{
				msg: ecoflowmqtt.Message{
					Topic:   "/open/open-account/R351ZABAPH331057/quota",
					Payload: []byte(`{"id":8221,"time":17072442,"typeCode":"pdStatus","cmdId":1,"cmdFunc":2}`),
				},
			},
		},
	}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}

	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = func(context.Context, time.Duration) error { return nil }

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "R351ZABAPH331057",
		DeviceID:           "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:       "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
		AccessKey:          "ak",
		SecretKey:          "sk",
		CredentialIsActive: true,
	}

	if err := runner.Run(ctx, assignment); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := resolver.calls.Load(); got != 1 {
		t.Fatalf("expected one certification call, got=%d", got)
	}
	if got := len(factory.configs); got != 1 {
		t.Fatalf("expected one subscriber init, got=%d", got)
	}
	if got := factory.configs[0].ClientID; got != ecoflowmqtt.BuildClientIDFromSN(assignment.ProviderDeviceID) {
		t.Fatalf("unexpected client id: got=%q want=%q", got, ecoflowmqtt.BuildClientIDFromSN(assignment.ProviderDeviceID))
	}
	if got := publisher.publishCount.Load(); got != 1 {
		t.Fatalf("expected one envelope publish, got=%d", got)
	}
	if subscriber.closeCalls.Load() == 0 {
		t.Fatalf("expected subscriber close to be called")
	}
}

func TestEcoFlowSessionRunnerReconnectsAfterReadFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := &fakeEnvelopePublisher{
		onPublish: func(*envelopev1.TelemetryEnvelope) error {
			cancel()
			return nil
		},
	}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, EcoFlowSessionConfig{
		ReconnectInitialBackoff: time.Millisecond,
		ReconnectMaxBackoff:     2 * time.Millisecond,
		ReconnectJitter:         0,
	})
	if err != nil {
		t.Fatalf("NewEcoFlowSessionRunner() error = %v", err)
	}

	resolver := &fakeCertResolver{
		cert: ecoflow.GeneralInfoMQTTCertification{
			CertificateAccount:  "open-account",
			CertificatePassword: "secret",
			URL:                 "mqtt.ecoflow.com",
			Port:                "8883",
		},
	}
	subscriber1 := &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{err: io.EOF},
		},
	}
	subscriber2 := &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{
				msg: ecoflowmqtt.Message{
					Topic:   "/open/open-account/Y711ZABA9H2P0294/quota",
					Payload: []byte(`{"id":1,"typeCode":"kitInfo"}`),
				},
			},
		},
	}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber1, subscriber2}}
	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = func(context.Context, time.Duration) error { return nil }

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "Y711ZABA9H2P0294",
		DeviceID:           "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:       "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
		AccessKey:          "ak",
		SecretKey:          "sk",
		CredentialIsActive: true,
	}

	if err := runner.Run(ctx, assignment); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := resolver.calls.Load(); got < 2 {
		t.Fatalf("expected certification refresh after reconnect, got=%d", got)
	}
	if got := len(factory.configs); got < 2 {
		t.Fatalf("expected at least two subscriber inits, got=%d", got)
	}
}

func TestEcoFlowSessionRunnerRejectsUnsupportedProvider(t *testing.T) {
	t.Parallel()

	publisher := &fakeEnvelopePublisher{}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, EcoFlowSessionConfig{})
	if err != nil {
		t.Fatalf("NewEcoFlowSessionRunner() error = %v", err)
	}
	err = runner.Run(context.Background(), controlplane.IngestAssignment{
		Provider: "victron",
	})
	if err == nil {
		t.Fatalf("expected unsupported provider error")
	}
}

type fakeCertResolver struct {
	cert  ecoflow.GeneralInfoMQTTCertification
	err   error
	calls atomic.Int64
}

func (f *fakeCertResolver) GetMQTTCertification(_ context.Context, _ controlplane.ProviderCredential, _ string) (ecoflow.GeneralInfoMQTTCertification, error) {
	f.calls.Add(1)
	if f.err != nil {
		return ecoflow.GeneralInfoMQTTCertification{}, f.err
	}
	return f.cert, nil
}

type fakeReadResult struct {
	msg ecoflowmqtt.Message
	err error
}

type fakeMQTTSubscriber struct {
	connectErr   error
	subscribeErr error

	mu    sync.Mutex
	reads []fakeReadResult

	closeCalls atomic.Int64
}

func (f *fakeMQTTSubscriber) Connect(context.Context) error {
	return f.connectErr
}

func (f *fakeMQTTSubscriber) Subscribe(context.Context, string, byte) error {
	return f.subscribeErr
}

func (f *fakeMQTTSubscriber) ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error) {
	f.mu.Lock()
	if len(f.reads) > 0 {
		next := f.reads[0]
		f.reads = f.reads[1:]
		f.mu.Unlock()
		if next.err != nil {
			return ecoflowmqtt.Message{}, next.err
		}
		return next.msg, nil
	}
	f.mu.Unlock()
	<-ctx.Done()
	return ecoflowmqtt.Message{}, ctx.Err()
}

func (f *fakeMQTTSubscriber) Disconnect() error {
	return nil
}

func (f *fakeMQTTSubscriber) Close() error {
	f.closeCalls.Add(1)
	return nil
}

type fakeSubscriberFactory struct {
	mu          sync.Mutex
	subscribers []mqttSubscriber
	configs     []ecoflowmqtt.Config
}

func (f *fakeSubscriberFactory) new(cfg ecoflowmqtt.Config) (mqttSubscriber, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs = append(f.configs, cfg)
	if len(f.subscribers) == 0 {
		return nil, errors.New("no fake subscriber configured")
	}
	sub := f.subscribers[0]
	f.subscribers = f.subscribers[1:]
	return sub, nil
}

type fakeEnvelopePublisher struct {
	onPublish func(*envelopev1.TelemetryEnvelope) error

	publishCount atomic.Int64
}

func (f *fakeEnvelopePublisher) PublishEnvelope(_ context.Context, envelope *envelopev1.TelemetryEnvelope) error {
	f.publishCount.Add(1)
	if f.onPublish == nil {
		return nil
	}
	return f.onPublish(envelope)
}

func (f *fakeEnvelopePublisher) Close() error { return nil }
