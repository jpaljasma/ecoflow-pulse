package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnqueueLoggerChunkDropOldest(t *testing.T) {
	queue := make(chan []byte, 2)
	if ok, dropped := enqueueLoggerChunkDropOldest(queue, []byte("a"), nil); !ok || dropped {
		t.Fatalf("enqueue a failed: ok=%t dropped=%t", ok, dropped)
	}
	if ok, dropped := enqueueLoggerChunkDropOldest(queue, []byte("b"), nil); !ok || dropped {
		t.Fatalf("enqueue b failed: ok=%t dropped=%t", ok, dropped)
	}
	if ok, dropped := enqueueLoggerChunkDropOldest(queue, []byte("c"), nil); !ok || !dropped {
		t.Fatalf("enqueue c should drop oldest: ok=%t dropped=%t", ok, dropped)
	}

	got1 := string(<-queue)
	got2 := string(<-queue)
	if got1 != "b" || got2 != "c" {
		t.Fatalf("queue order mismatch: got=(%q,%q) want=(%q,%q)", got1, got2, "b", "c")
	}
}

func TestMQTTOutputLoggerFlushesOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mqtt.log")
	logger, err := newMQTTOutputLogger(path, 128)
	if err != nil {
		t.Fatalf("newMQTTOutputLogger: %v", err)
	}

	const total = 40
	for i := 0; i < total; i++ {
		logger.Printf("line=%d", i)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close mqtt logger: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mqtt log: %v", err)
	}
	content := string(raw)
	for i := 0; i < total; i++ {
		token := fmt.Sprintf("line=%d", i)
		if !strings.Contains(content, token) {
			t.Fatalf("missing expected token %q in mqtt log", token)
		}
	}
}
