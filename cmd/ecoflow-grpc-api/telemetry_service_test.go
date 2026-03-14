package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
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

func newTestEnergyService() *EnergyService {
	return NewEnergyService(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func numberPtr(value float64) *float64 {
	return &value
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

func TestGetSnapshotFallbackMetricsAreClonedPerRequest(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	first, err := svc.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{DeviceId: "dev-1"})
	if err != nil {
		t.Fatalf("first GetSnapshot() error = %v", err)
	}
	first.GetSnapshot().GetMetrics()["soc"] = 99
	first.GetSnapshot().GetMetrics()["custom"] = 123

	second, err := svc.GetSnapshot(context.Background(), &telemetryv1.GetSnapshotRequest{DeviceId: "dev-1"})
	if err != nil {
		t.Fatalf("second GetSnapshot() error = %v", err)
	}
	if got := second.GetSnapshot().GetMetrics()["soc"]; got != defaultSnapshotMetrics["soc"] {
		t.Fatalf("expected fallback soc=%v, got=%v", defaultSnapshotMetrics["soc"], got)
	}
	if _, ok := second.GetSnapshot().GetMetrics()["custom"]; ok {
		t.Fatal("expected fallback metrics map to be isolated per request")
	}
}

func TestShouldCompressHistoryResponse(t *testing.T) {
	t.Parallel()

	if shouldCompressHistoryResponse(nil, defaultHistoryGzipMinBytes) {
		t.Fatalf("expected nil message to skip compression")
	}
	small := &telemetryv1.QueryRollupRangeResponse{
		Series: &telemetryv1.RollupSeries{
			DeviceId: "dev-1",
			Points: []*telemetryv1.RollupPoint{
				{
					BucketStartUnixMs: 1,
					Metrics: &telemetryv1.RollupMetrics{
						PvAvgW: numberPtr(10),
					},
				},
			},
		},
	}
	if shouldCompressHistoryResponse(small, defaultHistoryGzipMinBytes) {
		t.Fatalf("expected small response to skip compression")
	}
	largePoints := make([]*telemetryv1.RollupPoint, 0, 512)
	for i := 0; i < 512; i++ {
		largePoints = append(largePoints, &telemetryv1.RollupPoint{
			BucketStartUnixMs: int64(i),
			Metrics: &telemetryv1.RollupMetrics{
				SocAvgPct:        numberPtr(50),
				AcInAvgW:         numberPtr(100),
				PvAvgW:           numberPtr(200),
				LoadAvgW:         numberPtr(150),
				SolarGeneratedWh: numberPtr(3),
			},
		})
	}
	large := &telemetryv1.QueryRollupRangeResponse{
		Series: &telemetryv1.RollupSeries{
			DeviceId: "dev-1",
			Points:   largePoints,
		},
	}
	if !shouldCompressHistoryResponse(large, defaultHistoryGzipMinBytes) {
		t.Fatalf("expected large response to enable compression")
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

func TestMergeMetricsIncludesACOutputFields(t *testing.T) {
	t.Parallel()

	left := telemetryquery.Metrics{
		ACOutputAvgW:     numberPtr(0),
		ACOutputMaxW:     numberPtr(0),
		ACOutputEnergyWh: numberPtr(0),
	}
	right := telemetryquery.Metrics{
		ACOutputAvgW:     numberPtr(143),
		ACOutputMaxW:     numberPtr(166),
		ACOutputEnergyWh: numberPtr(143),
	}

	merged := mergeMetrics(left, right)

	if merged.ACOutputAvgW == nil || *merged.ACOutputAvgW != 143 {
		t.Fatalf("ACOutputAvgW mismatch: got=%v want=143", merged.ACOutputAvgW)
	}
	if merged.ACOutputMaxW == nil || *merged.ACOutputMaxW != 166 {
		t.Fatalf("ACOutputMaxW mismatch: got=%v want=166", merged.ACOutputMaxW)
	}
	if merged.ACOutputEnergyWh == nil || *merged.ACOutputEnergyWh != 143 {
		t.Fatalf("ACOutputEnergyWh mismatch: got=%v want=143", merged.ACOutputEnergyWh)
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

func TestGetSnapshotPermissionDenied(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f50"
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"owner": {{DeviceID: deviceID, EcoflowSN: "DEMOD2M00001057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
	})
	svc := NewTelemetryServiceWithDeps(TelemetryServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
	})

	_, err := svc.GetSnapshot(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "other-user"}), &telemetryv1.GetSnapshotRequest{
		DeviceId: deviceID,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestSubscribePermissionDenied(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f51"
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"owner": {{DeviceID: deviceID, EcoflowSN: "DEMODPU0000294", ProductName: "DPU A 12 kWh", Model: "DELTA Pro Ultra", Role: "admin"}},
	})
	svc := NewTelemetryServiceWithDeps(TelemetryServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
	})
	ctx, cancel := context.WithCancel(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "other-user"}))
	defer cancel()
	stream := &telemetryTestStream{ctx: ctx, cancel: cancel}

	err := svc.Subscribe(&telemetryv1.SubscribeRequest{
		DeviceId:               deviceID,
		IncludeInitialSnapshot: true,
	}, stream)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestQueryRollupRangeValidation(t *testing.T) {
	t.Parallel()

	svc := newTestEnergyService()
	_, err := svc.QueryRollupRange(context.Background(), &telemetryv1.QueryRollupRangeRequest{
		DeviceId:   "not-a-uuid",
		Resolution: telemetryv1.RollupResolution_ROLLUP_RESOLUTION_HOUR,
		FromUnixMs: time.Now().Add(-time.Hour).UnixMilli(),
		ToUnixMs:   time.Now().UnixMilli(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestQueryRollupRangeUnavailableWithoutReader(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52"
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "DEMOD2M00001057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
	})

	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
	})

	_, err := svc.QueryRollupRange(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}), &telemetryv1.QueryRollupRangeRequest{
		DeviceId:   deviceID,
		Resolution: telemetryv1.RollupResolution_ROLLUP_RESOLUTION_HOUR,
		FromUnixMs: time.Now().Add(-time.Hour).UnixMilli(),
		ToUnixMs:   time.Now().UnixMilli(),
	})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
}

