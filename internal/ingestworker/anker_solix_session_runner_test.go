package ingestworker

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ankersolix"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

type fakeAnkerSolixResolver struct {
	session provideradapter.AnkerSolixMQTTSession
}

func (f fakeAnkerSolixResolver) MQTTSession(context.Context, controlplane.ProviderCredential, string) (provideradapter.AnkerSolixMQTTSession, error) {
	return f.session, nil
}

func TestAnkerSolixSessionRunnerPublishesDecodedMQTTIntoSharedEnvelopePipeline(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	publisher := &fakeEnvelopePublisher{
		onPublish: func(*envelopev1.TelemetryEnvelope) error {
			cancel()
			return nil
		},
	}
	runner, err := NewAnkerSolixSessionRunner(
		testLogger(),
		fakeAnkerSolixResolver{session: provideradapter.AnkerSolixMQTTSession{
			Address:      "aiot-mqtt.anker.example:8883",
			ClientID:     "thing-1",
			Topics:       []string{"dt/anker_power/A1783/SN-C2000/#"},
			PublishTopic: "cmd/anker_power/A1783/SN-C2000/req",
			DeviceRef:    ankersolix.DeviceRef{ProductCode: "A1783", DeviceSN: "SN-C2000"},
		}},
		publisher,
		AnkerSolixSessionConfig{
			PublishQueueSize:      4,
			PublishWorkers:        1,
			PublishEnqueueTimeout: time.Second,
			ReconnectJitter:       0,
		},
	)
	if err != nil {
		t.Fatalf("NewAnkerSolixSessionRunner() error = %v", err)
	}
	subscriber := &fakeAnkerSolixSubscriber{
		reads: []fakeReadResult{{
			err: context.DeadlineExceeded,
		}, {
			msg: ecoflowmqtt.Message{
				Topic:   "dt/anker_power/A1783/SN-C2000/param_info",
				Payload: []byte("decoded by injected decoder"),
			},
		}},
	}
	factory := &fakeAnkerSolixSubscriberFactory{subscribers: []ankerSolixSessionSubscriber{subscriber}}
	runner.newSubscriber = factory.new
	runner.publishTrigger = func(context.Context, ankerSolixSessionSubscriber, provideradapter.AnkerSolixMQTTSession, time.Duration, time.Time) error {
		subscriber.triggerCalls.Add(1)
		return nil
	}
	runner.decodeMQTT = func(string, []byte) (provideradapter.AnkerSolixDecodedMessage, error) {
		return provideradapter.AnkerSolixDecodedMessage{
			Ref:    ankersolix.DeviceRef{ProductCode: "A1783", DeviceSN: "SN-C2000"},
			Values: map[string]any{"battery_soc": float64(77), "pv_1_power": float64(123)},
		}, nil
	}
	runner.mergeValues = func(base map[string]any, next map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range base {
			out[k] = v
		}
		for k, v := range next {
			out[k] = v
		}
		return out
	}
	runner.normalizeTelemetry = func(ankersolix.DeviceRef, map[string]any) ankersolix.NormalizedTelemetry {
		return ankersolix.NormalizedTelemetry{
			Params: map[string]any{
				"soc":            float64(77),
				"pv1ChargeWatts": float64(123),
			},
			ObservedAt: time.Unix(1_700_000_000, 0).UTC(),
		}
	}

	err = runner.Run(ctx, controlplane.IngestAssignment{
		Provider:           controlplane.ProviderAnkerSolix,
		ProviderDeviceID:   "A1783:SN-C2000",
		DeviceID:           "device-1",
		CredentialID:       "cred-1",
		ProductName:        "Garage C2000",
		Model:              "A1783",
		AccessKey:          "owner@example.test",
		SecretKey:          "password",
		CredentialConfig:   map[string]any{"server": "com", "country": "US"},
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
	if env.GetLabels()["provider"] != controlplane.ProviderAnkerSolix {
		t.Fatalf("provider label = %#v", env.GetLabels())
	}
	var payload struct {
		TypeCode string         `json:"typeCode"`
		Params   map[string]any `json:"params"`
	}
	if err := json.Unmarshal(env.GetPayload(), &payload); err != nil {
		t.Fatalf("unmarshal envelope payload: %v", err)
	}
	if got := payload.Params["soc"]; got != float64(77) {
		t.Fatalf("payload soc = %#v", got)
	}
	if got := payload.Params["pv1ChargeWatts"]; got != float64(123) {
		t.Fatalf("payload pv watts = %#v", got)
	}
	if subscriber.triggerCalls.Load() == 0 {
		t.Fatalf("expected realtime trigger on connect")
	}
	if subscriber.closeCalls.Load() == 0 {
		t.Fatalf("expected subscriber close")
	}
}

func TestProviderSessionRunnerSupportsAnkerSolixRegistration(t *testing.T) {
	t.Parallel()

	runner := NewProviderSessionRunner()
	wantErr := errors.New("anker solix runner reached")
	runner.Register(controlplane.ProviderAnkerSolix, stubSessionRunner{
		run: func(context.Context, controlplane.IngestAssignment) error {
			return wantErr
		},
	})
	err := runner.Run(context.Background(), controlplane.IngestAssignment{Provider: " ANKER_SOLIX "})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestDefaultAnkerSolixSessionConfigUsesBalancedRealtimeRefresh(t *testing.T) {
	t.Parallel()

	cfg := DefaultAnkerSolixSessionConfig()
	if cfg.RealtimeTriggerTimeout != 300*time.Second {
		t.Fatalf("trigger timeout = %s, want 300s", cfg.RealtimeTriggerTimeout)
	}
	if cfg.RealtimeTriggerRefreshInterval != 270*time.Second {
		t.Fatalf("trigger refresh = %s, want 270s", cfg.RealtimeTriggerRefreshInterval)
	}
	if cfg.RealtimeTriggerRefreshInterval >= cfg.RealtimeTriggerTimeout {
		t.Fatalf("refresh interval should be before trigger timeout: %+v", cfg)
	}
}

func TestAnkerSolixLogDeviceRefRedactsProviderDeviceID(t *testing.T) {
	t.Parallel()

	if got := providerDeviceLogRef(controlplane.ProviderAnkerSolix, "A1783:SN-C2000"); got != "A1783:redacted" {
		t.Fatalf("log device ref = %q", got)
	}
	if got := providerDeviceLogRef(controlplane.ProviderAnkerSolix, "not-a-provider-id"); got != "redacted" {
		t.Fatalf("invalid log device ref = %q", got)
	}
}

func TestAnkerSolixTriggerRefreshLoopStopsOnCancellation(t *testing.T) {
	t.Parallel()

	runner, err := NewAnkerSolixSessionRunner(
		testLogger(),
		fakeAnkerSolixResolver{},
		&fakeEnvelopePublisher{},
		AnkerSolixSessionConfig{
			RealtimeTriggerRefreshInterval: time.Millisecond,
			RealtimeTriggerTimeout:         300 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewAnkerSolixSessionRunner() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int64
	runner.sleepFn = func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}
	runner.publishTrigger = func(context.Context, ankerSolixSessionSubscriber, provideradapter.AnkerSolixMQTTSession, time.Duration, time.Time) error {
		calls.Add(1)
		return nil
	}
	runner.runRealtimeTriggerRefreshLoop(ctx, &fakeAnkerSolixSubscriber{}, provideradapter.AnkerSolixMQTTSession{}, runner.cfg)
	if calls.Load() != 0 {
		t.Fatalf("refresh loop published after cancellation")
	}
}

type fakeAnkerSolixSubscriber struct {
	connectErr   error
	subscribeErr error
	publishErr   error

	mu    sync.Mutex
	reads []fakeReadResult

	triggerCalls atomic.Int64
	closeCalls   atomic.Int64
}

func (f *fakeAnkerSolixSubscriber) Connect(context.Context) error {
	return f.connectErr
}

func (f *fakeAnkerSolixSubscriber) Subscribe(context.Context, string, byte) error {
	return f.subscribeErr
}

func (f *fakeAnkerSolixSubscriber) Publish(context.Context, string, []byte, byte) error {
	f.triggerCalls.Add(1)
	return f.publishErr
}

func (f *fakeAnkerSolixSubscriber) ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error) {
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

func (f *fakeAnkerSolixSubscriber) Close() error {
	f.closeCalls.Add(1)
	return nil
}

type fakeAnkerSolixSubscriberFactory struct {
	mu          sync.Mutex
	subscribers []ankerSolixSessionSubscriber
	configs     []ankersolix.MQTTConfig
}

func (f *fakeAnkerSolixSubscriberFactory) new(cfg ankersolix.MQTTConfig) (ankerSolixSessionSubscriber, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs = append(f.configs, cfg)
	if len(f.subscribers) == 0 {
		return nil, errors.New("no fake anker solix subscribers")
	}
	next := f.subscribers[0]
	f.subscribers = f.subscribers[1:]
	return next, nil
}
