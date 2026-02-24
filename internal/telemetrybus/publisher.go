package telemetrybus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"strings"
	"sync"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	defaultNATSURL             = "nats://127.0.0.1:4222"
	defaultNATSConnectTimeout  = 5 * time.Second
	defaultNATSReconnectWait   = 2 * time.Second
	defaultNATSReconnectJitter = 2 * time.Second
	defaultNATSPingInterval    = 20 * time.Second
	defaultNATSMaxPingsOut     = 3
	defaultNATSMaxReconnects   = -1

	defaultPublishTimeout      = 3 * time.Second
	defaultPublishMaxRetries   = 3
	defaultPublishRetryInitial = 50 * time.Millisecond
	defaultPublishRetryMax     = 500 * time.Millisecond
	defaultPublishRetryJitter  = 0.20
)

type EnvelopePublisher interface {
	PublishEnvelope(ctx context.Context, envelope *envelopev1.TelemetryEnvelope) error
	Close() error
}

// PublishEnvelope validates inputs and forwards publish to the concrete
// publisher implementation. Shared helper keeps callers/tests consistent.
func PublishEnvelope(ctx context.Context, publisher EnvelopePublisher, envelope *envelopev1.TelemetryEnvelope) error {
	if publisher == nil {
		return errors.New("publisher is required")
	}
	return publisher.PublishEnvelope(ctx, envelope)
}

type NATSConnConfig struct {
	URLs            []string
	Name            string
	ConnectTimeout  time.Duration
	ReconnectWait   time.Duration
	ReconnectJitter time.Duration
	PingInterval    time.Duration
	MaxPingsOut     int
	MaxReconnects   int
}

type NATSEnvelopePublisherOptions struct {
	// StripLabels removes optional envelope labels before protobuf marshal.
	// This reduces map-serialization allocations in high-throughput paths.
	StripLabels bool
	// UseJetStream publishes envelopes via JetStream PublishMsg and waits for ack.
	UseJetStream bool
	// PublishTimeout bounds a single publish attempt.
	PublishTimeout time.Duration
	// PublishMaxRetries controls retry attempts after the first publish attempt fails.
	PublishMaxRetries int
	// PublishRetryInitialBackoff is the starting retry delay.
	PublishRetryInitialBackoff time.Duration
	// PublishRetryMaxBackoff caps exponential retry growth.
	PublishRetryMaxBackoff time.Duration
	// PublishRetryJitter applies full-jitter percentage to retry delays.
	PublishRetryJitter float64
}

func (o NATSEnvelopePublisherOptions) normalized() NATSEnvelopePublisherOptions {
	out := o
	if out.PublishTimeout <= 0 {
		out.PublishTimeout = defaultPublishTimeout
	}
	if out.PublishMaxRetries < 0 {
		out.PublishMaxRetries = 0
	}
	if out.PublishRetryInitialBackoff <= 0 {
		out.PublishRetryInitialBackoff = defaultPublishRetryInitial
	}
	if out.PublishRetryMaxBackoff < out.PublishRetryInitialBackoff {
		out.PublishRetryMaxBackoff = defaultPublishRetryMax
		if out.PublishRetryMaxBackoff < out.PublishRetryInitialBackoff {
			out.PublishRetryMaxBackoff = out.PublishRetryInitialBackoff
		}
	}
	if out.PublishRetryJitter < 0 {
		out.PublishRetryJitter = 0
	}
	return out
}

func DefaultNATSConnConfig(urls []string) NATSConnConfig {
	return NATSConnConfig{
		URLs:            append([]string(nil), urls...),
		ConnectTimeout:  defaultNATSConnectTimeout,
		ReconnectWait:   defaultNATSReconnectWait,
		ReconnectJitter: defaultNATSReconnectJitter,
		PingInterval:    defaultNATSPingInterval,
		MaxPingsOut:     defaultNATSMaxPingsOut,
		MaxReconnects:   defaultNATSMaxReconnects,
	}
}

