package ingestworker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ankersolix"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

type ankerSolixTelemetryResolver interface {
	MQTTSession(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (provideradapter.AnkerSolixMQTTSession, error)
}

type ankerSolixSessionSubscriber interface {
	Connect(ctx context.Context) error
	Subscribe(ctx context.Context, topic string, qos byte) error
	Publish(ctx context.Context, topic string, payload []byte, qos byte) error
	ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error)
	Close() error
}

type ankerSolixSubscriberFactory func(cfg ankersolix.MQTTConfig) (ankerSolixSessionSubscriber, error)
type ankerSolixTriggerPublisher func(ctx context.Context, subscriber ankerSolixSessionSubscriber, session provideradapter.AnkerSolixMQTTSession, timeout time.Duration, now time.Time) error

type AnkerSolixSessionConfig struct {
	ShardCount uint32

	MQTTClientIDNamespace string

	KeepAlive      time.Duration
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	SubscribeQoS   byte

	PublishQueueSize      int
	PublishWorkers        int
	PublishEnqueueTimeout time.Duration
	AllowUnorderedPublish bool
	DisableEnvelopeLabels bool

	ReconnectInitialBackoff time.Duration
	ReconnectMaxBackoff     time.Duration
	ReconnectJitter         float64

	RealtimeTriggerTimeout         time.Duration
	RealtimeTriggerRefreshInterval time.Duration
	RealtimeTriggerRefreshJitter   float64
}

func DefaultAnkerSolixSessionConfig() AnkerSolixSessionConfig {
	return AnkerSolixSessionConfig{
		ShardCount: telemetrybus.DefaultShardCount,

		KeepAlive:      defaultMQTTKeepAlive,
		ConnectTimeout: defaultMQTTConnectTimeout,
		ReadTimeout:    defaultMQTTReadTimeout,
		SubscribeQoS:   0,

		PublishQueueSize:      defaultPublishQueueSize,
		PublishWorkers:        defaultPublishWorkers,
		PublishEnqueueTimeout: defaultPublishEnqueueTimeout,

		ReconnectInitialBackoff: defaultMQTTReconnectInitialDelay,
		ReconnectMaxBackoff:     defaultMQTTReconnectMaxDelay,
		ReconnectJitter:         defaultMQTTReconnectJitter,

		RealtimeTriggerTimeout:         300 * time.Second,
		RealtimeTriggerRefreshInterval: 270 * time.Second,
		RealtimeTriggerRefreshJitter:   0.10,
	}
}

func (c AnkerSolixSessionConfig) normalized() AnkerSolixSessionConfig {
	cfg := c
	defaults := DefaultAnkerSolixSessionConfig()
	if cfg.ShardCount == 0 {
		cfg.ShardCount = defaults.ShardCount
	}
	cfg.MQTTClientIDNamespace = strings.TrimSpace(cfg.MQTTClientIDNamespace)
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = defaults.KeepAlive
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = defaults.ConnectTimeout
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = defaults.ReadTimeout
	}
	if cfg.SubscribeQoS > 1 {
		cfg.SubscribeQoS = defaults.SubscribeQoS
	}
	if cfg.PublishQueueSize <= 0 {
		cfg.PublishQueueSize = defaults.PublishQueueSize
	}
	if cfg.PublishWorkers <= 0 {
		cfg.PublishWorkers = defaults.PublishWorkers
	}
	if cfg.PublishEnqueueTimeout <= 0 {
		cfg.PublishEnqueueTimeout = defaults.PublishEnqueueTimeout
	}
	if cfg.ReconnectInitialBackoff <= 0 {
		cfg.ReconnectInitialBackoff = defaults.ReconnectInitialBackoff
	}
	if cfg.ReconnectMaxBackoff < cfg.ReconnectInitialBackoff {
		cfg.ReconnectMaxBackoff = defaults.ReconnectMaxBackoff
		if cfg.ReconnectMaxBackoff < cfg.ReconnectInitialBackoff {
			cfg.ReconnectMaxBackoff = cfg.ReconnectInitialBackoff
		}
	}
	if cfg.ReconnectJitter < 0 {
		cfg.ReconnectJitter = 0
	}
	if cfg.RealtimeTriggerTimeout <= 0 {
		cfg.RealtimeTriggerTimeout = defaults.RealtimeTriggerTimeout
	}
	if cfg.RealtimeTriggerRefreshInterval <= 0 {
		cfg.RealtimeTriggerRefreshInterval = defaults.RealtimeTriggerRefreshInterval
	}
	if cfg.RealtimeTriggerRefreshInterval >= cfg.RealtimeTriggerTimeout {
		cfg.RealtimeTriggerRefreshInterval = cfg.RealtimeTriggerTimeout - 30*time.Second
		if cfg.RealtimeTriggerRefreshInterval <= 0 {
			cfg.RealtimeTriggerRefreshInterval = cfg.RealtimeTriggerTimeout / 2
		}
	}
	if cfg.RealtimeTriggerRefreshJitter < 0 {
		cfg.RealtimeTriggerRefreshJitter = 0
	}
	return cfg
}

