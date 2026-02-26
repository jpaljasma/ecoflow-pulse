package main

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type asyncChunkQueue struct {
	mu      sync.RWMutex
	queue   chan []byte
	writeFn func([]byte) error
	wg      sync.WaitGroup
	closed  bool

	pending atomic.Int64
	dropped atomic.Uint64
	errOnce sync.Once
	err     atomic.Value
}

func newAsyncChunkQueue(queueCapacity int, writeFn func([]byte) error) *asyncChunkQueue {
	if queueCapacity <= 0 {
		queueCapacity = 1
	}
	q := &asyncChunkQueue{
		queue:   make(chan []byte, queueCapacity),
		writeFn: writeFn,
	}
	q.wg.Add(1)
	go q.run()
	return q
}

func (q *asyncChunkQueue) run() {
	defer q.wg.Done()
	for payload := range q.queue {
		if len(payload) > 0 && q.writeFn != nil {
			if err := q.writeFn(payload); err != nil {
				q.errOnce.Do(func() {
					q.err.Store(err.Error())
				})
			}
		}
		q.pending.Add(-1)
	}
}

func (q *asyncChunkQueue) EnqueueDropOldest(payload []byte) (enqueued bool, droppedOldest bool) {
	if q == nil || len(payload) == 0 {
		return false, false
	}
	q.mu.RLock()
	if q.closed || q.queue == nil {
		q.mu.RUnlock()
		return false, false
	}
	queue := q.queue
	q.mu.RUnlock()

	select {
	case queue <- payload:
		q.pending.Add(1)
		return true, false
	default:
	}

	select {
	case <-queue:
		q.pending.Add(-1)
		q.dropped.Add(1)
		droppedOldest = true
	default:
	}

	select {
	case queue <- payload:
		q.pending.Add(1)
		return true, droppedOldest
	default:
		return false, droppedOldest
	}
}

func (q *asyncChunkQueue) Flush() error {
	if q == nil {
		return nil
	}
	for q.pending.Load() > 0 {
		time.Sleep(1 * time.Millisecond)
	}
	if raw, ok := q.err.Load().(string); ok && raw != "" {
		return fmt.Errorf("async chunk queue write failed: %s", raw)
	}
	return nil
}

func (q *asyncChunkQueue) Close() error {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	queue := q.queue
	q.mu.Unlock()

	if queue != nil {
		close(queue)
	}
	q.wg.Wait()
	if raw, ok := q.err.Load().(string); ok && raw != "" {
		return errors.New(raw)
	}
	return nil
}

func (q *asyncChunkQueue) DroppedCount() uint64 {
	if q == nil {
		return 0
	}
	return q.dropped.Load()
}
