package main

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestAsyncChunkQueueFlushAndClose(t *testing.T) {
	var mu sync.Mutex
	var out bytes.Buffer
	q := newAsyncChunkQueue(8, func(payload []byte) error {
		mu.Lock()
		defer mu.Unlock()
		_, err := out.Write(payload)
		return err
	})

	enqueued, _ := q.EnqueueDropOldest([]byte("a\n"))
	if !enqueued {
		t.Fatal("expected first enqueue")
	}
	enqueued, _ = q.EnqueueDropOldest([]byte("b\n"))
	if !enqueued {
		t.Fatal("expected second enqueue")
	}

	if err := q.Flush(); err != nil {
		t.Fatalf("flush queue: %v", err)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("close queue: %v", err)
	}

	mu.Lock()
	got := out.String()
	mu.Unlock()
	if got != "a\nb\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestAsyncChunkQueueDropsOldest(t *testing.T) {
	block := make(chan struct{})
	var mu sync.Mutex
	var written []string
	q := newAsyncChunkQueue(2, func(payload []byte) error {
		<-block
		mu.Lock()
		written = append(written, string(payload))
		mu.Unlock()
		return nil
	})

	if ok, _ := q.EnqueueDropOldest([]byte("one")); !ok {
		t.Fatal("enqueue one failed")
	}
	time.Sleep(10 * time.Millisecond)
	if ok, _ := q.EnqueueDropOldest([]byte("two")); !ok {
		t.Fatal("enqueue two failed")
	}
	if ok, _ := q.EnqueueDropOldest([]byte("three")); !ok {
		t.Fatal("enqueue three failed")
	}
	if ok, _ := q.EnqueueDropOldest([]byte("four")); !ok {
		t.Fatal("enqueue four failed")
	}

	close(block)
	if err := q.Close(); err != nil {
		t.Fatalf("close queue: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), written...)
	mu.Unlock()
	if len(got) < 2 {
		t.Fatalf("expected writes, got=%v", got)
	}
	// "two" should be dropped first once queue pressure occurs.
	for _, item := range got {
		if item == "two" {
			t.Fatalf("expected oldest queued payload to be dropped, got=%v", got)
		}
	}
	if q.DroppedCount() == 0 {
		t.Fatalf("expected dropped count > 0")
	}
}
