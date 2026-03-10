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
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
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
		"owner": {{DeviceID: deviceID, EcoflowSN: "R351ZABAPH331057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
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
		"owner": {{DeviceID: deviceID, EcoflowSN: "Y711ZABA9H2P0294", ProductName: "DPU A 12 kWh", Model: "DELTA Pro Ultra", Role: "admin"}},
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

	svc := newTestService()
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
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "R351ZABAPH331057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
	})

	svc := NewTelemetryServiceWithDeps(TelemetryServiceDeps{
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
		"owner": {{DeviceID: deviceID, EcoflowSN: "Y711ZABA9H2P0294", ProductName: "DPU A 12 kWh", Model: "DELTA Pro Ultra", Role: "admin"}},
	})

	svc := NewTelemetryServiceWithDeps(TelemetryServiceDeps{
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
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "R351ZABAPH331057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
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

	svc := NewTelemetryServiceWithDeps(TelemetryServiceDeps{
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
		"dev-user": {{DeviceID: deviceID, EcoflowSN: "R351ZABAPH331057", ProductName: "Kitchen Delta 2 Max", Model: "DELTA 2 Max", Role: "admin"}},
	})

	from := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Hour)
	reader := &fakeQueryReader{
		series: []telemetryquery.Series{
			{DeviceID: deviceID, Resolution: telemetryquery.ResolutionHour, From: from, To: to},
			{DeviceID: deviceID, Resolution: telemetryquery.ResolutionHour, From: from.Add(-2 * time.Hour), To: from},
		},
	}
	svc := NewTelemetryServiceWithDeps(TelemetryServiceDeps{
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
	if reader.queries[1].From != from.Add(-2*time.Hour) || reader.queries[1].To != from {
		t.Fatalf("previous window mismatch: got=[%s,%s)", reader.queries[1].From, reader.queries[1].To)
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
	mu      sync.Mutex
	queries []telemetryquery.RangeQuery
	series  []telemetryquery.Series
	err     error
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
	next := f.series[0]
	f.series = f.series[1:]
	return next, nil
}

func (f *fakeQueryReader) Close() error {
	return nil
}

type fakeControlPlaneStore struct {
	userDevices map[string][]controlplane.UserDevice
}

func newFakeControlPlaneStore(userDevices map[string][]controlplane.UserDevice) *fakeControlPlaneStore {
	return &fakeControlPlaneStore{userDevices: userDevices}
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

func (f *fakeControlPlaneStore) UpsertProviderDevice(context.Context, controlplane.UpsertProviderDeviceInput) (controlplane.ProviderDevice, error) {
	return controlplane.ProviderDevice{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) ListProviderDevices(context.Context, controlplane.ListProviderDevicesInput) ([]controlplane.ProviderDevice, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) GetProviderDeviceByDeviceID(context.Context, string) (controlplane.ProviderDevice, error) {
	return controlplane.ProviderDevice{}, errors.New("not implemented")
}

func (f *fakeControlPlaneStore) ListIngestAssignments(context.Context, controlplane.ListIngestAssignmentsInput) ([]controlplane.IngestAssignment, error) {
	return nil, errors.New("not implemented")
}
