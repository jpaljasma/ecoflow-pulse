package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type telemetryTestStream struct {
	ctx          context.Context
	cancel       context.CancelFunc
	cancelOnSend int
	sendErr      error

	mu   sync.Mutex
	sent []*telemetryv1.SubscribeResponse
}

func (s *telemetryTestStream) Send(msg *telemetryv1.SubscribeResponse) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.mu.Lock()
	s.sent = append(s.sent, msg)
	n := len(s.sent)
	s.mu.Unlock()
	if s.cancelOnSend > 0 && n >= s.cancelOnSend && s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *telemetryTestStream) SetHeader(metadata.MD) error  { return nil }
func (s *telemetryTestStream) SendHeader(metadata.MD) error { return nil }
func (s *telemetryTestStream) SetTrailer(metadata.MD)       {}
func (s *telemetryTestStream) Context() context.Context     { return s.ctx }
func (s *telemetryTestStream) SendMsg(any) error            { return nil }
func (s *telemetryTestStream) RecvMsg(any) error            { return nil }

func newTestService() *TelemetryService {
	return NewTelemetryService(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestGetSnapshotValidation(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	_, err := svc.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestGetSnapshotOK(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	resp, err := svc.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{DeviceId: "dev-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSnapshot().GetDeviceId() != "dev-1" {
		t.Fatalf("unexpected device id: %q", resp.GetSnapshot().GetDeviceId())
	}
	if _, ok := resp.GetSnapshot().GetMetrics()["soc"]; !ok {
		t.Fatalf("expected soc metric")
	}
}

func TestGetSnapshotUsesProjectionReadModelWhenAvailable(t *testing.T) {
	t.Parallel()

	svc := NewTelemetryServiceWithSnapshotReader(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		fakeSnapshotReader{
			snapshot: &projectionworker.SnapshotReadModel{
				DeviceID: "dev-proj",
				Cursor: projectionworker.SnapshotCursor{
					Seq:      55,
					TsUnixMs: 9999,
				},
				Metrics: map[string]float64{
					"soc":      22.5,
					"watts_in": 180,
				},
			},
		},
	)

	resp, err := svc.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{DeviceId: "dev-proj"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.GetSnapshot().GetCursor().GetSeq(); got != 55 {
		t.Fatalf("cursor seq mismatch: got=%d want=55", got)
	}
	if got := resp.GetSnapshot().GetMetrics()["watts_in"]; got != 180 {
		t.Fatalf("metric mismatch: got=%v want=180", got)
	}
}

func TestGetSnapshotProjectionReadError(t *testing.T) {
	t.Parallel()

	svc := NewTelemetryServiceWithSnapshotReader(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		fakeSnapshotReader{err: errors.New("valkey down")},
	)
	_, err := svc.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{DeviceId: "dev-1"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
}

func TestSubscribeValidation(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &telemetryTestStream{ctx: ctx, cancel: cancel}

	err := svc.Subscribe(&telemetryv1.SubscribeRequest{}, stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestSubscribeSendsInitialSnapshotFirst(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &telemetryTestStream{ctx: ctx, cancel: cancel, cancelOnSend: 1}

	err := svc.Subscribe(&telemetryv1.SubscribeRequest{
		DeviceId:               "dev-1",
		IncludeInitialSnapshot: true,
	}, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(stream.sent))
	}
	if stream.sent[0].GetSnapshot() == nil {
		t.Fatalf("expected first payload to be snapshot")
	}
}

func TestSubscribeSendsHeartbeat(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &telemetryTestStream{ctx: ctx, cancel: cancel, cancelOnSend: 1}

	done := make(chan error, 1)
	go func() {
		done <- svc.Subscribe(&telemetryv1.SubscribeRequest{
			DeviceId:               "dev-1",
			IncludeInitialSnapshot: false,
		}, stream)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for heartbeat")
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(stream.sent))
	}
	hb := stream.sent[0].GetHeartbeat()
	if hb == nil {
		t.Fatalf("expected heartbeat payload")
	}
	if hb.GetCursor().GetSeq() < 2 {
		t.Fatalf("expected heartbeat sequence >= 2, got %d", hb.GetCursor().GetSeq())
	}
}

func TestSubscribePropagatesSendError(t *testing.T) {
	t.Parallel()

	svc := newTestService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &telemetryTestStream{ctx: ctx, cancel: cancel, sendErr: errors.New("send failed")}

	err := svc.Subscribe(&telemetryv1.SubscribeRequest{
		DeviceId:               "dev-1",
		IncludeInitialSnapshot: true,
	}, stream)
	if err == nil || err.Error() != "send failed" {
		t.Fatalf("expected send error, got %v", err)
	}
}

type fakeSnapshotReader struct {
	snapshot *projectionworker.SnapshotReadModel
	err      error
}

func (f fakeSnapshotReader) ReadSnapshot(context.Context, projectionworker.SnapshotIdentity) (*projectionworker.SnapshotReadModel, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}