func TestQueryRollupRangePermissionDenied(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f53"
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"owner": {{DeviceID: deviceID, EcoflowSN: "DEMODPU0000294", ProductName: "DPU A 12 kWh", Model: "DELTA Pro Ultra", Role: "admin"}},
	})

	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
		QueryReader:       &fakeQueryReader{},
	})

	_, err := svc.QueryRollupRange(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "other-user"}), &telemetryv1.QueryRollupRangeRequest{
		DeviceId:   deviceID,
		Resolution: telemetryv1.RollupResolution_ROLLUP_RESOLUTION_HOUR,
		FromUnixMs: time.Now().Add(-time.Hour).UnixMilli(),
		ToUnixMs:   time.Now().UnixMilli(),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestQueryRollupRangeReturnsSeries(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f54"
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "DEMOD2M00001057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
	})

	from := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Hour)
	pvAvg := 52.5
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionHour,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{
					{
						BucketStart:   from,
						BucketEnd:     from.Add(time.Hour),
						SampleCount:   60,
						FirstTsUnixMs: from.UnixMilli(),
						LastTsUnixMs:  from.Add(time.Hour - time.Minute).UnixMilli(),
						Metrics: telemetryquery.Metrics{
							PVAvgW: &pvAvg,
						},
					},
				},
			},
		},
	}

	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
		QueryReader:       reader,
	})

	resp, err := svc.QueryRollupRange(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}), &telemetryv1.QueryRollupRangeRequest{
		DeviceId:   deviceID,
		Resolution: telemetryv1.RollupResolution_ROLLUP_RESOLUTION_HOUR,
		FromUnixMs: from.UnixMilli(),
		ToUnixMs:   to.UnixMilli(),
	})
	if err != nil {
		t.Fatalf("QueryRollupRange failed: %v", err)
	}
	if got := len(resp.GetSeries().GetPoints()); got != 1 {
		t.Fatalf("points mismatch: got=%d want=1", got)
	}
	if got := resp.GetSeries().GetPoints()[0].GetMetrics().GetPvAvgW(); got != pvAvg {
		t.Fatalf("pv_avg_w mismatch: got=%v want=%v", got, pvAvg)
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if got := len(reader.queries); got != 1 {
		t.Fatalf("query count mismatch: got=%d want=1", got)
	}
	if got := reader.queries[0].Resolution; got != telemetryquery.ResolutionHour {
		t.Fatalf("resolution mismatch: got=%v want=%v", got, telemetryquery.ResolutionHour)
	}
}

