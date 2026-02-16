package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

type gateWriter struct {
	mu      sync.Mutex
	started chan struct{}
	gate    chan struct{}
	parts   []string
}

func newGateWriter() *gateWriter {
	return &gateWriter{
		started: make(chan struct{}),
		gate:    make(chan struct{}),
		parts:   make([]string, 0, 8),
	}
}

func (w *gateWriter) Write(p []byte) (int, error) {
	select {
	case <-w.started:
	default:
		close(w.started)
	}
	<-w.gate
	w.mu.Lock()
	w.parts = append(w.parts, string(p))
	w.mu.Unlock()
	return len(p), nil
}

func (w *gateWriter) Joined() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.parts, "")
}

func TestAsyncUIWriterCloseFlushesQueue(t *testing.T) {
	var out bytes.Buffer
	writer := newAsyncUIWriter(&out, 8)
	writer.Enqueue("frame-1\n")
	writer.Enqueue("frame-2\n")
	writer.Enqueue("frame-3\n")
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "frame-1\n") || !strings.Contains(got, "frame-2\n") || !strings.Contains(got, "frame-3\n") {
		t.Fatalf("unexpected flushed output: %q", got)
	}
	if writer.DroppedCount() != 0 {
		t.Fatalf("expected no drops, got %d", writer.DroppedCount())
	}
}

func TestAsyncUIWriterDropsOldestWhenQueueFull(t *testing.T) {
	gw := newGateWriter()
	writer := newAsyncUIWriter(gw, 2)

	writer.Enqueue("frame-a\n")

	select {
	case <-gw.started:
	case <-time.After(1 * time.Second):
		t.Fatal("writer did not start")
	}

	writer.Enqueue("frame-b\n")
	writer.Enqueue("frame-c\n")
	writer.Enqueue("frame-d\n")

	close(gw.gate)
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	got := gw.Joined()
	if !strings.Contains(got, "frame-a\n") {
		t.Fatalf("expected first frame to be written, got %q", got)
	}
	if !strings.Contains(got, "frame-c\n") || !strings.Contains(got, "frame-d\n") {
		t.Fatalf("expected newest frames to be retained, got %q", got)
	}
	if strings.Contains(got, "frame-b\n") {
		t.Fatalf("expected oldest queued frame to be dropped, got %q", got)
	}
	if writer.DroppedCount() == 0 {
		t.Fatalf("expected dropped frames count > 0")
	}
}