func (c AnkerSolixSessionConfig) validate() error {
	if c.PublishWorkers > 1 && !c.AllowUnorderedPublish {
		return errors.New("publish_workers > 1 requires allow_unordered_publish=true")
	}
	if c.RealtimeTriggerRefreshInterval >= c.RealtimeTriggerTimeout {
		return errors.New("anker solix realtime trigger refresh interval must be lower than trigger timeout")
	}
	return nil
}

type AnkerSolixSessionRunner struct {
	log       *slog.Logger
	adapter   ankerSolixTelemetryResolver
	publisher telemetrybus.EnvelopePublisher
	cfg       AnkerSolixSessionConfig

	newSubscriber      ankerSolixSubscriberFactory
	decodeMQTT         provideradapter.AnkerSolixMessageDecoder
	mergeValues        func(base map[string]any, next map[string]any) map[string]any
	normalizeTelemetry func(ref ankersolix.DeviceRef, values map[string]any) ankersolix.NormalizedTelemetry
	publishTrigger     ankerSolixTriggerPublisher
	sleepFn            sessionSleepFunc
	nowFn              func() time.Time
}

func NewAnkerSolixSessionRunner(
	log *slog.Logger,
	adapter ankerSolixTelemetryResolver,
	publisher telemetrybus.EnvelopePublisher,
	cfg AnkerSolixSessionConfig,
) (*AnkerSolixSessionRunner, error) {
	if log == nil {
		log = slog.Default()
	}
	if adapter == nil {
		adapter = provideradapter.NewAnkerSolixAdapter(provideradapter.AnkerSolixAdapterConfig{})
	}
	if publisher == nil {
		return nil, errors.New("telemetry envelope publisher is required")
	}
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid anker solix session config: %w", err)
	}
	return &AnkerSolixSessionRunner{
		log:       log,
		adapter:   adapter,
		publisher: publisher,
		cfg:       cfg,
		newSubscriber: func(cfg ankersolix.MQTTConfig) (ankerSolixSessionSubscriber, error) {
			return ankersolix.NewMQTTSubscriber(cfg)
		},
		decodeMQTT: provideradapter.DecodeAnkerSolixMQTTMessage,
		mergeValues: func(base map[string]any, next map[string]any) map[string]any {
			return ankersolix.MergeValues(base, next)
		},
		normalizeTelemetry: func(ref ankersolix.DeviceRef, values map[string]any) ankersolix.NormalizedTelemetry {
			return ankersolix.NormalizeTelemetry(ref, values)
		},
		publishTrigger: publishAnkerSolixSessionTrigger,
		sleepFn:        sessionSleepContext,
		nowFn:          time.Now,
	}, nil
}

func (r *AnkerSolixSessionRunner) Run(ctx context.Context, a controlplane.IngestAssignment) error {
	if sanitizeProvider(a.Provider) != controlplane.ProviderAnkerSolix {
		return fmt.Errorf("unsupported provider in session runner: %s", a.Provider)
	}
	cfg := r.cfg.normalized()
	backoff := cfg.ReconnectInitialBackoff
	logDeviceRef := providerDeviceLogRef(a.Provider, a.ProviderDeviceID)
	for {
		if ctx.Err() != nil {
			return nil
		}
		connected, err := r.runSessionOnce(ctx, a, cfg)
		if err == nil || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return nil
		}
		retryIn := applySessionJitter(backoff, cfg.ReconnectJitter)
		r.log.Warn("anker solix ingest session error; reconnecting",
			slog.String("provider", a.Provider),
			slog.String("provider_device_ref", logDeviceRef),
			slog.String("error", err.Error()),
			slog.Duration("retry_in", retryIn),
		)
		if sleepErr := r.sleepFn(ctx, retryIn); sleepErr != nil {
			return nil
		}
		if connected {
			backoff = cfg.ReconnectInitialBackoff
		} else {
			backoff = nextBackoff(backoff, cfg.ReconnectMaxBackoff)
		}
	}
}