func TestCompareRollupRangeUsesPreviousPeriod(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f55"
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "DEMOD2M00001057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
	})

	from := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Hour)
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{
			{DeviceID: deviceID, Resolution: telemetryquery.ResolutionHour, From: from, To: to},
			{DeviceID: deviceID, Resolution: telemetryquery.ResolutionHour, From: from.Add(-2 * time.Hour), To: from},
		},
	}
	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
		QueryReader:       reader,
	})

	resp, err := svc.CompareRollupRange(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}), &telemetryv1.CompareRollupRangeRequest{
		DeviceId:          deviceID,
		Resolution:        telemetryv1.RollupResolution_ROLLUP_RESOLUTION_HOUR,
		FromUnixMs:        from.UnixMilli(),
		ToUnixMs:          to.UnixMilli(),
		UsePreviousPeriod: true,
	})
	if err != nil {
		t.Fatalf("CompareRollupRange failed: %v", err)
	}
	if resp.GetPrevious().GetFromUnixMs() != from.Add(-2*time.Hour).UnixMilli() {
		t.Fatalf("previous from mismatch: got=%d", resp.GetPrevious().GetFromUnixMs())
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if got := len(reader.queries); got != 2 {
		t.Fatalf("query count mismatch: got=%d want=2", got)
	}
	var foundPrevious bool
	for _, query := range reader.queries {
		if query.From.Equal(from.Add(-2*time.Hour)) && query.To.Equal(from) {
			foundPrevious = true
			break
		}
	}
	if !foundPrevious {
		t.Fatalf("expected previous window query [%s,%s), got=%v", from.Add(-2*time.Hour), from, reader.queries)
	}
}

func TestQueryRollupRangeLogsEnergyFallbackUsage(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f5a"
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "DEMOD2M00001057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
	})

	from := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{{
			DeviceID:   deviceID,
			Resolution: telemetryquery.ResolutionHour,
			From:       from,
			To:         to,
			EnergyBucketCoverage: telemetryquery.EnergyBucketCoverage{
				PointCount:          1,
				PersistedValueCount: 2,
				DerivedValueCount:   3,
				DerivedPointCount:   1,
			},
		}},
	}
	var logs bytes.Buffer
	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(&logs, nil)),
		ControlPlaneStore: store,
		QueryReader:       reader,
	})

	_, err := svc.QueryRollupRange(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}), &telemetryv1.QueryRollupRangeRequest{
		DeviceId:   deviceID,
		Resolution: telemetryv1.RollupResolution_ROLLUP_RESOLUTION_HOUR,
		FromUnixMs: from.UnixMilli(),
		ToUnixMs:   to.UnixMilli(),
	})
	if err != nil {
		t.Fatalf("QueryRollupRange failed: %v", err)
	}
	logText := logs.String()
	if !strings.Contains(logText, "telemetry history used derived energy fallback") {
		t.Fatalf("expected fallback log, got %q", logText)
	}
	if !strings.Contains(logText, "derived_values=3") {
		t.Fatalf("expected derived values count in log, got %q", logText)
	}
}

