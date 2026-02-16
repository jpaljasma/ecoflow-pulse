package main

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type asyncUIWriter struct {
	mu      sync.RWMutex
	out     io.Writer
	queue   chan string
	wg      sync.WaitGroup
	closed  bool
	dropped atomic.Uint64
	errOnce sync.Once
	err     atomic.Value
}

func newAsyncUIWriter(out io.Writer, queueCapacity int) *asyncUIWriter {
	if queueCapacity <= 0 {
		queueCapacity = defaultUIQueueCapacity
	}
	w := &asyncUIWriter{
		out:   out,
		queue: make(chan string, queueCapacity),
	}
	w.wg.Add(1)
	go w.run()
	return w
}

func (w *asyncUIWriter) run() {
	defer w.wg.Done()
	for frame := range w.queue {
		if w.out == nil || frame == "" {
			continue
		}
		if _, err := io.WriteString(w.out, frame); err != nil {
			w.errOnce.Do(func() {
				w.err.Store(err.Error())
			})
		}
	}
}

func (w *asyncUIWriter) Enqueue(frame string) {
	if w == nil || frame == "" {
		return
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed || w.queue == nil {
		return
	}
	_, dropped := enqueueUIFrameDropOldest(w.queue, frame, &w.dropped)
	if dropped {
		// Intentionally silent to avoid recursive stdout/log pressure.
	}
}

func (w *asyncUIWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	queue := w.queue
	w.mu.Unlock()

	if queue != nil {
		close(queue)
	}
	w.wg.Wait()

	if raw, ok := w.err.Load().(string); ok && raw != "" {
		return fmt.Errorf("ui writer flush failed: %s", raw)
	}
	return nil
}

func (w *asyncUIWriter) DroppedCount() uint64 {
	if w == nil {
		return 0
	}
	return w.dropped.Load()
}

func enqueueUIFrameDropOldest(queue chan string, frame string, counter *atomic.Uint64) (enqueued bool, droppedOldest bool) {
	if queue == nil {
		return false, false
	}
	select {
	case queue <- frame:
		return true, false
	default:
	}
	select {
	case <-queue:
		droppedOldest = true
		if counter != nil {
			counter.Add(1)
		}
	default:
	}
	select {
	case queue <- frame:
		return true, droppedOldest
	default:
		return false, droppedOldest
	}
}
