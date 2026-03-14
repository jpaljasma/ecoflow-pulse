package startupretry

import (
	"context"
	"log/slog"
	"time"
)

type Options struct {
	Timeout        time.Duration
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

func DefaultOptions() Options {
	return Options{
		Timeout:        2 * time.Minute,
		InitialBackoff: time.Second,
		MaxBackoff:     10 * time.Second,
	}
}

func Retry[T any](
	parent context.Context,
	log *slog.Logger,
	operation string,
	options Options,
	fn func(context.Context) (T, error),
) (T, error) {
	var zero T
	if options.Timeout <= 0 {
		options.Timeout = DefaultOptions().Timeout
	}
	if options.InitialBackoff <= 0 {
		options.InitialBackoff = DefaultOptions().InitialBackoff
	}
	if options.MaxBackoff < options.InitialBackoff {
		options.MaxBackoff = DefaultOptions().MaxBackoff
		if options.MaxBackoff < options.InitialBackoff {
			options.MaxBackoff = options.InitialBackoff
		}
	}

	ctx, cancel := context.WithTimeout(parent, options.Timeout)
	defer cancel()

	backoff := options.InitialBackoff
	for attempt := 1; ; attempt++ {
		value, err := fn(ctx)
		if err == nil {
			if attempt > 1 && log != nil {
				log.Info("startup dependency recovered", slog.String("operation", operation), slog.Int("attempt", attempt))
			}
			return value, nil
		}
		if ctx.Err() != nil {
			if log != nil {
				log.Error("startup dependency retry exhausted", slog.String("operation", operation), slog.Int("attempt", attempt), slog.String("error", err.Error()))
			}
			return zero, err
		}
		if log != nil {
			log.Warn(
				"startup dependency unavailable, retrying",
				slog.String("operation", operation),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", backoff),
				slog.String("error", err.Error()),
			)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, err
		case <-timer.C:
		}
		backoff *= 2
		if backoff > options.MaxBackoff {
			backoff = options.MaxBackoff
		}
	}
}
