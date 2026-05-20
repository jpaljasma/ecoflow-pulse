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
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
)

type pecronTelemetryResolver interface {
	GetDeviceTelemetrySnapshot(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (controlplane.ProviderDevice, pecron.NormalizedTelemetry, error)
	MQTTSession(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (pecron.MQTTSession, error)
}

type pecronSubscriberFactory func(cfg pecron.MQTTConfig) (mqttSubscriber, error)

type pecronMultiSubscriber interface {
	SubscribeMultiple(ctx context.Context, topics []string, qos byte) error
}

type pecronPublisher interface {
	Publish(ctx context.Context, topic string, payload []byte, qos byte) error
}

type PecronSessionConfig struct {
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

	SnapshotFetchTimeout     time.Duration
	SnapshotRefreshInterval  time.Duration
	SnapshotRefreshJitter    float64
	DisableSnapshotBootstrap bool
}

func DefaultPecronSessionConfig() PecronSessionConfig {
	return PecronSessionConfig{
		ShardCount: telemetrybus.DefaultShardCount,

		KeepAlive:      defaultMQTTKeepAlive,
		ConnectTimeout: defaultMQTTConnectTimeout,
		ReadTimeout:    defaultMQTTReadTimeout,
		SubscribeQoS:   1,

		PublishQueueSize:      defaultPublishQueueSize,
		PublishWorkers:        defaultPublishWorkers,
		PublishEnqueueTimeout: defaultPublishEnqueueTimeout,

		ReconnectInitialBackoff: defaultMQTTReconnectInitialDelay,
		ReconnectMaxBackoff:     defaultMQTTReconnectMaxDelay,
		ReconnectJitter:         defaultMQTTReconnectJitter,

		SnapshotFetchTimeout:    10 * time.Second,
		SnapshotRefreshInterval: pecron.RecommendedCloudRESTPollInterval,
		SnapshotRefreshJitter:   0.20,
	}
}

func (c PecronSessionConfig) normalized() PecronSessionConfig {
	cfg := c
	defaults := DefaultPecronSessionConfig()
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
	if cfg.SubscribeQoS == 0 {
		cfg.SubscribeQoS = defaults.SubscribeQoS
	} else if cfg.SubscribeQoS > 1 {
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
	if cfg.SnapshotFetchTimeout <= 0 {
		cfg.SnapshotFetchTimeout = defaults.SnapshotFetchTimeout
	}
	if cfg.SnapshotRefreshInterval <= 0 {
		cfg.SnapshotRefreshInterval = defaults.SnapshotRefreshInterval
	}
	if cfg.SnapshotRefreshJitter < 0 {
		cfg.SnapshotRefreshJitter = 0
	}
	return cfg
}

func (c PecronSessionConfig) validate() error {
	if c.PublishWorkers > 1 && !c.AllowUnorderedPublish {
		return errors.New("publish_workers > 1 requires allow_unordered_publish=true")
	}
	if c.SnapshotRefreshInterval < pecron.MinCloudRESTPollInterval {
		return fmt.Errorf(
			"pecron cloud REST snapshot refresh interval %s is below the %s floor; use %s or higher to avoid code 4026 rate-limit exhaustion",
			c.SnapshotRefreshInterval,
			pecron.MinCloudRESTPollInterval,
			pecron.RecommendedCloudRESTPollInterval,
		)
	}
	return nil
}

type PecronSessionRunner struct {
	log             *slog.Logger
	adapter         pecronTelemetryResolver
	publisher       telemetrybus.EnvelopePublisher
	providerDevices providerDeviceUpdater
	cfg             PecronSessionConfig

	newSubscriber pecronSubscriberFactory
	sleepFn       sessionSleepFunc
	nowFn         func() time.Time
}

func NewPecronSessionRunner(
	log *slog.Logger,
	adapter pecronTelemetryResolver,
	publisher telemetrybus.EnvelopePublisher,
	providerDevices providerDeviceUpdater,
	cfg PecronSessionConfig,
) (*PecronSessionRunner, error) {
	if log == nil {
		log = slog.Default()
	}
	if adapter == nil {
		adapter = provideradapter.NewPecronAdapter(provideradapter.PecronAdapterConfig{})
	}
	if publisher == nil {
		return nil, errors.New("telemetry envelope publisher is required")
	}
	if providerDevices == nil {
		return nil, errors.New("provider device updater is required")
	}
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid pecron session config: %w", err)
	}
	return &PecronSessionRunner{
		log:             log,
		adapter:         adapter,
		publisher:       publisher,
		providerDevices: providerDevices,
		cfg:             cfg,
		newSubscriber: func(cfg pecron.MQTTConfig) (mqttSubscriber, error) {
			return pecron.NewMQTTSubscriber(cfg)
		},
		sleepFn: sessionSleepContext,
		nowFn:   time.Now,
	}, nil
}

func (r *PecronSessionRunner) Run(ctx context.Context, a controlplane.IngestAssignment) error {
	if sanitizeProvider(a.Provider) != controlplane.ProviderPecron {
		return fmt.Errorf("unsupported provider in session runner: %s", a.Provider)
	}
	cfg := r.cfg.normalized()
	backoff := cfg.ReconnectInitialBackoff
	for {
		if ctx.Err() != nil {
			return nil
		}
		connected, err := r.runSessionOnce(ctx, a, cfg)
		if err == nil || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return nil
		}
		retryIn := applySessionJitter(backoff, cfg.ReconnectJitter)
		r.log.Warn("pecron ingest session error; reconnecting",
			slog.String("provider", a.Provider),
			slog.String("provider_device_ref", providerDeviceLogRef(a.Provider, a.ProviderDeviceID)),
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

func (r *PecronSessionRunner) runSessionOnce(
	ctx context.Context,
	a controlplane.IngestAssignment,
	cfg PecronSessionConfig,
) (connected bool, err error) {
	credential := credentialFromAssignment(a)
	session, err := r.adapter.MQTTSession(ctx, credential, a.ProviderDeviceID)
	if err != nil {
		return false, fmt.Errorf("resolve pecron mqtt session: %w", err)
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

	r.log.Info("pecron ingest session connected",
		slog.String("provider", a.Provider),
		slog.String("provider_device_ref", providerDeviceLogRef(a.Provider, a.ProviderDeviceID)),
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

	if !cfg.DisableSnapshotBootstrap {
		if err := r.publishSnapshot(refreshCtx, a, asyncPublisher, envelopeBuilder, r.nowFn().UTC()); err != nil {
			r.log.Warn("pecron snapshot bootstrap failed; continuing with mqtt session",
				slog.String("provider", a.Provider),
				slog.String("provider_device_ref", providerDeviceLogRef(a.Provider, a.ProviderDeviceID)),
				slog.String("error", err.Error()),
			)
		}
	}
	go func() {
		defer close(refreshDone)
		r.runSnapshotRefreshLoop(refreshCtx, a, asyncPublisher, envelopeBuilder, cfg)
	}()

	state := map[string]any{}
	ref, _ := pecron.ParseProviderDeviceID(a.ProviderDeviceID)
	for {
		select {
		case publishErr := <-asyncPublisher.Errors():
			if publishErr != nil {
				r.log.Warn("pecron ingest publish failed; dropping envelope and keeping mqtt session alive",
					slog.String("provider", a.Provider),
					slog.String("provider_device_ref", providerDeviceLogRef(a.Provider, a.ProviderDeviceID)),
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
			return true, fmt.Errorf("read pecron mqtt message: %w", readErr)
		}
		if !strings.HasSuffix(strings.TrimSpace(msg.Topic), "/bus_") {
			continue
		}
		decoded, err := pecron.DecodeMQTTBusPayload(msg.Payload)
		if err != nil {
			return true, err
		}
		if len(decoded.KV) == 0 {
			continue
		}
		state = pecron.MergeKV(state, decoded.KV)
		device := pecron.Device{
			ProductKey:  ref.ProductKey,
			DeviceKey:   ref.DeviceKey,
			DeviceName:  a.ProductName,
			ProductName: a.Model,
		}
		normalized := pecron.NormalizeTelemetry(device, state)
		if len(normalized.Params) == 0 {
			continue
		}
		envelope, err := envelopeBuilder.BuildProviderNormalizedParams(normalized.Params, r.nowFn().UTC())
		if err != nil {
			return true, fmt.Errorf("build pecron normalized envelope: %w", err)
		}
		if publishErr := asyncPublisher.Publish(ctx, envelope); publishErr != nil {
			if errors.Is(publishErr, context.Canceled) || ctx.Err() != nil {
				return true, nil
			}
			r.log.Warn("pecron ingest publish enqueue failed; dropping envelope and keeping mqtt session alive",
				slog.String("provider", a.Provider),
				slog.String("provider_device_ref", providerDeviceLogRef(a.Provider, a.ProviderDeviceID)),
				slog.String("error", publishErr.Error()),
			)
		}
	}
}

func (r *PecronSessionRunner) connectSubscriber(
	ctx context.Context,
	session pecron.MQTTSession,
	clientID string,
	cfg PecronSessionConfig,
) (mqttSubscriber, string, error) {
	addresses := session.BrokerAddresses()
	if len(addresses) == 0 {
		return nil, "", errors.New("pecron mqtt session has no broker addresses")
	}
	var lastErr error
	for _, address := range addresses {
		subscriber, err := r.newSubscriber(pecron.MQTTConfig{
			Address:        address,
			Path:           session.Path,
			Token:          session.Token,
			ClientID:       clientID,
			KeepAlive:      cfg.KeepAlive,
			ConnectTimeout: cfg.ConnectTimeout,
			ReadTimeout:    cfg.ReadTimeout,
		})
		if err != nil {
			lastErr = fmt.Errorf("init pecron mqtt subscriber for %s: %w", address, err)
			continue
		}
		if err := subscriber.Connect(ctx); err != nil {
			_ = subscriber.Close()
			lastErr = fmt.Errorf("connect pecron mqtt subscriber %s: %w", address, err)
			continue
		}
		if err := subscribePecronTopics(ctx, subscriber, session.Topics, cfg.SubscribeQoS); err != nil {
			_ = subscriber.Close()
			lastErr = fmt.Errorf("subscribe pecron mqtt topics on %s: %w", address, err)
			continue
		}
		if err := requestPecronTelemetry(ctx, subscriber, session.Ref, cfg.SubscribeQoS); err != nil {
			_ = subscriber.Close()
			lastErr = fmt.Errorf("request pecron mqtt telemetry on %s: %w", address, err)
			continue
		}
		return subscriber, address, nil
	}
	if lastErr == nil {
		lastErr = errors.New("pecron mqtt subscriber could not connect")
	}
	return nil, "", lastErr
}

func subscribePecronTopics(ctx context.Context, subscriber mqttSubscriber, topics []string, qos byte) error {
	if len(topics) == 0 {
		return errors.New("pecron mqtt session has no subscribe topics")
	}
	if multi, ok := subscriber.(pecronMultiSubscriber); ok {
		if err := multi.SubscribeMultiple(ctx, topics, qos); err != nil {
			return err
		}
		return nil
	}
	for _, topic := range topics {
		if err := subscriber.Subscribe(ctx, topic, qos); err != nil {
			return fmt.Errorf("subscribe pecron mqtt topic %s: %w", mqttTopicLogRef(topic), err)
		}
	}
	return nil
}

func requestPecronTelemetry(ctx context.Context, subscriber mqttSubscriber, ref pecron.DeviceRef, qos byte) error {
	publisher, ok := subscriber.(pecronPublisher)
	if !ok {
		return nil
	}
	topic := pecron.MQTTPublishTopic(ref)
	if topic == "" {
		return nil
	}
	return publisher.Publish(ctx, topic, pecron.TTLVReadPacket(1), qos)
}

func (r *PecronSessionRunner) runSnapshotRefreshLoop(
	ctx context.Context,
	a controlplane.IngestAssignment,
	asyncPublisher *asyncEnvelopePublisher,
	envelopeBuilder telemetryEnvelopeBuilder,
	cfg PecronSessionConfig,
) {
	for {
		wait := applySessionJitter(cfg.SnapshotRefreshInterval, cfg.SnapshotRefreshJitter)
		if err := r.sleepFn(ctx, wait); err != nil {
			return
		}
		if err := r.publishSnapshot(ctx, a, asyncPublisher, envelopeBuilder, r.nowFn().UTC()); err != nil {
			r.log.Warn("pecron snapshot refresh failed; keeping mqtt session alive",
				slog.String("provider", a.Provider),
				slog.String("provider_device_ref", providerDeviceLogRef(a.Provider, a.ProviderDeviceID)),
				slog.String("error", err.Error()),
			)
		}
	}
}

func (r *PecronSessionRunner) publishSnapshot(
	ctx context.Context,
	a controlplane.IngestAssignment,
	asyncPublisher *asyncEnvelopePublisher,
	envelopeBuilder telemetryEnvelopeBuilder,
	observedAt time.Time,
) error {
	credential := credentialFromAssignment(a)
	snapshotCtx, cancel := context.WithTimeout(ctx, r.cfg.SnapshotFetchTimeout)
	defer cancel()
	refreshedDevice, normalized, err := r.adapter.GetDeviceTelemetrySnapshot(snapshotCtx, credential, a.ProviderDeviceID)
	if err != nil {
		return fmt.Errorf("fetch pecron device snapshot: %w", err)
	}
	if _, err := r.providerDevices.UpsertProviderDevice(ctx, controlplane.UpsertProviderDeviceInput{
		DeviceID:           a.DeviceID,
		Provider:           a.Provider,
		ProviderDeviceID:   a.ProviderDeviceID,
		CredentialID:       a.CredentialID,
		ProductName:        firstNonEmptyString(refreshedDevice.ProductName, a.ProductName),
		Model:              firstNonEmptyString(refreshedDevice.Model, a.Model),
		Capabilities:       normalized.Capabilities,
		Metadata:           normalized.Metadata,
		IsActive:           a.DeviceIsActive,
		IngestDesiredState: a.IngestDesiredState,
	}); err != nil {
		r.log.Warn("pecron snapshot metadata upsert failed; continuing",
			slog.String("provider", a.Provider),
			slog.String("provider_device_ref", providerDeviceLogRef(a.Provider, a.ProviderDeviceID)),
			slog.String("error", err.Error()),
		)
	}
	if len(normalized.Params) == 0 {
		return nil
	}
	envelope, err := envelopeBuilder.BuildProviderNormalizedParams(normalized.Params, observedAt)
	if err != nil {
		return fmt.Errorf("build pecron snapshot envelope: %w", err)
	}
	if err := asyncPublisher.Publish(ctx, envelope); err != nil && !errors.Is(err, context.Canceled) && ctx.Err() == nil {
		r.log.Warn("pecron snapshot publish enqueue failed; dropping snapshot frame",
			slog.String("provider", a.Provider),
			slog.String("provider_device_ref", providerDeviceLogRef(a.Provider, a.ProviderDeviceID)),
			slog.String("error", err.Error()),
		)
	}
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
