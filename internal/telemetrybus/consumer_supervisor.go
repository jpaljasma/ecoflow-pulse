package telemetrybus

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	defaultConsumerMonitorInterval = 10 * time.Second
	defaultConsumerRetryBase       = 1 * time.Second
	defaultConsumerRetryMax        = 30 * time.Second
)

type QueueSubscription interface {
	Unsubscribe() error
	IsValid() bool
}

type QueueSubscribeFn func(handler nats.MsgHandler) (QueueSubscription, error)

type ConsumerInfoProvider interface {
	ConsumerInfo(stream, consumer string, opts ...nats.JSOpt) (*nats.ConsumerInfo, error)
}

type ConsumerSupervisorConfig struct {
	StreamName      string
	Durable         string
	DrainTimeout    time.Duration
	MonitorInterval time.Duration
	RetryBase       time.Duration
	RetryMax        time.Duration
}

func (c ConsumerSupervisorConfig) normalized() ConsumerSupervisorConfig {
	out := c
	if out.MonitorInterval <= 0 {
		out.MonitorInterval = defaultConsumerMonitorInterval
	}
	if out.RetryBase <= 0 {
		out.RetryBase = defaultConsumerRetryBase
	}
	if out.RetryMax <= 0 {
		out.RetryMax = defaultConsumerRetryMax
	}
	if out.RetryMax < out.RetryBase {
		out.RetryMax = out.RetryBase
	}
	return out
}

func RunConsumerSupervisor(
	ctx context.Context,
	log *slog.Logger,
	infoProvider ConsumerInfoProvider,
	subscribe QueueSubscribeFn,
	handler nats.MsgHandler,
	tracker *MsgHandlerTracker,
	cfg ConsumerSupervisorConfig,
) error {
	if subscribe == nil {
		return errors.New("queue subscribe function is required")
	}
	if infoProvider == nil {
		return errors.New("consumer info provider is required")
	}
	if log == nil {
		log = slog.Default()
	}

	cfg = cfg.normalized()
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		sub, err := subscribe(handler)
		if err != nil {
			attempt += 1
			delay := computeConsumerRetryBackoff(cfg.RetryBase, cfg.RetryMax, attempt)
			log.Warn("queue consumer subscribe failed; retrying",
				slog.String("stream", cfg.StreamName),
				slog.String("durable", cfg.Durable),
				slog.Int("attempt", attempt),
				slog.Duration("retry_in", delay),
				slog.String("error", err.Error()),
			)
			if !waitForConsumerRetry(ctx, delay) {
				return nil
			}
			continue
		}

		attempt = 0
		if err := superviseSubscription(ctx, log, infoProvider, sub, tracker, cfg); err != nil {
			return err
		}
	}
}

func superviseSubscription(
	ctx context.Context,
	log *slog.Logger,
	infoProvider ConsumerInfoProvider,
	sub QueueSubscription,
	tracker *MsgHandlerTracker,
	cfg ConsumerSupervisorConfig,
) error {
	ticker := time.NewTicker(cfg.MonitorInterval)
	defer ticker.Stop()
	defer cleanupSubscription(log, sub, tracker, cfg)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			ok, reason, err := consumerHealthy(infoProvider, sub, cfg)
			if ok {
				continue
			}
			attrs := []any{
				slog.String("stream", cfg.StreamName),
				slog.String("durable", cfg.Durable),
				slog.String("reason", reason),
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
			}
			log.Warn("queue consumer unhealthy; recreating subscription", attrs...)
			return nil
		}
	}
}

func consumerHealthy(infoProvider ConsumerInfoProvider, sub QueueSubscription, cfg ConsumerSupervisorConfig) (bool, string, error) {
	if sub == nil || !sub.IsValid() {
		return false, "subscription_invalid", nil
	}
	_, err := infoProvider.ConsumerInfo(cfg.StreamName, cfg.Durable)
	if err == nil {
		return true, "", nil
	}
	switch {
	case errors.Is(err, nats.ErrConsumerNotFound):
		return false, "consumer_missing", nil
	case errors.Is(err, nats.ErrStreamNotFound):
		return false, "stream_missing", nil
	default:
		return false, "consumer_check_failed", err
	}
}

func cleanupSubscription(log *slog.Logger, sub QueueSubscription, tracker *MsgHandlerTracker, cfg ConsumerSupervisorConfig) {
	if sub == nil {
		return
	}
	if err := sub.Unsubscribe(); err != nil && !errors.Is(err, nats.ErrBadSubscription) {
		log.Warn("queue consumer unsubscribe failed", slog.String("error", err.Error()))
	}
	if tracker != nil && !tracker.WaitForIdle(cfg.DrainTimeout) {
		log.Warn("queue consumer handler drain timeout",
			slog.String("stream", cfg.StreamName),
			slog.String("durable", cfg.Durable),
		)
	}
}

func computeConsumerRetryBackoff(base, max time.Duration, attempt int) time.Duration {
	if attempt <= 1 {
		return base
	}
	exponential := base * time.Duration(1<<min(attempt-1, 10))
	if exponential > max {
		exponential = max
	}
	if exponential <= base {
		return base
	}
	jitterWindow := exponential - base
	return base + time.Duration(rand.Int63n(int64(jitterWindow)+1))
}

func waitForConsumerRetry(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
