package ingestworker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

const (
	defaultMQTTKeepAlive             = 60 * time.Second
	defaultMQTTConnectTimeout        = 10 * time.Second
	defaultMQTTReadTimeout           = 30 * time.Second
	defaultMQTTWriteTimeout          = 15 * time.Second
	defaultMQTTReconnectInitialDelay = 500 * time.Millisecond
	defaultMQTTReconnectMaxDelay     = 15 * time.Second
	defaultMQTTReconnectJitter       = 0.25
	ecoflowAccessKeyInvalidCode      = "8513"
)

var ErrEcoFlowCredentialRejected = errors.New("ecoflow credential rejected")

type mqttSubscriber interface {
	Connect(ctx context.Context) error
	Subscribe(ctx context.Context, topic string, qos byte) error
	ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error)
	Disconnect() error
	Close() error
}

type mqttSubscriberFactory func(cfg ecoflowmqtt.Config) (mqttSubscriber, error)

type ecoFlowCertificationResolver interface {
	GetMQTTCertification(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (ecoflow.GeneralInfoMQTTCertification, error)
}

type sessionSleepFunc func(ctx context.Context, duration time.Duration) error

// EcoFlowSessionConfig controls MQTT session lifecycle defaults for worker sessions.
type EcoFlowSessionConfig struct {
	ShardCount uint32

	KeepAlive      time.Duration
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	SubscribeQoS   byte

	PublishQueueSize      int
	PublishWorkers        int
	PublishEnqueueTimeout time.Duration
	AllowUnorderedPublish bool
	DisableEnvelopeLabels bool

	ReconnectInitialBackoff time.Duration
	ReconnectMaxBackoff     time.Duration
	ReconnectJitter         float64
}

func DefaultEcoFlowSessionConfig() EcoFlowSessionConfig {
	return EcoFlowSessionConfig{
		ShardCount: telemetrybus.DefaultShardCount,

		KeepAlive:      defaultMQTTKeepAlive,
		ConnectTimeout: defaultMQTTConnectTimeout,
		ReadTimeout:    defaultMQTTReadTimeout,
		WriteTimeout:   defaultMQTTWriteTimeout,
		SubscribeQoS:   0,

		PublishQueueSize:      defaultPublishQueueSize,
		PublishWorkers:        defaultPublishWorkers,
		PublishEnqueueTimeout: defaultPublishEnqueueTimeout,
		AllowUnorderedPublish: false,
		DisableEnvelopeLabels: false,

		ReconnectInitialBackoff: defaultMQTTReconnectInitialDelay,
		ReconnectMaxBackoff:     defaultMQTTReconnectMaxDelay,
		ReconnectJitter:         defaultMQTTReconnectJitter,
	}
}

func (c EcoFlowSessionConfig) normalized() EcoFlowSessionConfig {
	cfg := c
	if cfg.ShardCount == 0 {
		cfg.ShardCount = telemetrybus.DefaultShardCount
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = defaultMQTTKeepAlive
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = defaultMQTTConnectTimeout
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = defaultMQTTReadTimeout
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaultMQTTWriteTimeout
	}
	if cfg.SubscribeQoS > 1 {
		cfg.SubscribeQoS = 0
	}
	if cfg.PublishQueueSize <= 0 {
		cfg.PublishQueueSize = defaultPublishQueueSize
	}
	if cfg.PublishWorkers <= 0 {
		cfg.PublishWorkers = defaultPublishWorkers
	}
	if cfg.PublishEnqueueTimeout <= 0 {
		cfg.PublishEnqueueTimeout = defaultPublishEnqueueTimeout
	}
	if cfg.ReconnectInitialBackoff <= 0 {
		cfg.ReconnectInitialBackoff = defaultMQTTReconnectInitialDelay
	}
	if cfg.ReconnectMaxBackoff < cfg.ReconnectInitialBackoff {
		cfg.ReconnectMaxBackoff = defaultMQTTReconnectMaxDelay
		if cfg.ReconnectMaxBackoff < cfg.ReconnectInitialBackoff {
			cfg.ReconnectMaxBackoff = cfg.ReconnectInitialBackoff
		}
	}
	if cfg.ReconnectJitter < 0 {
		cfg.ReconnectJitter = 0
	}
	return cfg
}

func (c EcoFlowSessionConfig) validate() error {
	if c.PublishWorkers > 1 && !c.AllowUnorderedPublish {
		return errors.New("publish_workers > 1 requires allow_unordered_publish=true")
	}
	return nil
}

type EcoFlowSessionRunner struct {
	log       *slog.Logger
	adapter   ecoFlowCertificationResolver
	publisher telemetrybus.EnvelopePublisher
	cfg       EcoFlowSessionConfig

	newSubscriber mqttSubscriberFactory
	sleepFn       sessionSleepFunc
	nowFn         func() time.Time
}

func NewEcoFlowSessionRunner(
	log *slog.Logger,
	adapter *provideradapter.EcoFlowAdapter,
	publisher telemetrybus.EnvelopePublisher,
	cfg EcoFlowSessionConfig,
) (*EcoFlowSessionRunner, error) {
	if log == nil {
		log = slog.Default()
	}
	if adapter == nil {
		adapter = provideradapter.NewEcoFlowAdapter(nil)
	}
	if publisher == nil {
		return nil, errors.New("telemetry envelope publisher is required")
	}
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid ecoflow session config: %w", err)
	}
	return &EcoFlowSessionRunner{
		log:           log,
		adapter:       adapter,
		publisher:     publisher,
		cfg:           cfg,
		newSubscriber: defaultMQTTSubscriberFactory,
		sleepFn:       sessionSleepContext,
		nowFn:         time.Now,
	}, nil
}