func TestGetEnergyDashboardReturnsSingleDeviceSummary(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f56"
	now := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "DEMOD2M00001057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
	})
	from := time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC)
	to := now
	previousFrom := from.AddDate(0, 0, -1)
	previousTo := from
	currentPV := 400.0
	currentLoad := 300.0
	currentACIn := 90.0
	currentBattery := -40.0
	currentLoadEnergy := 3600.0
	currentACInputEnergy := 1080.0
	currentBatteryDischargeEnergy := 480.0
	currentMinutePV := 320.0
	previousPV := 200.0
	previousLoad := 250.0
	previousACIn := 100.0
	previousLoadEnergy := 3000.0
	previousACInputEnergy := 1200.0
	previousMinutePV := 180.0
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionHour,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{{
					BucketStart: from,
					BucketEnd:   to,
					Metrics: telemetryquery.Metrics{
						SolarGeneratedWh:         &currentPV,
						LoadAvgW:                 &currentLoad,
						ACInAvgW:                 &currentACIn,
						BatteryAvgW:              &currentBattery,
						LoadEnergyWh:             &currentLoadEnergy,
						ACInputEnergyWh:          &currentACInputEnergy,
						BatteryDischargeEnergyWh: &currentBatteryDischargeEnergy,
					},
				}},
			},
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionHour,
				From:       previousFrom,
				To:         previousTo,
				Points: []telemetryquery.Point{{
					BucketStart: previousFrom,
					BucketEnd:   previousTo,
					Metrics: telemetryquery.Metrics{
						SolarGeneratedWh: &previousPV,
						LoadAvgW:         &previousLoad,
						ACInAvgW:         &previousACIn,
						LoadEnergyWh:     &previousLoadEnergy,
						ACInputEnergyWh:  &previousACInputEnergy,
					},
				}},
			},
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionFiveMinutes,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{{
					BucketStart: from,
					BucketEnd:   from.Add(5 * time.Minute),
					Metrics: telemetryquery.Metrics{
						PVAvgW:      &currentMinutePV,
						LoadAvgW:    &currentLoad,
						ACInAvgW:    &currentACIn,
						BatteryAvgW: &currentBattery,
					},
				}},
			},
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionFiveMinutes,
				From:       previousFrom,
				To:         previousTo,
				Points: []telemetryquery.Point{{
					BucketStart: previousFrom,
					BucketEnd:   previousFrom.Add(5 * time.Minute),
					Metrics: telemetryquery.Metrics{
						PVAvgW:   &previousMinutePV,
						LoadAvgW: &previousLoad,
						ACInAvgW: &previousACIn,
					},
				}},
			},
		},
	}
	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
		QueryReader:       reader,
		Now:               func() time.Time { return now },
	})

	resp, err := svc.GetEnergyDashboard(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}), &telemetryv1.GetEnergyDashboardRequest{
		DeviceId:          deviceID,
		Preset:            "today",
		Timezone:          "UTC",
		IncludeComparison: true,
		GridPricePerKwh:   0.30,
		Currency:          "USD",
	})
	if err != nil {
		t.Fatalf("GetEnergyDashboard failed: %v", err)
	}
	if got := resp.GetScope().GetMode(); got != "single" {
		t.Fatalf("scope mode mismatch: got=%s want=single", got)
	}
	if got := resp.GetSummary().GetEstimatedValue().GetCurrent(); got <= 0 {
		t.Fatalf("expected estimated value > 0, got=%v", got)
	}
	if got := resp.GetBattery().GetNetKwh(); got >= 0 {
		t.Fatalf("expected negative battery net kwh, got=%v", got)
	}
	if got := len(resp.GetCurrentPowerPoints()); got != 1 {
		t.Fatalf("current power point count mismatch: got=%d want=1", got)
	}
	if got := len(resp.GetCurrentEnergyPoints()); got != 1 {
		t.Fatalf("current energy point count mismatch: got=%d want=1", got)
	}
	if got := resp.GetCurrentEnergyPoints()[0].GetMetrics().GetLoadEnergyWh(); got != currentLoadEnergy {
		t.Fatalf("current energy load mismatch: got=%v want=%v", got, currentLoadEnergy)
	}
	if got := resp.GetCurrentPowerPoints()[0].GetMetrics().GetPvAvgW(); got != currentMinutePV {
		t.Fatalf("current power pv mismatch: got=%v want=%v", got, currentMinutePV)
	}
	if got := len(resp.GetPreviousPowerPoints()); got != 1 {
		t.Fatalf("previous power point count mismatch: got=%d want=1", got)
	}
	if got := len(resp.GetPreviousEnergyPoints()); got != 1 {
		t.Fatalf("previous energy point count mismatch: got=%d want=1", got)
	}
}

