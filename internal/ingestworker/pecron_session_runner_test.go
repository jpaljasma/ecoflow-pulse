package ingestworker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
)

type fakePecronSnapshotter struct {
	device   controlplane.ProviderDevice
	snapshot pecron.NormalizedTelemetry
	session  pecron.MQTTSession
}

func (f fakePecronSnapshotter) GetDeviceTelemetrySnapshot(context.Context, controlplane.ProviderCredential, string) (controlplane.ProviderDevice, pecron.NormalizedTelemetry, error) {
	return f.device, f.snapshot, nil
}

func (f fakePecronSnapshotter) MQTTSession(context.Context, controlplane.ProviderCredential, string) (pecron.MQTTSession, error) {
	if f.session.ClientID != "" || f.session.Address != "" || len(f.session.Addresses) > 0 {
		return f.session, nil
	}
	return pecron.MQTTSession{
		Address:  "iot-south.landecia.com:8443",
		Path:     "/ws/v2",
		Token:    "token",
		ClientID: "qu_user_1779327000000",
		Topics:   []string{"q/2/d/qdp11vxgaabbccddeeff/bus_"},
	}, nil
}

func TestPecronSessionRunnerPublishesDecodedMQTTIntoSharedEnvelopePipeline(t *testing.T) {
	t.Parallel()

	publisher := &fakeEnvelopePublisher{}
	runner, err := NewPecronSessionRunner(
		testLogger(),
		fakePecronSnapshotter{},
		publisher,
		&fakeProviderDeviceUpdater{},
		PecronSessionConfig{
			PublishQueueSize:         4,
			PublishWorkers:           1,
			PublishEnqueueTimeout:    time.Second,
			DisableSnapshotBootstrap: true,
		},
	)
	if err != nil {
		t.Fatalf("NewPecronSessionRunner() error = %v", err)
	}
	subscriber := &fakeMQTTSubscriber{
		reads: []fakeReadResult{
			{msg: ecoflowmqtt.Message{
				Topic:   "q/2/d/qdp11vxgaabbccddeeff/bus_",
				Payload: []byte(`{"deviceKey":"aabbccddeeff","data":{"kv":{"battery_percentage":66,"total_input_power":144,"dc_data_input_hm":{"gx16mf1_input_power":144}}}}`),
			}},
			{err: context.Canceled},
		},
	}
	factory := &fakePecronSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}
	runner.newSubscriber = factory.new

	err = runner.Run(context.Background(), controlplane.IngestAssignment{
		Provider:           controlplane.ProviderPecron,
		ProviderDeviceID:   "p11vxg:aabbccddeeff",
		DeviceID:           "device-1",
		CredentialID:       "cred-1",
		ProductName:        "Garage Pecron",
		Model:              "E1000LFP",
		AccessKey:          "owner@example.test",
		SecretKey:          "password",
		CredentialConfig:   map[string]any{"region": "us"},
		DeviceIsActive:     true,
		CredentialIsActive: true,
		IngestDesiredState: "active",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	published := publisher.snapshot()
	if len(published) != 1 {
		t.Fatalf("published envelopes = %d, want 1", len(published))
	}
	env := published[0]
	if env.GetPayloadType() != providerNormalizedPayloadType {
		t.Fatalf("payload type = %q, want %q", env.GetPayloadType(), providerNormalizedPayloadType)
	}
	if env.GetSourceKind() != envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA {
		t.Fatalf("source kind = %s", env.GetSourceKind())
	}
	if env.GetLabels()["provider"] != controlplane.ProviderPecron {
		t.Fatalf("provider label = %#v", env.GetLabels())
	}
	var payload struct {
		TypeCode string         `json:"typeCode"`
		Params   map[string]any `json:"params"`
	}
	if err := json.Unmarshal(env.GetPayload(), &payload); err != nil {
		t.Fatalf("unmarshal envelope payload: %v", err)
	}
	if got := payload.Params["soc"]; got != float64(66) {
		t.Fatalf("payload soc = %#v", got)
	}
	if got := payload.Params["pv1ChargeWatts"]; got != float64(144) {
		t.Fatalf("payload pv watts = %#v", got)
	}
}

func TestProviderSessionRunnerSupportsPecronRegistration(t *testing.T) {
	t.Parallel()

	runner := NewProviderSessionRunner()
	wantErr := errors.New("pecron runner reached")
	runner.Register(controlplane.ProviderPecron, stubSessionRunner{
		run: func(context.Context, controlplane.IngestAssignment) error {
			return wantErr
		},
	})
	err := runner.Run(context.Background(), controlplane.IngestAssignment{Provider: " PECRON "})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestDefaultPecronSessionConfigHonorsCloudRateLimitFloor(t *testing.T) {
	t.Parallel()

	cfg := DefaultPecronSessionConfig()
	if cfg.SnapshotRefreshInterval != pecron.RecommendedCloudRESTPollInterval {
		t.Fatalf("snapshot refresh interval = %s, want %s", cfg.SnapshotRefreshInterval, pecron.RecommendedCloudRESTPollInterval)
	}
	if cfg.SnapshotRefreshInterval < pecron.MinCloudRESTPollInterval {
		t.Fatalf("snapshot refresh interval %s is below Pecron floor %s", cfg.SnapshotRefreshInterval, pecron.MinCloudRESTPollInterval)
	}
}

func TestPecronSessionConfigRejectsSnapshotRefreshBelowCloudFloor(t *testing.T) {
	t.Parallel()

	_, err := NewPecronSessionRunner(
		testLogger(),
		fakePecronSnapshotter{},
		&fakeEnvelopePublisher{},
		&fakeProviderDeviceUpdater{},
		PecronSessionConfig{
			PublishQueueSize:         4,
			PublishWorkers:           1,
			PublishEnqueueTimeout:    time.Second,
			SnapshotRefreshInterval:  pecron.MinCloudRESTPollInterval - time.Second,
			DisableSnapshotBootstrap: true,
		},
	)
	if err == nil {
		t.Fatal("expected below-floor Pecron snapshot refresh interval to fail")
	}
	if !strings.Contains(err.Error(), "pecron cloud REST snapshot refresh interval") {
		t.Fatalf("error = %v", err)
	}
}

func TestPecronSessionRunnerConnectsUsingBrokerFallback(t *testing.T) {
	t.Parallel()

	runner, err := NewPecronSessionRunner(
		testLogger(),
		fakePecronSnapshotter{},
		&fakeEnvelopePublisher{},
		&fakeProviderDeviceUpdater{},
		PecronSessionConfig{
			PublishQueueSize:         4,
			PublishWorkers:           1,
			PublishEnqueueTimeout:    time.Second,
			DisableSnapshotBootstrap: true,
		},
	)
	if err != nil {
		t.Fatalf("NewPecronSessionRunner() error = %v", err)
	}
	first := &fakeMQTTSubscriber{connectErr: errors.New("primary unavailable")}
	second := &fakeMQTTSubscriber{}
	factory := &fakePecronSubscriberFactory{subscribers: []mqttSubscriber{first, second}}
	runner.newSubscriber = factory.new

	subscriber, address, err := runner.connectSubscriber(context.Background(), pecron.MQTTSession{
		Address:   "primary.example:8443",
		Addresses: []string{"primary.example:8443", "fallback.example:8443"},
		Path:      "/ws/v2",
		Token:     "token",
		Topics:    []string{"q/2/d/qdp11vxgaabbccddeeff/bus_"},
	}, "client-id", runner.cfg)
	if err != nil {
		t.Fatalf("connectSubscriber() error = %v", err)
	}
	defer func() { _ = subscriber.Close() }()
	if address != "fallback.example:8443" {
		t.Fatalf("connected address = %q, want fallback", address)
	}
	if got := first.closeCalls.Load(); got != 1 {
		t.Fatalf("primary close calls = %d, want 1", got)
	}
	if len(factory.configs) != 2 {
		t.Fatalf("subscriber configs = %d, want 2", len(factory.configs))
	}
	if factory.configs[0].Address != "primary.example:8443" || factory.configs[1].Address != "fallback.example:8443" {
		t.Fatalf("subscriber addresses = %#v", factory.configs)
	}
}

func TestPecronSessionRunnerRequestsTelemetryAfterSubscribe(t *testing.T) {
	t.Parallel()

	runner, err := NewPecronSessionRunner(
		testLogger(),
		fakePecronSnapshotter{},
		&fakeEnvelopePublisher{},
		&fakeProviderDeviceUpdater{},
		PecronSessionConfig{
			PublishQueueSize:         4,
			PublishWorkers:           1,
			PublishEnqueueTimeout:    time.Second,
			DisableSnapshotBootstrap: true,
		},
	)
	if err != nil {
		t.Fatalf("NewPecronSessionRunner() error = %v", err)
	}
	subscriber := &fakeMQTTSubscriber{}
	factory := &fakePecronSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}
	runner.newSubscriber = factory.new
	session := pecron.MQTTSession{
		Address: "iot-south.landecia.com:8443",
		Path:    "/ws/v2",
		Token:   "token",
		Topics: []string{
			"q/2/d/qdp11vxgAABBCCDDEEFF/bus_",
			"q/2/d/qdp11vxgAABBCCDDEEFF/ack_",
			"q/2/d/qdp11vxgAABBCCDDEEFF/onl_",
		},
		Ref: pecron.DeviceRef{ProductKey: pecron.ProductKeyE1000LFP, DeviceKey: "AABBCCDDEEFF"},
	}

	_, _, err = runner.connectSubscriber(context.Background(), session, "client-id", runner.cfg)
	if err != nil {
		t.Fatalf("connectSubscriber() error = %v", err)
	}
	if subscriber.subscribeMultipleCalls != 1 {
		t.Fatalf("subscribe multiple calls = %d, want 1", subscriber.subscribeMultipleCalls)
	}
	if len(subscriber.subscribedTopics) != len(session.Topics) {
		t.Fatalf("subscribed topics = %#v, want %#v", subscriber.subscribedTopics, session.Topics)
	}
	if len(subscriber.published) != 1 {
		t.Fatalf("published requests = %d, want 1", len(subscriber.published))
	}
	if got, want := subscriber.published[0].topic, "q/1/d/qdp11vxgAABBCCDDEEFF/bus"; got != want {
		t.Fatalf("publish topic = %q, want %q", got, want)
	}
	if got, want := subscriber.published[0].payload, pecron.TTLVReadPacket(1); string(got) != string(want) {
		t.Fatalf("publish payload = %x, want %x", got, want)
	}
	if got, want := subscriber.published[0].qos, byte(1); got != want {
		t.Fatalf("publish qos = %d, want %d", got, want)
	}
}

func TestPecronSessionRunnerUsesProviderIssuedClientIDWithoutNamespacing(t *testing.T) {
	t.Parallel()

	issuedClientID := "qu_user_1779327000000"
	runner, err := NewPecronSessionRunner(
		testLogger(),
		fakePecronSnapshotter{session: pecron.MQTTSession{
			Address:  "iot-south.landecia.com:8443",
			Path:     "/ws/v2",
			Token:    "token",
			ClientID: issuedClientID,
			Topics:   []string{"q/2/d/qdp11vxgaabbccddeeff/bus_"},
			Ref:      pecron.DeviceRef{ProductKey: pecron.ProductKeyE1000LFP, DeviceKey: "aabbccddeeff"},
		}},
		&fakeEnvelopePublisher{},
		&fakeProviderDeviceUpdater{},
		PecronSessionConfig{
			MQTTClientIDNamespace:    "cloud",
			PublishQueueSize:         4,
			PublishWorkers:           1,
			PublishEnqueueTimeout:    time.Second,
			DisableSnapshotBootstrap: true,
		},
	)
	if err != nil {
		t.Fatalf("NewPecronSessionRunner() error = %v", err)
	}
	subscriber := &fakeMQTTSubscriber{
		reads: []fakeReadResult{{err: context.Canceled}},
	}
	factory := &fakePecronSubscriberFactory{subscribers: []mqttSubscriber{subscriber}}
	runner.newSubscriber = factory.new

	err = runner.Run(context.Background(), controlplane.IngestAssignment{
		Provider:           controlplane.ProviderPecron,
		ProviderDeviceID:   "p11vxg:aabbccddeeff",
		DeviceID:           "device-1",
		CredentialID:       "cred-1",
		ProductName:        "Garage Pecron",
		Model:              "E1000LFP",
		AccessKey:          "owner@example.test",
		SecretKey:          "password",
		CredentialConfig:   map[string]any{"region": "us"},
		DeviceIsActive:     true,
		CredentialIsActive: true,
		IngestDesiredState: "active",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(factory.configs) != 1 {
		t.Fatalf("subscriber configs = %d, want 1", len(factory.configs))
	}
	if got := factory.configs[0].ClientID; got != issuedClientID {
		t.Fatalf("client id = %q, want provider-issued %q", got, issuedClientID)
	}
	if got := factory.configs[0].ClientID; got == ecoflowmqtt.BuildClientIDWithNamespace("cloud", issuedClientID) {
		t.Fatalf("pecron client id was namespaced like an EcoFlow client id: %q", got)
	}
}

type fakePecronSubscriberFactory struct {
	subscribers []mqttSubscriber
	configs     []pecron.MQTTConfig
}

func (f *fakePecronSubscriberFactory) new(cfg pecron.MQTTConfig) (mqttSubscriber, error) {
	f.configs = append(f.configs, cfg)
	if len(f.subscribers) == 0 {
		return nil, errors.New("no fake pecron subscribers")
	}
	next := f.subscribers[0]
	f.subscribers = f.subscribers[1:]
	return next, nil
}