func DialNATS(log *slog.Logger, cfg NATSConnConfig) (*nats.Conn, error) {
	if log == nil {
		log = slog.Default()
	}

	cfg = cfg.normalized()
	urls := strings.Join(cfg.URLs, ",")
	jitterSrc := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	var jitterMu sync.Mutex

	opts := []nats.Option{
		nats.Name(strings.TrimSpace(cfg.Name)),
		nats.Timeout(cfg.ConnectTimeout),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.PingInterval(cfg.PingInterval),
		nats.MaxPingsOutstanding(cfg.MaxPingsOut),
		nats.CustomReconnectDelay(func(_ int) time.Duration {
			jitterMu.Lock()
			defer jitterMu.Unlock()
			if cfg.ReconnectJitter <= 0 {
				return cfg.ReconnectWait
			}
			// Full jitter on top of base wait helps avoid reconnect herds.
			return cfg.ReconnectWait + time.Duration(jitterSrc.Int63n(int64(cfg.ReconnectJitter)))
		}),
		nats.DisconnectErrHandler(func(conn *nats.Conn, err error) {
			if err == nil {
				log.Warn("nats disconnected")
				return
			}
			log.Warn("nats disconnected", slog.String("error", err.Error()))
		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			log.Info("nats reconnected", slog.String("url", conn.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(conn *nats.Conn) {
			lastErr := conn.LastError()
			if lastErr == nil {
				log.Info("nats connection closed")
				return
			}
			log.Warn("nats connection closed", slog.String("error", lastErr.Error()))
		}),
	}

	conn, err := nats.Connect(urls, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	return conn, nil
}

func (c NATSConnConfig) normalized() NATSConnConfig {
	cfg := c
	cfg.URLs = compactStrings(cfg.URLs)
	if len(cfg.URLs) == 0 {
		cfg.URLs = []string{defaultNATSURL}
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = defaultNATSConnectTimeout
	}
	if cfg.ReconnectWait <= 0 {
		cfg.ReconnectWait = defaultNATSReconnectWait
	}
	if cfg.ReconnectJitter < 0 {
		cfg.ReconnectJitter = 0
	}
	if cfg.PingInterval <= 0 {
		cfg.PingInterval = defaultNATSPingInterval
	}
	if cfg.MaxPingsOut <= 0 {
		cfg.MaxPingsOut = defaultNATSMaxPingsOut
	}
	if cfg.MaxReconnects == 0 {
		cfg.MaxReconnects = defaultNATSMaxReconnects
	}
	return cfg
}

type NATSEnvelopePublisher struct {
	cfg            SubjectConfig
	subjectByShard []string
	options        NATSEnvelopePublisherOptions
	publish        func(ctx context.Context, msg *nats.Msg) error
	sleepFn        func(ctx context.Context, d time.Duration) error
	randFloat64Fn  func() float64
	closeFn        func() error
}

func NewNATSEnvelopePublisher(conn *nats.Conn, cfg SubjectConfig) (*NATSEnvelopePublisher, error) {
	return NewNATSEnvelopePublisherWithOptions(conn, cfg, NATSEnvelopePublisherOptions{})
}

func NewNATSEnvelopePublisherWithOptions(
	conn *nats.Conn,
	cfg SubjectConfig,
	options NATSEnvelopePublisherOptions,
) (*NATSEnvelopePublisher, error) {
	if conn == nil {
		return nil, errors.New("nats connection is required")
	}
	options = options.normalized()
	normalized := cfg.Normalized()
	publishFn := func(_ context.Context, msg *nats.Msg) error {
		return conn.PublishMsg(msg)
	}
	if options.UseJetStream {
		js, err := conn.JetStream()
		if err != nil {
			return nil, fmt.Errorf("init jetstream context: %w", err)
		}
		publishFn = func(ctx context.Context, msg *nats.Msg) error {
			publishCtx := ctx
			cancel := func() {}
			if options.PublishTimeout > 0 {
				publishCtx, cancel = context.WithTimeout(ctx, options.PublishTimeout)
			}
			defer cancel()
			_, err := js.PublishMsg(msg, nats.Context(publishCtx))
			return err
		}
	}
	return &NATSEnvelopePublisher{
		cfg:            normalized,
		subjectByShard: buildIngestSubjectCache(normalized),
		options:        options,
		publish:        publishFn,
		sleepFn:        sleepContext,
		randFloat64Fn:  mathrand.Float64,
		closeFn: func() error {
			if err := conn.Drain(); err != nil {
				conn.Close()
				return err
			}
			return nil
		},
	}, nil
}

func newNATSEnvelopePublisherForTest(cfg SubjectConfig, publishFn func(msg *nats.Msg) error) *NATSEnvelopePublisher {
	normalized := cfg.Normalized()
	return &NATSEnvelopePublisher{
		cfg:            normalized,
		subjectByShard: buildIngestSubjectCache(normalized),
		options:        NATSEnvelopePublisherOptions{}.normalized(),
		publish: func(_ context.Context, msg *nats.Msg) error {
			return publishFn(msg)
		},
		sleepFn:       sleepContext,
		randFloat64Fn: func() float64 { return 0 },
		closeFn:       func() error { return nil },
	}
}

func (p *NATSEnvelopePublisher) PublishEnvelope(ctx context.Context, envelope *envelopev1.TelemetryEnvelope) error {
	if envelope == nil {
		return errors.New("envelope is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	encodedEnvelope := envelope
	if p.options.StripLabels && len(envelope.GetLabels()) > 0 {
		clone := *envelope
		clone.Labels = nil
		encodedEnvelope = &clone
	}

	data, err := proto.Marshal(encodedEnvelope)
	if err != nil {
		return fmt.Errorf("marshal envelope protobuf: %w", err)
	}

	msg := &nats.Msg{
		Subject: p.subjectForShard(encodedEnvelope.GetShard()),
		Data:    data,
	}
	if id := encodedEnvelope.GetEnvelopeId(); id != "" {
		msg.Header = nats.Header{nats.MsgIdHdr: []string{id}}
	}

	if err := p.publishWithRetry(ctx, msg); err != nil {
		return fmt.Errorf("publish envelope to nats: %w", err)
	}
	return nil
}

func (p *NATSEnvelopePublisher) publishWithRetry(ctx context.Context, msg *nats.Msg) error {
	if err := p.publish(ctx, msg); err == nil {
		return nil
	} else {
		if !isRetryablePublishError(ctx, err) || p.options.PublishMaxRetries == 0 {
			return err
		}
		lastErr := err
		backoff := p.options.PublishRetryInitialBackoff
		for attempt := 0; attempt < p.options.PublishMaxRetries; attempt++ {
			delay := applyPublishJitter(backoff, p.options.PublishRetryJitter, p.randFloat64Fn)
			if sleepErr := p.sleepFn(ctx, delay); sleepErr != nil {
				return sleepErr
			}
			if err = p.publish(ctx, msg); err == nil {
				return nil
			}
			lastErr = err
			if !isRetryablePublishError(ctx, err) {
				break
			}
			backoff = nextBackoff(backoff, p.options.PublishRetryMaxBackoff)
		}
		return lastErr
	}
}

func (p *NATSEnvelopePublisher) Close() error {
	if p == nil || p.closeFn == nil {
		return nil
	}
	return p.closeFn()
}

func isRetryablePublishError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

func applyPublishJitter(base time.Duration, factor float64, randFloat64Fn func() float64) time.Duration {
	if base <= 0 || factor <= 0 {
		return base
	}
	if randFloat64Fn == nil {
		randFloat64Fn = mathrand.Float64
	}
	maxShift := time.Duration(float64(base) * factor)
	if maxShift <= 0 {
		return base
	}
	min := base - maxShift
	if min < 0 {
		min = 0
	}
	max := base + maxShift
	if max <= min {
		return min
	}
	return min + time.Duration(randFloat64Fn()*float64(max-min))
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	if current <= 0 {
		current = defaultPublishRetryInitial
	}
	next := current * 2
	if next <= 0 {
		next = max
	}
	if max > 0 && next > max {
		return max
	}
	return next
}

func compactStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		clean := strings.TrimSpace(v)
		if clean == "" {
			continue
		}
		out = append(out, clean)
	}
	return out
}

func buildIngestSubjectCache(cfg SubjectConfig) []string {
	cfg = cfg.Normalized()
	out := make([]string, cfg.ShardCount)
	for i := uint32(0); i < cfg.ShardCount; i++ {
		out[i] = IngestSubject(cfg, i)
	}
	return out
}

func (p *NATSEnvelopePublisher) subjectForShard(shard uint32) string {
	if int(shard) < len(p.subjectByShard) {
		return p.subjectByShard[shard]
	}
	return IngestSubject(p.cfg, shard)
}