func TestGetEnergyDashboardUsesVisibleDevicesForAllScope(t *testing.T) {
	t.Parallel()

	deviceA := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f57"
	deviceB := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f58"
	now := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {
			{DeviceID: deviceA, EcoflowSN: "A", ProductName: "A", Model: "DELTA 2 Max", Role: "admin"},
			{DeviceID: deviceB, EcoflowSN: "B", ProductName: "B", Model: "DELTA 2 Max", Role: "admin"},
		},
	})
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	to := now
	previousFrom := from.AddDate(0, -1, 0)
	loadA := 200.0
	loadB := 300.0
	loadEnergyA := 4800.0
	loadEnergyB := 7200.0
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{
			{DeviceID: deviceA, Resolution: telemetryquery.ResolutionDay, From: from, To: to, Points: []telemetryquery.Point{{BucketStart: from, BucketEnd: to, Metrics: telemetryquery.Metrics{LoadAvgW: &loadA, LoadEnergyWh: &loadEnergyA}}}},
			{DeviceID: deviceB, Resolution: telemetryquery.ResolutionDay, From: from, To: to, Points: []telemetryquery.Point{{BucketStart: from, BucketEnd: to, Metrics: telemetryquery.Metrics{LoadAvgW: &loadB, LoadEnergyWh: &loadEnergyB}}}},
			{DeviceID: deviceA, Resolution: telemetryquery.ResolutionDay, From: previousFrom, To: from, Points: []telemetryquery.Point{}},
			{DeviceID: deviceB, Resolution: telemetryquery.ResolutionDay, From: previousFrom, To: from, Points: []telemetryquery.Point{}},
		},
	}
	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
		QueryReader:       reader,
		Now:               func() time.Time { return now },
	})

	resp, err := svc.GetEnergyDashboard(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}), &telemetryv1.GetEnergyDashboardRequest{
		UseAllDevices:     true,
		Preset:            "thisMonth",
		Timezone:          "UTC",
		IncludeComparison: true,
	})
	if err != nil {
		t.Fatalf("GetEnergyDashboard failed: %v", err)
	}
	if got := len(resp.GetScope().GetResolvedDeviceIds()); got != 2 {
		t.Fatalf("resolved device id count mismatch: got=%d want=2", got)
	}
	if got := resp.GetSummary().GetLoadConsumedKwh().GetCurrent(); got != 12 {
		t.Fatalf("aggregate load mismatch: got=%v want=12", got)
	}
	if got := len(resp.GetCurrentEnergyPoints()); got != 1 {
		t.Fatalf("aggregate current energy point count mismatch: got=%d want=1", got)
	}
	if got := resp.GetCurrentEnergyPoints()[0].GetMetrics().GetLoadEnergyWh(); got != loadEnergyA+loadEnergyB {
		t.Fatalf("aggregate current load energy mismatch: got=%v want=%v", got, loadEnergyA+loadEnergyB)
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if got := len(reader.aggregateQueries); got != 4 {
		t.Fatalf("aggregate query count mismatch: got=%d want=4", got)
	}
}

func TestGetEnergyDashboardSkipsPreviousQueriesWhenComparisonDisabled(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f59"
	now := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "DEMO", ProductName: "Garage", Model: "DELTA 2 Max", Role: "admin"}},
	})
	from := time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC)
	to := now
	currentPV := 400.0
	currentMinutePV := 320.0
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionHour,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{{
					BucketStart: from,
					BucketEnd:   to,
					Metrics: telemetryquery.Metrics{
						SolarGeneratedWh: &currentPV,
					},
				}},
			},
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionFiveMinutes,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{{
					BucketStart: from,
					BucketEnd:   from.Add(5 * time.Minute),
					Metrics: telemetryquery.Metrics{
						PVAvgW: &currentMinutePV,
					},
				}},
			},
		},
	}
	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
		QueryReader:       reader,
		Now:               func() time.Time { return now },
	})

	resp, err := svc.GetEnergyDashboard(grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}), &telemetryv1.GetEnergyDashboardRequest{
		DeviceId:          deviceID,
		Preset:            "today",
		Timezone:          "UTC",
		IncludeComparison: false,
	})
	if err != nil {
		t.Fatalf("GetEnergyDashboard failed: %v", err)
	}
	if got := len(resp.GetPreviousPowerPoints()); got != 0 {
		t.Fatalf("expected no previous power points, got=%d", got)
	}
	if got := len(resp.GetPreviousEnergyPoints()); got != 0 {
		t.Fatalf("expected no previous energy points, got=%d", got)
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if got := len(reader.queries); got != 2 {
		t.Fatalf("query count mismatch: got=%d want=2", got)
	}
}

