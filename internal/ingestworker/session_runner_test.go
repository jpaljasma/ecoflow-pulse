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

	_, err := NewEcoFlowSessionRunner(testLogger(), nil, nil, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{})
	if err == nil {
		t.Fatalf("expected constructor to fail without publisher")
	}
}

func TestNewEcoFlowSessionRunnerRejectsUnorderedPublishWithoutOptIn(t *testing.T) {
	t.Parallel()

	_, err := NewEcoFlowSessionRunner(testLogger(), nil, &fakeEnvelopePublisher{}, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
		PublishWorkers: 4,
	})
	if err == nil {
		t.Fatalf("expected constructor to fail when publish_workers>1 without allow_unordered_publish")
	}
}

func TestNewEcoFlowSessionRunnerAllowsUnorderedPublishWithOptIn(t *testing.T) {
	t.Parallel()

	_, err := NewEcoFlowSessionRunner(testLogger(), nil, &fakeEnvelopePublisher{}, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
		PublishWorkers:        4,
		AllowUnorderedPublish: true,
	})
	if err != nil {
		t.Fatalf("expected constructor to succeed with unordered publish opt-in, got=%v", err)
	}
}

func TestNewEcoFlowSessionRunnerRequiresProviderDeviceUpdater(t *testing.T) {
	t.Parallel()

	_, err := NewEcoFlowSessionRunner(testLogger(), nil, &fakeEnvelopePublisher{}, nil, EcoFlowSessionConfig{})
	if err == nil {
		t.Fatalf("expected constructor to fail without provider device updater")
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
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
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
					Topic:   "/open/open-account/DEMOD2M00001057/quota",
					Payload: []byte(`{"id":8221,"time":17072442,"typeCode":"pdStatus","cmdId":1,"cmdFunc":2}`),
				},
			},
		},
	}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}

	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = testSessionSleep

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMOD2M00001057",
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
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
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
					Topic:   "/open/open-account/DEMODPU0000294/quota",
					Payload: []byte(`{"id":1,"typeCode":"kitInfo"}`),
				},
			},
		},
	}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber1, subscriber2}}
	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = testSessionSleep

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMODPU0000294",
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

func TestEcoFlowSessionRunnerDoesNotReconnectOnPublishFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var publishCalls atomic.Int64
	publisher := &fakeEnvelopePublisher{
		onPublish: func(*envelopev1.TelemetryEnvelope) error {
			if publishCalls.Add(1) == 1 {
				return errors.New("nats unavailable")
			}
			cancel()
			return nil
		},
	}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
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
	subscriber := &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{msg: ecoflowmqtt.Message{Topic: "/open/open-account/DEMODPU0000294/quota", Payload: []byte(`{"id":1,"typeCode":"kitInfo"}`)}},
			{msg: ecoflowmqtt.Message{Topic: "/open/open-account/DEMODPU0000294/quota", Payload: []byte(`{"id":2,"typeCode":"pdStatus"}`)}},
		},
	}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}
	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = testSessionSleep

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMODPU0000294",
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
		t.Fatalf("expected single certification call with no reconnect, got=%d", got)
	}
	if got := len(factory.configs); got != 1 {
		t.Fatalf("expected single subscriber init with no reconnect, got=%d", got)
	}
	if got := publisher.publishCount.Load(); got != 2 {
		t.Fatalf("expected two publish attempts, got=%d", got)
	}
}

func TestEcoFlowSessionRunnerStopsOnInvalidAccessKeyBusinessError(t *testing.T) {
	t.Parallel()

	publisher := &fakeEnvelopePublisher{}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
		ReconnectInitialBackoff: time.Millisecond,
		ReconnectMaxBackoff:     2 * time.Millisecond,
		ReconnectJitter:         0,
	})
	if err != nil {
		t.Fatalf("NewEcoFlowSessionRunner() error = %v", err)
	}

	resolver := &fakeCertResolver{
		err: &ecoflow.BusinessError{
			Code:    "8513",
			Message: "accessKey is invalid",
		},
	}
	runner.adapter = resolver
	sleepCalls := atomic.Int64{}
	runner.sleepFn = func(context.Context, time.Duration) error {
		sleepCalls.Add(1)
		return nil
	}

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMODPU0000294",
		DeviceID:           "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:       "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
		AccessKey:          "ak",
		SecretKey:          "sk",
		CredentialIsActive: true,
	}

	err = runner.Run(context.Background(), assignment)
	if !errors.Is(err, ErrEcoFlowCredentialRejected) {
		t.Fatalf("expected ErrEcoFlowCredentialRejected, got=%v", err)
	}
	if got := resolver.calls.Load(); got != 1 {
		t.Fatalf("expected one certification call, got=%d", got)
	}
	if got := sleepCalls.Load(); got != 0 {
		t.Fatalf("expected no reconnect sleep on invalid access key, got=%d", got)
	}
}

