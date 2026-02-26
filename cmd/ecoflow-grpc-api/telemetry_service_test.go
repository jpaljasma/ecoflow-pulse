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

func TestSubscribeReadModelSendsDeltaOnMetricChange(t *testing.T) {
	t.Parallel()

	reader := &sequenceSnapshotReader{
		snapshots: []*projectionworker.SnapshotReadModel{
			{
				DeviceID: "dev-1",
				Cursor: projectionworker.SnapshotCursor{
					Seq:      10,
					TsUnixMs: 1000,
				},
				Metrics: map[string]float64{
					"soc":      25,
					"watts_in": 100,
				},
			},
			{
				DeviceID: "dev-1",
				Cursor: projectionworker.SnapshotCursor{
					Seq:      11,
					TsUnixMs: 1100,
				},
				Metrics: map[string]float64{
					"soc":      25,
					"watts_in": 140,
				},
			},
		},
	}
	svc := NewTelemetryServiceWithSnapshotReader(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		reader,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &telemetryTestStream{ctx: ctx, cancel: cancel, cancelOnSend: 2}

	err := svc.Subscribe(&telemetryv1.SubscribeRequest{
		DeviceId:               "dev-1",
		IncludeInitialSnapshot: true,
		MaxUpdateHz:            50,
	}, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.sent) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(stream.sent))
	}
	if stream.sent[0].GetSnapshot() == nil {
		t.Fatalf("expected first payload to be snapshot")
	}
	delta := stream.sent[1].GetDelta()
	if delta == nil {
		t.Fatalf("expected second payload to be delta")
	}
	if got := delta.GetCursor().GetSeq(); got != 11 {
		t.Fatalf("delta cursor seq mismatch: got=%d want=11", got)
	}
	if got := delta.GetChanged()["watts_in"]; got != 140 {
		t.Fatalf("delta changed metric mismatch: got=%v want=140", got)
	}
}

func TestSubscribeReadModelSendsHeartbeatWhenUnchanged(t *testing.T) {
	t.Parallel()

	reader := &sequenceSnapshotReader{
		snapshots: []*projectionworker.SnapshotReadModel{
			{
				DeviceID: "dev-1",
				Cursor: projectionworker.SnapshotCursor{
					Seq:      20,
					TsUnixMs: 2000,
				},
				Metrics: map[string]float64{"soc": 55},
			},
			{
				DeviceID: "dev-1",
				Cursor: projectionworker.SnapshotCursor{
					Seq:      20,
					TsUnixMs: 2000,
				},
				Metrics: map[string]float64{"soc": 55},
			},
		},
	}
	svc := NewTelemetryServiceWithSnapshotReader(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		reader,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &telemetryTestStream{ctx: ctx, cancel: cancel, cancelOnSend: 2}

	err := svc.Subscribe(&telemetryv1.SubscribeRequest{
		DeviceId:               "dev-1",
		IncludeInitialSnapshot: true,
		MaxUpdateHz:            50,
	}, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.sent) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(stream.sent))
	}
	if stream.sent[1].GetHeartbeat() == nil {
		t.Fatalf("expected second payload to be heartbeat")
	}
}

func TestSubscribeReadModelContinuesOnReadError(t *testing.T) {
	t.Parallel()

	reader := &sequenceSnapshotReader{
		errAtCall: map[int]error{
			1: errors.New("valkey unavailable"),
		},
		snapshots: []*projectionworker.SnapshotReadModel{
			{
				DeviceID: "dev-1",
				Cursor: projectionworker.SnapshotCursor{
					Seq:      30,
					TsUnixMs: 3000,
				},
				Metrics: map[string]float64{"soc": 65},
			},
		},
	}
	svc := NewTelemetryServiceWithSnapshotReader(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		reader,
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &telemetryTestStream{ctx: ctx, cancel: cancel, cancelOnSend: 1}

	err := svc.Subscribe(&telemetryv1.SubscribeRequest{
		DeviceId:               "dev-1",
		IncludeInitialSnapshot: false,
		MaxUpdateHz:            50,
	}, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(stream.sent))
	}
	if stream.sent[0].GetHeartbeat() == nil {
		t.Fatalf("expected heartbeat on snapshot read error")
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

type sequenceSnapshotReader struct {
	mu        sync.Mutex
	snapshots []*projectionworker.SnapshotReadModel
	errAtCall map[int]error
	idx       int
}

func (s *sequenceSnapshotReader) ReadSnapshot(context.Context, projectionworker.SnapshotIdentity) (*projectionworker.SnapshotReadModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idx++
	if err := s.errAtCall[s.idx]; err != nil {
		return nil, err
	}
	if len(s.snapshots) == 0 {
		return nil, nil
	}
	if s.idx-1 >= len(s.snapshots) {
		return s.snapshots[len(s.snapshots)-1], nil
	}
	return s.snapshots[s.idx-1], nil
}
