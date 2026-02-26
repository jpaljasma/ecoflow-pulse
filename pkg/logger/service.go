package logger

import (
	"context"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"
)

const (
	defaultLogMetricsInterval = 30 * time.Second
)

// ServiceConfig controls standard service logger construction.
type ServiceConfig struct {
	Component string
	Out       io.Writer
	Level     slog.Level

	AsyncEnabled     bool
	AsyncQueueSize   int
	AsyncBypassLevel slog.Level
}

// DefaultServiceConfig returns recommended defaults for long-running services.
func DefaultServiceConfig(component string) ServiceConfig {
	return ServiceConfig{
		Component:        strings.TrimSpace(component),
		Out:              os.Stdout,
		Level:            slog.LevelInfo,
		AsyncEnabled:     true,
		AsyncQueueSize:   defaultAsyncQueueSize,
		AsyncBypassLevel: slog.LevelWarn,
	}
}

// BuildServiceLogger returns a logger with unified structured output and async queueing.
func BuildServiceLogger(cfg ServiceConfig) (*slog.Logger, *AsyncHandler, error) {
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	base := NewJSONHandler(cfg.Out, cfg.Level)
	if cfg.Component != "" {
		base = base.WithAttrs([]slog.Attr{slog.String("component", cfg.Component)})
	}
	if !cfg.AsyncEnabled {
		return slog.New(base), nil, nil
	}
	asyncHandler, err := NewAsyncHandler(base, AsyncHandlerConfig{
		QueueSize:   cfg.AsyncQueueSize,
		BypassLevel: cfg.AsyncBypassLevel,
	})
	if err != nil {
		return nil, nil, err
	}
	return slog.New(asyncHandler), asyncHandler, nil
}

// ParseLevel parses text log levels and falls back on invalid input.
func ParseLevel(raw string, fallback slog.Level) slog.Level {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(strings.ToUpper(value))); err == nil {
		return parsed
	}
	if err := parsed.UnmarshalText([]byte(strings.ToLower(value))); err == nil {
		return parsed
	}
	return fallback
}

// LogMetricsSnapshot writes async logger queue/counter metrics.
func LogMetricsSnapshot(log *slog.Logger, component string, snapshot AsyncHandlerSnapshot) {
	if log == nil {
		return
	}
	attrs := []any{
		slog.String("component", strings.TrimSpace(component)),
		slog.Int("queue_depth", snapshot.QueueDepth),
		slog.Int("queue_capacity", snapshot.QueueCapacity),
		slog.Uint64("enqueued_total", snapshot.EnqueuedTotal),
		slog.Uint64("processed_total", snapshot.ProcessedTotal),
		slog.Uint64("dropped_total", snapshot.DroppedTotal),
		slog.Uint64("bypassed_total", snapshot.BypassedTotal),
	}
	log.Info("logging_async_metrics", attrs...)
}

// StartAsyncMetricsReporter periodically logs async queue/counter metrics with initial jitter.
func StartAsyncMetricsReporter(
	ctx context.Context,
	log *slog.Logger,
	component string,
	handler *AsyncHandler,
	interval time.Duration,
) func() {
	if ctx == nil || log == nil || handler == nil || interval <= 0 {
		return func() {}
	}
	reportCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	jitterRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	go func() {
		defer close(done)
		maxJitter := int64(interval) / 4
		if maxJitter < 1 {
			maxJitter = 1
		}
		initialJitter := time.Duration(jitterRand.Int63n(maxJitter))
		if initialJitter > 0 {
			timer := time.NewTimer(initialJitter)
			select {
			case <-reportCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			LogMetricsSnapshot(log, component, handler.Snapshot())
			select {
			case <-reportCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

func DefaultLogMetricsInterval() time.Duration {
	return defaultLogMetricsInterval
}