func TestEcoFlowSessionRunnerRejectsUnsupportedProvider(t *testing.T) {
	t.Parallel()

	publisher := &fakeEnvelopePublisher{}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{})
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

func TestEcoFlowSessionRunnerPublishesQuotaBootstrapAndUpsertsMetadata(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := &fakeEnvelopePublisher{
		onPublish: func(env *envelopev1.TelemetryEnvelope) error {
			if env.GetPayloadType() == "ecoflow.mqtt.raw" {
				cancel()
			}
			return nil
		},
	}
	updater := &fakeProviderDeviceUpdater{}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, updater, EcoFlowSessionConfig{
		ReconnectJitter:      0,
		ReconnectMaxBackoff:  5 * time.Millisecond,
		QuotaRefreshInterval: time.Hour,
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
		device: controlplane.ProviderDevice{
			ProductName: "Renamed Delta 2 Max",
			Model:       "DELTA 2 Max",
		},
		quota: map[string]string{
			"pd.soc":                         "35",
			"pd.wattsInSum":                  "123.5",
			"hs_yj751_pd_appshow_addr.bpNum": "2",
			"pd.dcOutState":                  "1",
		},
	}
	subscriber := &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{
				msg: ecoflowmqtt.Message{
					Topic:   "/open/open-account/DEMOD2M00001057/quota",
					Payload: []byte(`{"id":8221,"typeCode":"pdStatus"}`),
				},
			},
		},
	}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}
	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = testSessionSleep

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMOD2M00001057",
		DeviceID:           "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:       "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
		ProductName:        "Kitchen Delta 2 Max",
		Model:              "delta2max",
		AccessKey:          "ak",
		SecretKey:          "sk",
		CredentialIsActive: true,
		DeviceIsActive:     true,
		IngestDesiredState: "active",
	}

	if err := runner.Run(ctx, assignment); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := resolver.quotaCalls.Load(); got < 1 {
		t.Fatalf("expected at least one quota call, got=%d", got)
	}
	if got := updater.calls.Load(); got < 1 {
		t.Fatalf("expected at least one provider device upsert, got=%d", got)
	}
	inputs := updater.snapshot()
	if len(inputs) == 0 {
		t.Fatalf("expected recorded provider device upsert input")
	}
	if got := inputs[0].Capabilities["battery_pack_count"]; got != int64(2) {
		t.Fatalf("expected quota-derived battery pack count=2, got=%v", got)
	}
	if got := inputs[0].ProductName; got != "Renamed Delta 2 Max" {
		t.Fatalf("expected refreshed product name from provider, got=%q", got)
	}
	if got := inputs[0].Model; got != "DELTA 2 Max" {
		t.Fatalf("expected refreshed model from provider, got=%q", got)
	}
	settings, ok := inputs[0].Metadata["settings"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata settings map, got=%T", inputs[0].Metadata["settings"])
	}
	if got := settings["dc_out_state"]; got != int64(1) {
		t.Fatalf("expected metadata dc_out_state=1, got=%v", got)
	}
	published := publisher.snapshot()
	if len(published) < 2 {
		t.Fatalf("expected quota + mqtt publishes, got=%d", len(published))
	}
	if published[0].GetPayloadType() != "ecoflow.quota.normalized" {
		t.Fatalf("expected first publish to be quota bootstrap, got=%q", published[0].GetPayloadType())
	}
	metrics := runner.QuotaMetrics().Snapshot()
	if metrics.BootstrapApplied < 1 {
		t.Fatalf("expected bootstrap applied metric, got=%d", metrics.BootstrapApplied)
	}
	if metrics.FetchSuccessTotal < 1 {
		t.Fatalf("expected fetch success metric, got=%d", metrics.FetchSuccessTotal)
	}
	if metrics.LastMetadataGroups < 1 {
		t.Fatalf("expected metadata group count to be tracked, got=%d", metrics.LastMetadataGroups)
	}
}

