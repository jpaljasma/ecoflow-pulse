package main

import (
	"context"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

func TestReconnectAttemptStateBackoffAndReset(t *testing.T) {
	state := newReconnectAttemptState(500*time.Millisecond, 2*time.Second)
	if state.currentAttempt() != 1 {
		t.Fatalf("initial attempt mismatch: got=%d want=1", state.currentAttempt())
	}

	attempt1, wait1 := state.registerFailure(0.25)
	if attempt1 != 1 {
		t.Fatalf("attempt1 mismatch: got=%d want=1", attempt1)
	}
	if wait1 <= 0 {
		t.Fatalf("wait1 must be positive: got=%s", wait1)
	}
	if state.failureCount != 1 {
		t.Fatalf("failure count mismatch after first failure: got=%d want=1", state.failureCount)
	}
	if state.currentBackoff != 1*time.Second {
		t.Fatalf("backoff mismatch after first failure: got=%s want=1s", state.currentBackoff)
	}

	attempt2, wait2 := state.registerFailure(0.25)
	if attempt2 != 2 {
		t.Fatalf("attempt2 mismatch: got=%d want=2", attempt2)
	}
	if wait2 <= 0 {
		t.Fatalf("wait2 must be positive: got=%s", wait2)
	}
	if state.failureCount != 2 {
		t.Fatalf("failure count mismatch after second failure: got=%d want=2", state.failureCount)
	}
	if state.currentBackoff != 2*time.Second {
		t.Fatalf("backoff mismatch after second failure: got=%s want=2s", state.currentBackoff)
	}

	attempt3, _ := state.registerFailure(0.25)
	if attempt3 != 3 {
		t.Fatalf("attempt3 mismatch: got=%d want=3", attempt3)
	}
	if state.currentBackoff != 2*time.Second {
		t.Fatalf("backoff should clamp at max=2s: got=%s", state.currentBackoff)
	}

	state.reset()
	if state.failureCount != 0 {
		t.Fatalf("failure count should reset to zero: got=%d", state.failureCount)
	}
	if state.currentBackoff != 500*time.Millisecond {
		t.Fatalf("backoff should reset to initial: got=%s want=500ms", state.currentBackoff)
	}
	if state.currentAttempt() != 1 {
		t.Fatalf("attempt should reset to one after success: got=%d", state.currentAttempt())
	}
}

func TestEnqueueMQTTMessageDropOldest(t *testing.T) {
	ctx := context.Background()
	queue := make(chan ecoflowmqtt.Message, 2)
	stats := &mqttQueueStats{}

	msg1 := ecoflowmqtt.Message{Topic: "t", Payload: []byte("1")}
	msg2 := ecoflowmqtt.Message{Topic: "t", Payload: []byte("2")}
	msg3 := ecoflowmqtt.Message{Topic: "t", Payload: []byte("3")}

	if ok, dropped := enqueueMQTTMessageDropOldest(ctx, queue, msg1, stats); !ok || dropped {
		t.Fatalf("enqueue msg1 failed: ok=%v dropped=%v", ok, dropped)
	}
	if ok, dropped := enqueueMQTTMessageDropOldest(ctx, queue, msg2, stats); !ok || dropped {
		t.Fatalf("enqueue msg2 failed: ok=%v dropped=%v", ok, dropped)
	}
	if ok, dropped := enqueueMQTTMessageDropOldest(ctx, queue, msg3, stats); !ok || !dropped {
		t.Fatalf("enqueue msg3 should drop oldest: ok=%v dropped=%v", ok, dropped)
	}

	if got := stats.droppedOldest.Load(); got != 1 {
		t.Fatalf("drop count mismatch: got=%d want=1", got)
	}
	if got := len(queue); got != 2 {
		t.Fatalf("queue depth mismatch: got=%d want=2", got)
	}

	out1 := <-queue
	out2 := <-queue
	if string(out1.Payload) != "2" || string(out2.Payload) != "3" {
		t.Fatalf("queue order mismatch after drop-oldest: got=[%s,%s] want=[2,3]", string(out1.Payload), string(out2.Payload))
	}
}