func defaultMQTTSubscriberFactory(cfg ecoflowmqtt.Config) (mqttSubscriber, error) {
	return ecoflowmqtt.NewSubscriber(cfg)
}

func (r *EcoFlowSessionRunner) Run(ctx context.Context, a controlplane.IngestAssignment) error {
	if sanitizeProvider(a.Provider) != controlplane.ProviderEcoFlow {
		return fmt.Errorf("unsupported provider in session runner: %s", a.Provider)
	}

	cfg := r.cfg.normalized()
	backoff := cfg.ReconnectInitialBackoff

	for {
		if ctx.Err() != nil {
			return nil
		}

		connected, err := r.runSessionOnce(ctx, a, cfg)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return nil
		}
		if ecoflow.IsBusinessErrorCode(err, ecoflowAccessKeyInvalidCode) {
			r.log.Warn("ecoflow credential rejected; stopping session for credential refresh",
				slog.String("provider", a.Provider),
				slog.String("provider_device_id", strings.TrimSpace(a.ProviderDeviceID)),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("%w: %v", ErrEcoFlowCredentialRejected, err)
		}
		if isMQTTConnectRejected(err) {
			r.log.Warn("ecoflow mqtt connect rejected by broker; refreshing certification and retrying",
				slog.String("provider", a.Provider),
				slog.String("provider_device_id", strings.TrimSpace(a.ProviderDeviceID)),
				slog.String("error", err.Error()),
			)
		}

		if connected {
			backoff = cfg.ReconnectInitialBackoff
		}
		retryIn := applySessionJitter(backoff, cfg.ReconnectJitter)
		r.log.Warn("ecoflow ingest session error; reconnecting",
			slog.String("provider", a.Provider),
			slog.String("provider_device_id", strings.TrimSpace(a.ProviderDeviceID)),
			slog.String("error", err.Error()),
			slog.Duration("retry_in", retryIn),
		)
		if sleepErr := r.sleepFn(ctx, retryIn); sleepErr != nil {
			return nil
		}
		backoff = nextBackoff(backoff, cfg.ReconnectMaxBackoff)
	}
}