func TestEcoFlowSessionRunnerQuotaFetchErrorIsNonFatal(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := &fakeEnvelopePublisher{
		onPublish: func(env *envelopev1.TelemetryEnvelope) error {
			if env.GetPayloadType() == "ecoflow.mqtt.raw" {
				cancel()
			}
			return nil
		},
	}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
		ReconnectJitter:      0,
		ReconnectMaxBackoff:  5 * time.Millisecond,
		QuotaRefreshInterval: time.Hour,
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
		quotaErr: errors.New("quota unavailable"),
	}
	subscriber := &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{msg: ecoflowmqtt.Message{Topic: "/open/open-account/DEMOD2M00001057/quota", Payload: []byte(`{"id":1,"typeCode":"pdStatus"}`)}},
		},
	}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}
	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = testSessionSleep

	if err := runner.Run(ctx, controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMOD2M00001057",
		DeviceID:           "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:       "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
		AccessKey:          "ak",
		SecretKey:          "sk",
		CredentialIsActive: true,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := publisher.publishCount.Load(); got != 1 {
		t.Fatalf("expected only mqtt publish when quota fails, got=%d", got)
	}
	metrics := runner.QuotaMetrics().Snapshot()
	if metrics.BootstrapFailures < 1 {
		t.Fatalf("expected bootstrap failure metric, got=%d", metrics.BootstrapFailures)
	}
	if metrics.FetchFailureTotal < 1 {
		t.Fatalf("expected fetch failure metric, got=%d", metrics.FetchFailureTotal)
	}
}

func TestEcoFlowSessionRunnerPeriodicQuotaRefresh(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := &fakeEnvelopePublisher{}
	publisher.onPublish = func(env *envelopev1.TelemetryEnvelope) error {
		if env.GetPayloadType() == "ecoflow.quota.normalized" {
			// stop after startup quota + first periodic refresh
			if publisher.publishCount.Load() >= 2 {
				cancel()
			}
		}
		return nil
	}
	updater := &fakeProviderDeviceUpdater{}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, updater, EcoFlowSessionConfig{
		ReconnectJitter:      0,
		ReconnectMaxBackoff:  5 * time.Millisecond,
		QuotaRefreshInterval: 2 * time.Millisecond,
		QuotaRefreshJitter:   0,
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
		quota: map[string]string{"pd.soc": "35"},
	}
	subscriber := &fakeMQTTSubscriber{}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}
	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = sessionSleepContext

	_ = runner.Run(ctx, controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMOD2M00001057",
		DeviceID:           "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:       "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
		AccessKey:          "ak",
		SecretKey:          "sk",
		CredentialIsActive: true,
	})
	if got := resolver.quotaCalls.Load(); got < 2 {
		t.Fatalf("expected periodic quota refresh, got=%d calls", got)
	}
	metrics := runner.QuotaMetrics().Snapshot()
	if metrics.PeriodicApplied < 1 {
		t.Fatalf("expected periodic applied metric, got=%d", metrics.PeriodicApplied)
	}
}

func TestEcoFlowSessionRunnerRefreshesQuotaOnReadFailureBeforeReconnect(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := &fakeEnvelopePublisher{
		onPublish: func(env *envelopev1.TelemetryEnvelope) error {
			if env.GetPayloadType() == "ecoflow.mqtt.raw" {
				cancel()
			}
			return nil
		},
	}
	runner, err := NewEcoFlowSessionRunner(testLogger(), nil, publisher, &fakeProviderDeviceUpdater{}, EcoFlowSessionConfig{
		ReconnectInitialBackoff: time.Millisecond,
		ReconnectMaxBackoff:     2 * time.Millisecond,
		ReconnectJitter:         0,
		QuotaRefreshInterval:    time.Hour,
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
		quota: map[string]string{"pd.soc": "35"},
	}
	subscriber1 := &fakeMQTTSubscriber{
		reads: []fakeReadResult{{err: io.EOF}},
	}
	subscriber2 := &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{msg: ecoflowmqtt.Message{Topic: "/open/open-account/DEMODPU0000294/quota", Payload: []byte(`{"id":1,"typeCode":"kitInfo"}`)}},
		},
	}
	factory := &fakeSubscriberFactory{subscribers: []mqttSubscriber{subscriber1, subscriber2}}
	runner.adapter = resolver
	runner.newSubscriber = factory.new
	runner.sleepFn = testSessionSleep

	assignment := controlplane.IngestAssignment{
		Provider:           controlplane.ProviderEcoFlow,
		ProviderDeviceID:   "DEMODPU0000294",
		DeviceID:           "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:       "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
		AccessKey:          "ak",
		SecretKey:          "sk",
		CredentialIsActive: true,
	}

	if err := runner.Run(ctx, assignment); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := resolver.quotaCalls.Load(); got < 3 {
		t.Fatalf("expected startup quota + stale quota + reconnect bootstrap quota, got=%d", got)
	}
	published := publisher.snapshot()
	quotaPublishes := 0
	for _, env := range published {
		if env.GetPayloadType() == "ecoflow.quota.normalized" {
			quotaPublishes++
		}
	}
	if quotaPublishes < 3 {
		t.Fatalf("expected at least three quota publishes across stale reconnect path, got=%d", quotaPublishes)
	}
	metrics := runner.QuotaMetrics().Snapshot()
	if metrics.StaleApplied < 1 {
		t.Fatalf("expected stale reconnect applied metric, got=%d", metrics.StaleApplied)
	}
	if metrics.BootstrapApplied < 2 {
		t.Fatalf("expected bootstrap applied metric for initial + reconnect, got=%d", metrics.BootstrapApplied)
	}
}

