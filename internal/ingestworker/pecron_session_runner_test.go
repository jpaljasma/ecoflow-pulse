package ingestworker

import (
	"context"
	"encoding/json"
	"errors"
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
}

func (f fakePecronSnapshotter) GetDeviceTelemetrySnapshot(context.Context, controlplane.ProviderCredential, string) (controlplane.ProviderDevice, pecron.NormalizedTelemetry, error) {
	return f.device, f.snapshot, nil
}

func (f fakePecronSnapshotter) MQTTSession(context.Context, controlplane.ProviderCredential, string) (pecron.MQTTSession, error) {
	return pecron.MQTTSession{
		Address: "iot-south.landecia.com:8443",
		Path:    "/ws/v2",
		Token:   "token",
		Topics:  []string{"q/2/d/qdp11vxgaabbccddeeff/bus_"},
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
