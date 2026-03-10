package telemetrybus

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestMsgHandlerTrackerWaitForIdle(t *testing.T) {
	t.Parallel()

	tracker := NewMsgHandlerTracker()
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	handler := tracker.Wrap(func(*nats.Msg) {
		started <- struct{}{}
		<-release
	})

	go handler(&nats.Msg{})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handler start")
	}
	if tracker.WaitForIdle(10 * time.Millisecond) {
		t.Fatal("expected tracker to report busy before release")
	}
	close(release)
	if !tracker.WaitForIdle(time.Second) {
		t.Fatal("expected tracker to become idle after handler release")
	}
}

func TestMsgHandlerTrackerNilWaitForIdle(t *testing.T) {
	t.Parallel()

	var tracker *MsgHandlerTracker
	if !tracker.WaitForIdle(time.Second) {
		t.Fatal("nil tracker should be idle")
	}
}