func TestDefaultEcoFlowSessionConfigReconnectAlertDefaults(t *testing.T) {
	t.Parallel()

	cfg := DefaultEcoFlowSessionConfig()
	if cfg.KeepAlive != 90*time.Second {
		t.Fatalf("keepalive default mismatch: got=%s want=90s", cfg.KeepAlive)
	}
	if cfg.ReadTimeout != 45*time.Second {
		t.Fatalf("read timeout default mismatch: got=%s want=45s", cfg.ReadTimeout)
	}
	if cfg.ReconnectAlertWindow != 5*time.Minute {
		t.Fatalf("reconnect alert window mismatch: got=%s want=5m", cfg.ReconnectAlertWindow)
	}
	if cfg.ReconnectAlertThreshold != 8 {
		t.Fatalf("reconnect alert threshold mismatch: got=%d want=8", cfg.ReconnectAlertThreshold)
	}
	if cfg.ReconnectAlertCooldown != 2*time.Minute {
		t.Fatalf("reconnect alert cooldown mismatch: got=%s want=2m", cfg.ReconnectAlertCooldown)
	}
	if cfg.AuthAlertWindow != 10*time.Minute {
		t.Fatalf("auth alert window mismatch: got=%s want=10m", cfg.AuthAlertWindow)
	}
	if cfg.AuthAlertThreshold != 5 {
		t.Fatalf("auth alert threshold mismatch: got=%d want=5", cfg.AuthAlertThreshold)
	}
	if cfg.AuthAlertCooldown != 5*time.Minute {
		t.Fatalf("auth alert cooldown mismatch: got=%s want=5m", cfg.AuthAlertCooldown)
	}
	if cfg.QuotaFetchTimeout != 10*time.Second {
		t.Fatalf("quota fetch timeout mismatch: got=%s want=10s", cfg.QuotaFetchTimeout)
	}
	if cfg.QuotaRefreshInterval != 30*time.Second {
		t.Fatalf("quota refresh interval mismatch: got=%s want=30s", cfg.QuotaRefreshInterval)
	}
	if cfg.QuotaRefreshJitter != 0.20 {
		t.Fatalf("quota refresh jitter mismatch: got=%v want=0.20", cfg.QuotaRefreshJitter)
	}
	if cfg.LogMQTTPayloadDebug {
		t.Fatalf("mqtt payload debug should default to false")
	}
	if cfg.LogMQTTPayloadSampleEvery != 100 {
		t.Fatalf("mqtt payload sample default mismatch: got=%d want=100", cfg.LogMQTTPayloadSampleEvery)
	}
}

func TestReconnectRateTrackerAlertingAndCooldown(t *testing.T) {
	t.Parallel()

	base := time.Unix(1_700_000_000, 0).UTC()
	tracker := newReconnectRateTracker(5*time.Minute, 3, 2*time.Minute)
	if tracker == nil {
		t.Fatal("tracker should not be nil")
	}

	count, perMin, spike := tracker.Record(base)
	if count != 1 || spike {
		t.Fatalf("first event mismatch: count=%d spike=%v", count, spike)
	}
	if perMin <= 0 {
		t.Fatalf("per-minute rate should be positive, got=%v", perMin)
	}

	count, _, spike = tracker.Record(base.Add(20 * time.Second))
	if count != 2 || spike {
		t.Fatalf("second event mismatch: count=%d spike=%v", count, spike)
	}

	count, _, spike = tracker.Record(base.Add(40 * time.Second))
	if count != 3 || !spike {
		t.Fatalf("third event should trigger spike: count=%d spike=%v", count, spike)
	}

	// Cooldown suppresses duplicate alerts while still tracking count.
	count, _, spike = tracker.Record(base.Add(60 * time.Second))
	if count != 4 || spike {
		t.Fatalf("cooldown event mismatch: count=%d spike=%v", count, spike)
	}

	// After cooldown expires and threshold still exceeded, alert should fire again.
	count, _, spike = tracker.Record(base.Add(3 * time.Minute))
	if count < 3 || !spike {
		t.Fatalf("post-cooldown event should trigger spike: count=%d spike=%v", count, spike)
	}

	// Old events should age out of the window.
	count, _, spike = tracker.Record(base.Add(10 * time.Minute))
	if count != 1 || spike {
		t.Fatalf("aged-out window mismatch: count=%d spike=%v", count, spike)
	}
}