func TestGetEnergyPvPortHistorySkipsMissingArchiveObjects(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f60"
	providerDeviceID := "PROVIDER-DEVICE-1"
	now := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "DEMO", ProductName: "Garage", Model: "DELTA 2 Max", Role: "admin"}},
	})
	store.providerDevices[deviceID] = controlplane.ProviderDevice{
		DeviceID:         deviceID,
		Provider:         controlplane.ProviderEcoFlow,
		ProviderDeviceID: providerDeviceID,
	}
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	to := now
	previousFrom := from.AddDate(0, -1, 0)
	solarGenerated := 2400.0
	pvAvg := 120.0
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionDay,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{{
					BucketStart: from,
					BucketEnd:   to,
					Metrics: telemetryquery.Metrics{
						SolarGeneratedWh: &solarGenerated,
					},
				}},
			},
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionFiveMinutes,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{{
					BucketStart: from,
					BucketEnd:   from.Add(5 * time.Minute),
					Metrics: telemetryquery.Metrics{
						PVAvgW: &pvAvg,
					},
				}},
			},
			{DeviceID: deviceID, Resolution: telemetryquery.ResolutionDay, From: previousFrom, To: from, Points: []telemetryquery.Point{}},
			{DeviceID: deviceID, Resolution: telemetryquery.ResolutionFiveMinutes, From: previousFrom, To: from, Points: []telemetryquery.Point{}},
		},
	}
	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
		QueryReader:       reader,
		Now:               func() time.Time { return now },
		ArchiveManifestStore: fakeManifestStore{
			objects: []replaycli.ManifestObject{{
				Provider:          controlplane.ProviderEcoFlow,
				ObjectBucket:      "pulse-telemetry-raw",
				ObjectKey:         "missing.pb.zst",
				ProviderDeviceIDs: []string{providerDeviceID},
			}},
		},
		ArchiveObjectReader: fakeObjectReader{
			read: func(bucket, key string) ([]byte, error) {
				return nil, errors.New("read object " + bucket + "/" + key + ": The specified key does not exist.")
			},
		},
	})

	resp, err := svc.GetEnergyPvPortHistory(
		grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}),
		&telemetryv1.GetEnergyPvPortHistoryRequest{
			UseAllDevices: true,
			Preset:        "thisMonth",
			Timezone:      "UTC",
		},
	)
	if err != nil {
		t.Fatalf("GetEnergyPvPortHistory failed: %v", err)
	}
	if got := len(resp.GetPvPortHistory()); got != 0 {
		t.Fatalf("expected missing archive object to be skipped, got pv history rows=%d", got)
	}
}

