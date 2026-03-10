package telemetrybus

import (
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
)

type MsgHandlerTracker struct {
	active atomic.Int64
	idle   chan struct{}
}

func NewMsgHandlerTracker() *MsgHandlerTracker {
	return &MsgHandlerTracker{
		idle: make(chan struct{}, 1),
	}
}

func (t *MsgHandlerTracker) Wrap(handler nats.MsgHandler) nats.MsgHandler {
	if handler == nil {
		return nil
	}
	if t == nil {
		return handler
	}
	return func(msg *nats.Msg) {
		t.active.Add(1)
		defer func() {
			if t.active.Add(-1) == 0 {
				select {
				case t.idle <- struct{}{}:
				default:
				}
			}
		}()
		handler(msg)
	}
}

func (t *MsgHandlerTracker) WaitForIdle(timeout time.Duration) bool {
	if t == nil {
		return true
	}
	if t.active.Load() == 0 {
		return true
	}
	if timeout <= 0 {
		return false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if t.active.Load() == 0 {
			return true
		}
		select {
		case <-t.idle:
		case <-timer.C:
			return t.active.Load() == 0
		}
	}
}