func TestIsMQTTReadEOF(t *testing.T) {
	t.Parallel()

	if !isMQTTReadEOF(io.EOF) {
		t.Fatal("expected io.EOF to be recognized as mqtt read eof")
	}
	if !isMQTTReadEOF(errors.New("read mqtt message: EOF")) {
		t.Fatal("expected wrapped read eof string to be recognized")
	}
	if isMQTTReadEOF(errors.New("connect rejected, return code=5")) {
		t.Fatal("did not expect non-read error to be classified as mqtt read eof")
	}
}

type fakeCertResolver struct {
	cert       ecoflow.GeneralInfoMQTTCertification
	err        error
	device     controlplane.ProviderDevice
	quota      map[string]string
	quotaErr   error
	calls      atomic.Int64
	quotaCalls atomic.Int64
}

func (f *fakeCertResolver) GetMQTTCertification(_ context.Context, _ controlplane.ProviderCredential, _ string) (ecoflow.GeneralInfoMQTTCertification, error) {
	f.calls.Add(1)
	if f.err != nil {
		return ecoflow.GeneralInfoMQTTCertification{}, f.err
	}
	return f.cert, nil
}

func (f *fakeCertResolver) GetDeviceQuotaSnapshot(
	_ context.Context,
	_ controlplane.ProviderCredential,
	_ string,
) (controlplane.ProviderDevice, map[string]string, error) {
	f.quotaCalls.Add(1)
	if f.quotaErr != nil {
		return controlplane.ProviderDevice{}, nil, f.quotaErr
	}
	out := make(map[string]string, len(f.quota))
	for k, v := range f.quota {
		out[k] = v
	}
	return f.device, out, nil
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

func testSessionSleep(ctx context.Context, duration time.Duration) error {
	if duration <= 10*time.Millisecond {
		return nil
	}
	<-ctx.Done()
	return ctx.Err()
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
	mu           sync.Mutex
	envelopes    []*envelopev1.TelemetryEnvelope
}

func (f *fakeEnvelopePublisher) PublishEnvelope(_ context.Context, envelope *envelopev1.TelemetryEnvelope) error {
	f.publishCount.Add(1)
	f.mu.Lock()
	f.envelopes = append(f.envelopes, envelope)
	f.mu.Unlock()
	if f.onPublish == nil {
		return nil
	}
	return f.onPublish(envelope)
}

func (f *fakeEnvelopePublisher) Close() error { return nil }

func (f *fakeEnvelopePublisher) snapshot() []*envelopev1.TelemetryEnvelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*envelopev1.TelemetryEnvelope, len(f.envelopes))
	copy(out, f.envelopes)
	return out
}

type fakeProviderDeviceUpdater struct {
	mu     sync.Mutex
	calls  atomic.Int64
	inputs []controlplane.UpsertProviderDeviceInput
	err    error
}

func (f *fakeProviderDeviceUpdater) UpsertProviderDevice(_ context.Context, in controlplane.UpsertProviderDeviceInput) (controlplane.ProviderDevice, error) {
	f.calls.Add(1)
	f.mu.Lock()
	f.inputs = append(f.inputs, in)
	f.mu.Unlock()
	if f.err != nil {
		return controlplane.ProviderDevice{}, f.err
	}
	return controlplane.ProviderDevice{
		DeviceID:         in.DeviceID,
		Provider:         in.Provider,
		ProviderDeviceID: in.ProviderDeviceID,
		CredentialID:     in.CredentialID,
		Capabilities:     in.Capabilities,
		Metadata:         in.Metadata,
	}, nil
}

func (f *fakeProviderDeviceUpdater) snapshot() []controlplane.UpsertProviderDeviceInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]controlplane.UpsertProviderDeviceInput, len(f.inputs))
	copy(out, f.inputs)
	return out
}
