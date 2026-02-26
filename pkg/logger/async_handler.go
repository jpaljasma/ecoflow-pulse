package logger

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
)

const (
	defaultAsyncQueueSize = 8192
)

// AsyncHandlerConfig controls bounded async logging behavior.
type AsyncHandlerConfig struct {
	QueueSize   int
	BypassLevel slog.Level
}

// AsyncHandlerSnapshot is a point-in-time queue and throughput view.
type AsyncHandlerSnapshot struct {
	QueueDepth     int
	QueueCapacity  int
	EnqueuedTotal  uint64
	ProcessedTotal uint64
	DroppedTotal   uint64
	BypassedTotal  uint64
}

// AsyncHandlerStats stores counters for log pipeline SLO visibility.
type AsyncHandlerStats struct {
	queueCapacity  int
	queueDepth     atomic.Int64
	enqueuedTotal  atomic.Uint64
	processedTotal atomic.Uint64
	droppedTotal   atomic.Uint64
	bypassedTotal  atomic.Uint64
}

func (s *AsyncHandlerStats) snapshot() AsyncHandlerSnapshot {
	if s == nil {
		return AsyncHandlerSnapshot{}
	}
	depth := int(s.queueDepth.Load())
	if depth < 0 {
		depth = 0
	}
	return AsyncHandlerSnapshot{
		QueueDepth:     depth,
		QueueCapacity:  s.queueCapacity,
		EnqueuedTotal:  s.enqueuedTotal.Load(),
		ProcessedTotal: s.processedTotal.Load(),
		DroppedTotal:   s.droppedTotal.Load(),
		BypassedTotal:  s.bypassedTotal.Load(),
	}
}

type asyncRecord struct {
	ctx     context.Context
	record  slog.Record
	handler slog.Handler
}

type asyncCore struct {
	queue     chan asyncRecord
	stop      chan struct{}
	stats     *AsyncHandlerStats
	bypassMin slog.Level
	closed    atomic.Bool
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func newAsyncCore(cfg AsyncHandlerConfig) *asyncCore {
	capacity := cfg.QueueSize
	if capacity <= 0 {
		capacity = defaultAsyncQueueSize
	}
	core := &asyncCore{
		queue:     make(chan asyncRecord, capacity),
		stop:      make(chan struct{}),
		stats:     &AsyncHandlerStats{queueCapacity: capacity},
		bypassMin: cfg.BypassLevel,
	}
	core.wg.Add(1)
	go core.run()
	return core
}

func (c *asyncCore) run() {
	defer c.wg.Done()
	for {
		select {
		case rec := <-c.queue:
			c.process(rec)
		case <-c.stop:
			// Drain remaining buffered records before exit.
			for {
				select {
				case rec := <-c.queue:
					c.process(rec)
				default:
					return
				}
			}
		}
	}
}

func (c *asyncCore) process(rec asyncRecord) {
	c.stats.queueDepth.Add(-1)
	if rec.handler != nil {
		_ = rec.handler.Handle(rec.ctx, rec.record)
	}
	c.stats.processedTotal.Add(1)
}

func (c *asyncCore) close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		close(c.stop)
		c.wg.Wait()
	})
}

func (c *asyncCore) statsSnapshot() AsyncHandlerSnapshot {
	if c == nil {
		return AsyncHandlerSnapshot{}
	}
	return c.stats.snapshot()
}

// AsyncHandler wraps a slog handler with bounded async queueing.
type AsyncHandler struct {
	base slog.Handler
	core *asyncCore
}

// NewAsyncHandler wraps the provided base handler with bounded async behavior.
func NewAsyncHandler(base slog.Handler, cfg AsyncHandlerConfig) (*AsyncHandler, error) {
	if base == nil {
		return nil, errors.New("base slog handler is required")
	}
	return &AsyncHandler{
		base: base,
		core: newAsyncCore(cfg),
	}, nil
}

func (h *AsyncHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h == nil || h.base == nil {
		return false
	}
	return h.base.Enabled(ctx, level)
}

func (h *AsyncHandler) Handle(ctx context.Context, record slog.Record) error {
	if h == nil || h.base == nil {
		return nil
	}
	if !h.base.Enabled(ctx, record.Level) {
		return nil
	}
	if h.core == nil {
		return h.base.Handle(ctx, record)
	}
	if h.core.closed.Load() || record.Level >= h.core.bypassMin {
		h.core.stats.bypassedTotal.Add(1)
		return h.base.Handle(ctx, record)
	}

	select {
	case h.core.queue <- asyncRecord{
		ctx:     ctx,
		record:  record.Clone(),
		handler: h.base,
	}:
		h.core.stats.enqueuedTotal.Add(1)
		h.core.stats.queueDepth.Add(1)
		return nil
	default:
		h.core.stats.droppedTotal.Add(1)
		return nil
	}
}

func (h *AsyncHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h == nil {
		return h
	}
	return &AsyncHandler{
		base: h.base.WithAttrs(attrs),
		core: h.core,
	}
}

func (h *AsyncHandler) WithGroup(name string) slog.Handler {
	if h == nil {
		return h
	}
	return &AsyncHandler{
		base: h.base.WithGroup(name),
		core: h.core,
	}
}

// Close drains queued log records.
func (h *AsyncHandler) Close() {
	if h == nil || h.core == nil {
		return
	}
	h.core.close()
}

// Snapshot returns queue/counter metrics for this handler.
func (h *AsyncHandler) Snapshot() AsyncHandlerSnapshot {
	if h == nil || h.core == nil {
		return AsyncHandlerSnapshot{}
	}
	return h.core.statsSnapshot()
}