func TestGetEnergyDashboardLeavesPVHistoryForLazyLoad(t *testing.T) {
	t.Parallel()

	deviceID := "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f60"
	now := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "DEMO", ProductName: "Garage", Model: "DELTA 2 Max", Role: "admin"}},
	})
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	to := now
	previousFrom := from.AddDate(0, -1, 0)
	solarGenerated := 2400.0
	pvAvg := 120.0
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionDay,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{{
					BucketStart: from,
					BucketEnd:   to,
					Metrics: telemetryquery.Metrics{
						SolarGeneratedWh: &solarGenerated,
					},
				}},
			},
			{
				DeviceID:   deviceID,
				Resolution: telemetryquery.ResolutionFiveMinutes,
				From:       from,
				To:         to,
				Points: []telemetryquery.Point{{
					BucketStart: from,
					BucketEnd:   from.Add(5 * time.Minute),
					Metrics: telemetryquery.Metrics{
						PVAvgW: &pvAvg,
					},
				}},
			},
			{DeviceID: deviceID, Resolution: telemetryquery.ResolutionDay, From: previousFrom, To: from, Points: []telemetryquery.Point{}},
			{DeviceID: deviceID, Resolution: telemetryquery.ResolutionFiveMinutes, From: previousFrom, To: from, Points: []telemetryquery.Point{}},
		},
	}
	svc := NewEnergyServiceWithDeps(EnergyServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		ControlPlaneStore: store,
		QueryReader:       reader,
		Now:               func() time.Time { return now },
	})

	resp, err := svc.GetEnergyDashboard(
		grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"}),
		&telemetryv1.GetEnergyDashboardRequest{
			DeviceId:          deviceID,
			Preset:            "thisMonth",
			Timezone:          "UTC",
			IncludeComparison: true,
		},
	)
	if err != nil {
		t.Fatalf("GetEnergyDashboard failed: %v", err)
	}
	if got := len(resp.GetPvPortHistory()); got != 0 {
		t.Fatalf("expected dashboard PV history to stay empty for lazy load, got pv history rows=%d", got)
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

type fakeQueryReader struct {
	mu               sync.Mutex
	queries          []telemetryquery.RangeQuery
	aggregateQueries []telemetryquery.AggregateRangeQuery
	series           []telemetryquery.Series
	err              error
}

func (f *fakeQueryReader) QueryRange(_ context.Context, query telemetryquery.RangeQuery) (telemetryquery.Series, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.queries = append(f.queries, query)
	if f.err != nil {
		return telemetryquery.Series{}, f.err
	}
	if len(f.series) == 0 {
		return telemetryquery.Series{
			DeviceID:   query.DeviceID,
			Resolution: query.Resolution,
			From:       query.From,
			To:         query.To,
		}, nil
	}
	for idx, candidate := range f.series {
		if candidate.DeviceID == query.DeviceID &&
			candidate.Resolution == query.Resolution &&
			candidate.From.Equal(query.From) &&
			candidate.To.Equal(query.To) {
			f.series = append(f.series[:idx], f.series[idx+1:]...)
			return candidate, nil
		}
	}
	return telemetryquery.Series{
		DeviceID:   query.DeviceID,
		Resolution: query.Resolution,
		From:       query.From,
		To:         query.To,
		Points:     []telemetryquery.Point{},
	}, nil
}

func (f *fakeQueryReader) Close() error {
	return nil
}

func (f *fakeQueryReader) QueryRangeMany(_ context.Context, query telemetryquery.AggregateRangeQuery) (telemetryquery.Series, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.aggregateQueries = append(f.aggregateQueries, query)
	if f.err != nil {
		return telemetryquery.Series{}, f.err
	}
	if len(f.series) == 0 {
		return telemetryquery.Series{
			DeviceID:   query.AggregateID,
			Resolution: query.Resolution,
			From:       query.From,
			To:         query.To,
		}, nil
	}
	deviceFilter := make(map[string]struct{}, len(query.DeviceIDs))
	for _, deviceID := range query.DeviceIDs {
		deviceFilter[deviceID] = struct{}{}
	}
	aggregated := telemetryquery.Series{
		DeviceID:   query.AggregateID,
		Resolution: query.Resolution,
		From:       query.From,
		To:         query.To,
		Points:     []telemetryquery.Point{},
	}
	consumed := make([]int, 0, len(query.DeviceIDs))
	for idx, candidate := range f.series {
		if _, ok := deviceFilter[candidate.DeviceID]; !ok {
			continue
		}
		if candidate.Resolution != query.Resolution ||
			!candidate.From.Equal(query.From) ||
			!candidate.To.Equal(query.To) {
			continue
		}
		aggregated = mergeSeries(aggregated, candidate)
		consumed = append(consumed, idx)
	}
	if len(consumed) == 0 {
		return telemetryquery.Series{
			DeviceID:   query.AggregateID,
			Resolution: query.Resolution,
			From:       query.From,
			To:         query.To,
			Points:     []telemetryquery.Point{},
		}, nil
	}
	for i := len(consumed) - 1; i >= 0; i-- {
		idx := consumed[i]
		f.series = append(f.series[:idx], f.series[idx+1:]...)
	}
	return aggregated, nil
}

type fakeControlPlaneStore struct {
	userDevices     map[string][]controlplane.UserDevice
	providerDevices map[string]controlplane.ProviderDevice
}

func newFakeControlPlaneStore(userDevices map[string][]controlplane.UserDevice) *fakeControlPlaneStore {
	return &fakeControlPlaneStore{
		userDevices:     userDevices,
		providerDevices: map[string]controlplane.ProviderDevice{},
	}
}

func (f *fakeControlPlaneStore) CreateProviderCredential(context.Context, controlplane.CreateProviderCredentialInput) (controlplane.ProviderCredential, error) {
	return controlplane.ProviderCredential{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) ListProviderCredentials(context.Context, controlplane.ListProviderCredentialsInput) ([]controlplane.ProviderCredential, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) SetProviderCredentialActive(context.Context, controlplane.SetProviderCredentialActiveInput) (controlplane.ProviderCredential, error) {
	return controlplane.ProviderCredential{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) GetProviderCredential(context.Context, string, string) (controlplane.ProviderCredential, error) {
	return controlplane.ProviderCredential{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) CreateDevice(context.Context, controlplane.CreateDeviceInput) (controlplane.UserDevice, error) {
	return controlplane.UserDevice{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) LinkDevice(context.Context, controlplane.LinkDeviceInput) (controlplane.UserDevice, error) {
	return controlplane.UserDevice{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) ListUserDevices(_ context.Context, in controlplane.ListUserDevicesInput) ([]controlplane.UserDevice, error) {
	rows := f.userDevices[in.UserSubject]
	out := make([]controlplane.UserDevice, len(rows))
	copy(out, rows)
	return out, nil
}

func (f *fakeControlPlaneStore) GetOrProvisionCurrentUser(context.Context, controlplane.GetOrProvisionCurrentUserInput) (controlplane.CurrentUser, error) {
	return controlplane.CurrentUser{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) UpdateCurrentUserProfile(context.Context, controlplane.UpdateCurrentUserProfileInput) (controlplane.CurrentUser, error) {
	return controlplane.CurrentUser{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) UpsertProviderDevice(context.Context, controlplane.UpsertProviderDeviceInput) (controlplane.ProviderDevice, error) {
	return controlplane.ProviderDevice{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) ListProviderDevices(context.Context, controlplane.ListProviderDevicesInput) ([]controlplane.ProviderDevice, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) GetProviderDeviceByDeviceID(_ context.Context, deviceID string) (controlplane.ProviderDevice, error) {
	if f.providerDevices == nil {
		return controlplane.ProviderDevice{}, errors.New("not implemented")
	}
	row, ok := f.providerDevices[deviceID]
	if !ok {
		return controlplane.ProviderDevice{}, errors.New("not implemented")
	}
	return row, nil
}

func (f *fakeControlPlaneStore) ListIngestAssignments(context.Context, controlplane.ListIngestAssignmentsInput) ([]controlplane.IngestAssignment, error) {
	return nil, errors.New("not implemented")
}

type fakeManifestStore struct {
	objects []replaycli.ManifestObject
}

func (f fakeManifestStore) ListByDevices(context.Context, replaycli.DeviceQuery) ([]replaycli.ManifestObject, error) {
	out := make([]replaycli.ManifestObject, len(f.objects))
	copy(out, f.objects)
	return out, nil
}

func (f fakeManifestStore) ListByFleetRange(context.Context, replaycli.FleetQuery) ([]replaycli.ManifestObject, error) {
	return nil, nil
}

func (f fakeManifestStore) Close() error { return nil }

type fakeObjectReader struct {
	read func(bucket, key string) ([]byte, error)
}

func (f fakeObjectReader) ReadObject(_ context.Context, bucket string, key string) ([]byte, error) {
	if f.read == nil {
		return nil, errors.New("not implemented")
	}
	return f.read(bucket, key)
}

func (f fakeObjectReader) Close() error { return nil }