func (r *AnkerSolixSessionRunner) runSessionOnce(
	ctx context.Context,
	a controlplane.IngestAssignment,
	cfg AnkerSolixSessionConfig,
) (connected bool, err error) {
	logDeviceRef := providerDeviceLogRef(a.Provider, a.ProviderDeviceID)
	credential := credentialFromAssignment(a)
	session, err := r.adapter.MQTTSession(ctx, credential, a.ProviderDeviceID)
	if err != nil {
		return false, fmt.Errorf("resolve anker solix mqtt session: %w", err)
	}
	clientID := strings.TrimSpace(session.ClientID)
	if cfg.MQTTClientIDNamespace != "" {
		clientID = ecoflowmqtt.BuildClientIDWithNamespace(cfg.MQTTClientIDNamespace, clientID)
	}
	subscriber, connectedAddress, err := r.connectSubscriber(ctx, session, clientID, cfg)
	if err != nil {
		return false, err
	}
	defer func() { _ = subscriber.Close() }()
	if err := r.publishTrigger(ctx, subscriber, session, cfg.RealtimeTriggerTimeout, r.nowFn().UTC()); err != nil {
		return true, fmt.Errorf("publish anker solix realtime trigger: %w", err)
	}
	r.log.Info("anker solix ingest session connected",
		slog.String("provider", a.Provider),
		slog.String("provider_device_ref", logDeviceRef),
		slog.String("broker", connectedAddress),
	)

	asyncPublisher := newAsyncEnvelopePublisher(ctx, r.publisher, cfg.PublishQueueSize, cfg.PublishWorkers, cfg.PublishEnqueueTimeout)
	envelopeBuilder := newTelemetryEnvelopeBuilder(a, EcoFlowSessionConfig{
		ShardCount:            cfg.ShardCount,
		DisableEnvelopeLabels: cfg.DisableEnvelopeLabels,
	})
	refreshCtx, refreshCancel := context.WithCancel(ctx)
	refreshDone := make(chan struct{})
	defer func() {
		refreshCancel()
		<-refreshDone
		_ = asyncPublisher.Close()
	}()
	go func() {
		defer close(refreshDone)
		r.runRealtimeTriggerRefreshLoop(refreshCtx, subscriber, session, cfg)
	}()

	state := map[string]any{}
	ref := session.DeviceRef
	for {
		select {
		case publishErr := <-asyncPublisher.Errors():
			if publishErr != nil {
				r.log.Warn("anker solix ingest publish failed; dropping envelope and keeping mqtt session alive",
					slog.String("provider", a.Provider),
					slog.String("provider_device_ref", logDeviceRef),
					slog.String("error", publishErr.Error()),
				)
			}
		default:
		}

		msg, readErr := subscriber.ReadMessage(ctx)
		if readErr != nil {
			if errors.Is(readErr, context.Canceled) || ctx.Err() != nil {
				return true, nil
			}
			if errors.Is(readErr, context.DeadlineExceeded) {
				continue
			}
			return true, fmt.Errorf("read anker solix mqtt message: %w", readErr)
		}
		decoded, err := r.decodeMQTT(strings.TrimSpace(msg.Topic), msg.Payload)
		if err != nil {
			continue
		}
		if decoded.Ref.ProductCode != "" || decoded.Ref.DeviceSN != "" {
			if !sameAnkerSolixRef(decoded.Ref, ref) {
				continue
			}
		}
		if len(decoded.Values) == 0 {
			continue
		}
		state = r.mergeValues(state, decoded.Values)
		normalized := r.normalizeTelemetry(ref, state)
		if len(normalized.Params) == 0 {
			continue
		}
		observedAt := normalized.ObservedAt
		if observedAt.IsZero() {
			observedAt = r.nowFn().UTC()
		}
		envelope, err := envelopeBuilder.BuildProviderNormalizedParams(normalized.Params, observedAt.UTC())
		if err != nil {
			return true, fmt.Errorf("build anker solix normalized envelope: %w", err)
		}
		if publishErr := asyncPublisher.Publish(ctx, envelope); publishErr != nil {
			if errors.Is(publishErr, context.Canceled) || ctx.Err() != nil {
				return true, nil
			}
			r.log.Warn("anker solix ingest publish enqueue failed; dropping envelope and keeping mqtt session alive",
				slog.String("provider", a.Provider),
				slog.String("provider_device_ref", logDeviceRef),
				slog.String("error", publishErr.Error()),
			)
		}
	}
}

