package logger

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type recordingHandler struct {
	delay time.Duration
	mu    sync.Mutex
	count int
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *recordingHandler) Handle(_ context.Context, _ slog.Record) error {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	h.mu.Lock()
	h.count++
	h.mu.Unlock()
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *recordingHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *recordingHandler) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

func TestAsyncHandlerDrainsQueuedRecords(t *testing.T) {
	base := &recordingHandler{}
	handler, err := NewAsyncHandler(base, AsyncHandlerConfig{
		QueueSize:   16,
		BypassLevel: slog.LevelWarn,
	})
	if err != nil {
		t.Fatalf("new async handler: %v", err)
	}

	log := slog.New(handler)
	for i := 0; i < 8; i++ {
		log.Info("queued message", slog.Int("i", i))
	}

	handler.Close()

	snapshot := handler.Snapshot()
	if snapshot.DroppedTotal != 0 {
		t.Fatalf("expected no drops, got=%d", snapshot.DroppedTotal)
	}
	if base.Count() != 8 {
		t.Fatalf("expected all records handled, got=%d want=8", base.Count())
	}
}

func TestAsyncHandlerDropsWhenQueueIsFull(t *testing.T) {
	base := &recordingHandler{delay: 3 * time.Millisecond}
	handler, err := NewAsyncHandler(base, AsyncHandlerConfig{
		QueueSize:   2,
		BypassLevel: slog.LevelWarn,
	})
	if err != nil {
		t.Fatalf("new async handler: %v", err)
	}

	log := slog.New(handler)
	for i := 0; i < 64; i++ {
		log.Info("burst message", slog.Int("i", i))
	}

	handler.Close()

	snapshot := handler.Snapshot()
	if snapshot.DroppedTotal == 0 {
		t.Fatalf("expected drops under saturated queue, got=%d", snapshot.DroppedTotal)
	}
}

func TestAsyncHandlerBypassesWarningLevel(t *testing.T) {
	base := &recordingHandler{}
	handler, err := NewAsyncHandler(base, AsyncHandlerConfig{
		QueueSize:   4,
		BypassLevel: slog.LevelWarn,
	})
	if err != nil {
		t.Fatalf("new async handler: %v", err)
	}

	log := slog.New(handler)
	log.Warn("warn path")
	handler.Close()

	snapshot := handler.Snapshot()
	if snapshot.BypassedTotal == 0 {
		t.Fatalf("expected warn to bypass async queue")
	}
}