func (r *EcoFlowSessionRunner) runSessionOnce(
	ctx context.Context,
	a controlplane.IngestAssignment,
	cfg EcoFlowSessionConfig,
) (connected bool, err error) {
	credential := credentialFromAssignment(a)
	cert, err := r.adapter.GetMQTTCertification(ctx, credential, a.ProviderDeviceID)
	if err != nil {
		return false, fmt.Errorf("resolve mqtt certification for %s/%s: %w", a.Provider, a.ProviderDeviceID, err)
	}

	address, topic, err := mqttAddressAndTopic(cert, a.ProviderDeviceID)
	if err != nil {
		return false, err
	}
	subscriber, err := r.newSubscriber(ecoflowmqtt.Config{
		Address:        address,
		Username:       strings.TrimSpace(cert.CertificateAccount),
		Password:       strings.TrimSpace(cert.CertificatePassword),
		ClientID:       ecoflowmqtt.BuildClientIDFromSN(a.ProviderDeviceID),
		KeepAlive:      cfg.KeepAlive,
		ConnectTimeout: cfg.ConnectTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
	})
	if err != nil {
		return false, fmt.Errorf("init mqtt subscriber: %w", err)
	}
	defer func() { _ = subscriber.Close() }()

	if err := subscriber.Connect(ctx); err != nil {
		return false, fmt.Errorf("connect mqtt subscriber: %w", err)
	}
	if err := subscriber.Subscribe(ctx, topic, cfg.SubscribeQoS); err != nil {
		return false, fmt.Errorf("subscribe mqtt topic: %w", err)
	}
	r.log.Info("ecoflow ingest session connected",
		slog.String("provider", a.Provider),
		slog.String("provider_device_id", strings.TrimSpace(a.ProviderDeviceID)),
		slog.String("topic", topic),
		slog.String("broker", address),
	)

	asyncPublisher := newAsyncEnvelopePublisher(
		ctx,
		r.publisher,
		cfg.PublishQueueSize,
		cfg.PublishWorkers,
		cfg.PublishEnqueueTimeout,
	)
	defer func() { _ = asyncPublisher.Close() }()

	envelopeBuilder := newTelemetryEnvelopeBuilder(a, cfg)

	for {
		select {
		case publishErr := <-asyncPublisher.Errors():
			if publishErr != nil {
				r.log.Warn("ecoflow ingest publish failed; dropping envelope and keeping mqtt session alive",
					slog.String("provider", a.Provider),
					slog.String("provider_device_id", strings.TrimSpace(a.ProviderDeviceID)),
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
			return true, fmt.Errorf("read mqtt message: %w", readErr)
		}
		r.log.Info("ecoflow ingest mqtt message received",
			slog.String("provider", a.Provider),
			slog.String("provider_device_id", strings.TrimSpace(a.ProviderDeviceID)),
			slog.String("topic", msg.Topic),
			slog.Int("payload_bytes", len(msg.Payload)),
			slog.String("payload_raw", string(msg.Payload)),
		)
		envelope, buildErr := envelopeBuilder.Build(msg, r.nowFn().UTC())
		if buildErr != nil {
			return true, fmt.Errorf("build telemetry envelope: %w", buildErr)
		}
		if publishErr := asyncPublisher.Publish(ctx, envelope); publishErr != nil {
			if errors.Is(publishErr, context.Canceled) || ctx.Err() != nil {
				return true, nil
			}
			r.log.Warn("ecoflow ingest publish enqueue failed; dropping envelope and keeping mqtt session alive",
				slog.String("provider", a.Provider),
				slog.String("provider_device_id", strings.TrimSpace(a.ProviderDeviceID)),
				slog.String("error", publishErr.Error()),
			)
			continue
		}
	}
}

func credentialFromAssignment(a controlplane.IngestAssignment) controlplane.ProviderCredential {
	return controlplane.ProviderCredential{
		ID:        a.CredentialID,
		Provider:  a.Provider,
		AccessKey: a.AccessKey,
		SecretKey: a.SecretKey,
		IsActive:  a.CredentialIsActive,
	}
}

func mqttAddressAndTopic(cert ecoflow.GeneralInfoMQTTCertification, providerDeviceID string) (address string, topic string, err error) {
	url := strings.TrimSpace(cert.URL)
	port := strings.TrimSpace(cert.Port)
	account := strings.TrimSpace(cert.CertificateAccount)
	if url == "" || port == "" {
		return "", "", errors.New("mqtt certification missing broker url/port")
	}
	if account == "" {
		return "", "", errors.New("mqtt certification missing certificate account")
	}
	sn := strings.ToUpper(strings.TrimSpace(providerDeviceID))
	if sn == "" {
		return "", "", errors.New("provider_device_id is required")
	}
	return fmt.Sprintf("%s:%s", url, port), fmt.Sprintf("/open/%s/%s/quota", account, sn), nil
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	if next <= 0 {
		return max
	}
	return next
}

func sessionSleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func applySessionJitter(base time.Duration, factor float64) time.Duration {
	if base <= 0 || factor <= 0 {
		return base
	}
	maxShift := time.Duration(float64(base) * factor)
	if maxShift <= 0 {
		return base
	}
	min := base - maxShift
	max := base + maxShift
	if min < 100*time.Millisecond {
		min = 100 * time.Millisecond
	}
	if max < min {
		max = min
	}
	if max == min {
		return min
	}
	delta := max - min
	return min + time.Duration(mathrand.Int63n(int64(delta)+1))
}

func isMQTTConnectRejected(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "connect rejected") ||
		strings.Contains(lower, "return code=5") ||
		strings.Contains(lower, "not authorized")
}