func (r *AnkerSolixSessionRunner) connectSubscriber(
	ctx context.Context,
	session provideradapter.AnkerSolixMQTTSession,
	clientID string,
	cfg AnkerSolixSessionConfig,
) (ankerSolixSessionSubscriber, string, error) {
	addresses := session.BrokerAddresses()
	if len(addresses) == 0 {
		return nil, "", errors.New("anker solix mqtt session has no broker addresses")
	}
	var lastErr error
	for _, address := range addresses {
		subscriber, err := r.newSubscriber(ankersolix.MQTTConfig{
			Address:        address,
			ClientID:       clientID,
			KeepAlive:      cfg.KeepAlive,
			ConnectTimeout: cfg.ConnectTimeout,
			ReadTimeout:    cfg.ReadTimeout,
			TLSConfig:      session.TLSConfig,
			BufferSize:     cfg.PublishQueueSize,
		})
		if err != nil {
			lastErr = fmt.Errorf("init anker solix mqtt subscriber for %s: %w", address, err)
			continue
		}
		if err := subscriber.Connect(ctx); err != nil {
			_ = subscriber.Close()
			lastErr = fmt.Errorf("connect anker solix mqtt subscriber %s: %w", address, err)
			continue
		}
		if err := subscribeAnkerSolixSessionTopics(ctx, subscriber, session.Topics, cfg.SubscribeQoS); err != nil {
			_ = subscriber.Close()
			lastErr = fmt.Errorf("subscribe anker solix mqtt topics on %s: %w", address, err)
			continue
		}
		return subscriber, address, nil
	}
	if lastErr == nil {
		lastErr = errors.New("anker solix mqtt subscriber could not connect")
	}
	return nil, "", lastErr
}

func subscribeAnkerSolixSessionTopics(ctx context.Context, subscriber ankerSolixSessionSubscriber, topics []string, qos byte) error {
	if len(topics) == 0 {
		return errors.New("anker solix mqtt session has no subscribe topics")
	}
	for _, topic := range topics {
		if err := subscriber.Subscribe(ctx, topic, qos); err != nil {
			return fmt.Errorf("subscribe anker solix mqtt topic %s: %w", mqttTopicLogRef(topic), err)
		}
	}
	return nil
}

func (r *AnkerSolixSessionRunner) runRealtimeTriggerRefreshLoop(
	ctx context.Context,
	subscriber ankerSolixSessionSubscriber,
	session provideradapter.AnkerSolixMQTTSession,
	cfg AnkerSolixSessionConfig,
) {
	for {
		wait := applySessionJitter(cfg.RealtimeTriggerRefreshInterval, cfg.RealtimeTriggerRefreshJitter)
		if err := r.sleepFn(ctx, wait); err != nil {
			return
		}
		if err := r.publishTrigger(ctx, subscriber, session, cfg.RealtimeTriggerTimeout, r.nowFn().UTC()); err != nil {
			r.log.Warn("anker solix realtime trigger refresh failed; keeping mqtt session alive",
				slog.String("provider", controlplane.ProviderAnkerSolix),
				slog.String("provider_device_ref", providerDeviceLogRef(controlplane.ProviderAnkerSolix, session.DeviceRef.ProviderDeviceID())),
				slog.String("error", err.Error()),
			)
		}
	}
}

func publishAnkerSolixSessionTrigger(
	ctx context.Context,
	subscriber ankerSolixSessionSubscriber,
	session provideradapter.AnkerSolixMQTTSession,
	timeout time.Duration,
	now time.Time,
) error {
	topic, payload, qos, err := session.RealtimeTriggerCommand(timeout, now)
	if err != nil {
		return err
	}
	return subscriber.Publish(ctx, topic, payload, qos)
}

func sameAnkerSolixRef(left, right ankersolix.DeviceRef) bool {
	return strings.EqualFold(strings.TrimSpace(left.ProductCode), strings.TrimSpace(right.ProductCode)) &&
		strings.EqualFold(strings.TrimSpace(left.DeviceSN), strings.TrimSpace(right.DeviceSN))
}
