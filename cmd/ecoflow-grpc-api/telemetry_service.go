package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/energydashboard"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// TelemetryService serves live snapshots/streams and historical rollup queries.
// Live paths read from the Valkey projection read model; history reads from
// Timescale rollup tables through the telemetryquery reader.
type TelemetryService struct {
	telemetryv1.UnimplementedTelemetryServiceServer
	log               *slog.Logger
	snapshotReader    projectionworker.SnapshotReader
	controlPlaneStore controlplane.Store
}

type TelemetryServiceDeps struct {
	Log               *slog.Logger
	SnapshotReader    projectionworker.SnapshotReader
	ControlPlaneStore controlplane.Store
}

type EnergyService struct {
	telemetryv1.UnimplementedEnergyServiceServer
	log                  *slog.Logger
	queryReader          telemetryquery.Reader
	controlPlaneStore    controlplane.Store
	archiveManifestStore replaycli.ManifestStore
	archiveObjectReader  replaycli.ObjectReader
	maxQueryBuckets      int
	historyGzipMinBytes  int
	pvPortHistoryCache   map[string]pvPortHistoryCacheEntry
	pvPortHistoryMu      sync.Mutex
	pvPortHistoryGroup   singleflight.Group
	now                  func() time.Time
}

type EnergyServiceDeps struct {
	Log                  *slog.Logger
	QueryReader          telemetryquery.Reader
	ControlPlaneStore    controlplane.Store
	ArchiveManifestStore replaycli.ManifestStore
	ArchiveObjectReader  replaycli.ObjectReader
	MaxQueryBuckets      int
	HistoryGzipMinBytes  int
	Now                  func() time.Time
}

var defaultSnapshotMetrics = map[string]float64{
	"soc":       50,
	"watts_in":  0,
	"watts_out": 0,
}

const (
	defaultSubscribeUpdateHz     uint32 = 4
	maxSubscribeUpdateHz         uint32 = 50
	defaultMaxQueryBuckets              = 10_000
	defaultHistoryGzipMinBytes          = 16 << 10 // 16 KiB
	defaultPVPortHistoryCacheTTL        = 15 * time.Second
)

type pvPortHistoryCacheEntry struct {
	rows      []energydashboard.PVPortHistory
	expiresAt time.Time
}

func NewTelemetryService(log *slog.Logger) *TelemetryService {
	return NewTelemetryServiceWithDeps(TelemetryServiceDeps{Log: log})
}

func NewTelemetryServiceWithSnapshotReader(log *slog.Logger, reader projectionworker.SnapshotReader) *TelemetryService {
	return NewTelemetryServiceWithDeps(TelemetryServiceDeps{
		Log:            log,
		SnapshotReader: reader,
	})
}

func NewTelemetryServiceWithDeps(deps TelemetryServiceDeps) *TelemetryService {
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	return &TelemetryService{
		log:               log,
		snapshotReader:    deps.SnapshotReader,
		controlPlaneStore: deps.ControlPlaneStore,
	}
}

func NewEnergyService(log *slog.Logger) *EnergyService {
	return NewEnergyServiceWithDeps(EnergyServiceDeps{Log: log})
}

func NewEnergyServiceWithDeps(deps EnergyServiceDeps) *EnergyService {
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	maxQueryBuckets := deps.MaxQueryBuckets
	if maxQueryBuckets <= 0 {
		maxQueryBuckets = defaultMaxQueryBuckets
	}
	historyGzipMinBytes := deps.HistoryGzipMinBytes
	if historyGzipMinBytes <= 0 {
		historyGzipMinBytes = defaultHistoryGzipMinBytes
	}
	nowFn := deps.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	return &EnergyService{
		log:                  log,
		queryReader:          deps.QueryReader,
		controlPlaneStore:    deps.ControlPlaneStore,
		archiveManifestStore: deps.ArchiveManifestStore,
		archiveObjectReader:  deps.ArchiveObjectReader,
		maxQueryBuckets:      maxQueryBuckets,
		historyGzipMinBytes:  historyGzipMinBytes,
		pvPortHistoryCache:   map[string]pvPortHistoryCacheEntry{},
		now:                  nowFn,
	}
}

func (s *TelemetryService) GetSnapshot(ctx context.Context, req *telemetryv1.GetSnapshotRequest) (*telemetryv1.GetSnapshotResponse, error) {
	if req.GetDeviceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id required")
	}
	if err := authorizeDeviceAccess(ctx, s.controlPlaneStore, req.GetDeviceId()); err != nil {
		return nil, err
	}

	nowMs := time.Now().UnixMilli()
	if s.snapshotReader != nil {
		snap, err := s.snapshotReader.ReadSnapshot(ctx, projectionworker.SnapshotIdentity{DeviceID: req.GetDeviceId()})
		if err != nil {
			s.log.Warn("snapshot read failed", "device_id", req.GetDeviceId(), "error", err.Error())
			return nil, status.Error(codes.Unavailable, "snapshot unavailable")
		}
		if snap != nil {
			deviceID := strings.TrimSpace(snap.DeviceID)
			if deviceID == "" {
				deviceID = req.GetDeviceId()
			}
			cursorTs := snap.Cursor.TsUnixMs
			if cursorTs <= 0 {
				cursorTs = nowMs
			}
			metrics := snap.Metrics
			if metrics == nil {
				metrics = map[string]float64{}
			}
			return &telemetryv1.GetSnapshotResponse{
				Snapshot: &telemetryv1.Snapshot{
					DeviceId: deviceID,
					Cursor:   &telemetryv1.Cursor{Seq: snap.Cursor.Seq, TsUnixMs: cursorTs},
					Metrics:  metrics,
				},
			}, nil
		}
	}

	snap := &telemetryv1.Snapshot{
		DeviceId: req.DeviceId,
		Cursor:   &telemetryv1.Cursor{Seq: 1, TsUnixMs: nowMs},
		Metrics:  cloneMetrics(defaultSnapshotMetrics),
	}

	return &telemetryv1.GetSnapshotResponse{Snapshot: snap}, nil
}

func (s *TelemetryService) Subscribe(req *telemetryv1.SubscribeRequest, stream telemetryv1.TelemetryService_SubscribeServer) error {
	if req.GetDeviceId() == "" {
		return status.Error(codes.InvalidArgument, "device_id required")
	}
	if err := authorizeDeviceAccess(stream.Context(), s.controlPlaneStore, req.GetDeviceId()); err != nil {
		return err
	}

	var initialSnapshot *telemetryv1.Snapshot
	if req.GetIncludeInitialSnapshot() {
		resp, err := s.GetSnapshot(stream.Context(), &telemetryv1.GetSnapshotRequest{DeviceId: req.DeviceId})
		if err != nil {
			return err
		}
		initialSnapshot = resp.GetSnapshot()
		if err := stream.Send(&telemetryv1.SubscribeResponse{Payload: &telemetryv1.SubscribeResponse_Snapshot{Snapshot: resp.Snapshot}}); err != nil {
			return err
		}
	}

	updateHz := req.GetMaxUpdateHz()
	if updateHz == 0 {
		updateHz = defaultSubscribeUpdateHz
	}
	if updateHz > maxSubscribeUpdateHz {
		updateHz = maxSubscribeUpdateHz
	}
	updateInterval := time.Second / time.Duration(updateHz)

	if s.snapshotReader != nil {
		return s.subscribeFromReadModel(req, stream, updateInterval, initialSnapshot)
	}

	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	var seq uint64 = 1
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			seq++
			hb := &telemetryv1.Heartbeat{
				DeviceId: req.DeviceId,
				Cursor:   &telemetryv1.Cursor{Seq: seq, TsUnixMs: time.Now().UnixMilli()},
			}
			if err := stream.Send(&telemetryv1.SubscribeResponse{Payload: &telemetryv1.SubscribeResponse_Heartbeat{Heartbeat: hb}}); err != nil {
				return err
			}
		}
	}
}

func (s *EnergyService) QueryRollupRange(ctx context.Context, req *telemetryv1.QueryRollupRangeRequest) (*telemetryv1.QueryRollupRangeResponse, error) {
	query, err := s.buildRangeQuery(req.GetDeviceId(), req.GetResolution(), req.GetFromUnixMs(), req.GetToUnixMs())
	if err != nil {
		return nil, err
	}
	if err := authorizeDeviceAccess(ctx, s.controlPlaneStore, query.DeviceID); err != nil {
		return nil, err
	}
	if s.queryReader == nil {
		return nil, status.Error(codes.Unavailable, "telemetry history unavailable")
	}

	series, err := s.queryReader.QueryRange(ctx, query)
	if err != nil {
		return nil, s.mapQueryError(err)
	}
	s.logEnergyBucketFallback("query_rollup_range", series)
	resp := &telemetryv1.QueryRollupRangeResponse{
		Series: seriesToProto(series),
	}
	s.maybeEnableHistoryCompression(ctx, resp)
	return resp, nil
}

func (s *EnergyService) CompareRollupRange(ctx context.Context, req *telemetryv1.CompareRollupRangeRequest) (*telemetryv1.CompareRollupRangeResponse, error) {
	currentQuery, err := s.buildRangeQuery(req.GetDeviceId(), req.GetResolution(), req.GetFromUnixMs(), req.GetToUnixMs())
	if err != nil {
		return nil, err
	}
	if err := authorizeDeviceAccess(ctx, s.controlPlaneStore, currentQuery.DeviceID); err != nil {
		return nil, err
	}
	if s.queryReader == nil {
		return nil, status.Error(codes.Unavailable, "telemetry history unavailable")
	}

	previousQuery, err := s.buildCompareQuery(req, currentQuery)
	if err != nil {
		return nil, err
	}

	var (
		current  telemetryquery.Series
		previous telemetryquery.Series
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		series, queryErr := s.queryReader.QueryRange(groupCtx, currentQuery)
		if queryErr != nil {
			return queryErr
		}
		current = series
		return nil
	})
	group.Go(func() error {
		series, queryErr := s.queryReader.QueryRange(groupCtx, previousQuery)
		if queryErr != nil {
			return queryErr
		}
		previous = series
		return nil
	})
	if err := group.Wait(); err != nil {
		return nil, s.mapQueryError(err)
	}
	s.logEnergyBucketFallback("compare_rollup_range_current", current)
	s.logEnergyBucketFallback("compare_rollup_range_previous", previous)

	resp := &telemetryv1.CompareRollupRangeResponse{
		Current:  seriesToProto(current),
		Previous: seriesToProto(previous),
	}
	s.maybeEnableHistoryCompression(ctx, resp)
	return resp, nil
}

func (s *EnergyService) maybeEnableHistoryCompression(ctx context.Context, message proto.Message) {
	if s == nil || !shouldCompressHistoryResponse(message, s.historyGzipMinBytes) {
		return
	}
	if err := grpc.SetSendCompressor(ctx, "gzip"); err != nil {
		if s.log != nil {
			s.log.Warn("set history response compressor failed", "error", err.Error())
		}
	}
}

func (s *EnergyService) logEnergyBucketFallback(operation string, series telemetryquery.Series) {
	if s == nil || s.log == nil {
		return
	}
	coverage := series.EnergyBucketCoverage
	if coverage.DerivedValueCount == 0 {
		return
	}
	s.log.Info(
		"telemetry history used derived energy fallback",
		"operation", operation,
		"device_id", series.DeviceID,
		"resolution", series.Resolution.String(),
		"points", coverage.PointCount,
		"derived_points", coverage.DerivedPointCount,
		"persisted_values", coverage.PersistedValueCount,
		"derived_values", coverage.DerivedValueCount,
	)
}

func shouldCompressHistoryResponse(message proto.Message, minBytes int) bool {
	if message == nil || minBytes <= 0 {
		return false
	}
	return proto.Size(message) >= minBytes
}

func (s *TelemetryService) subscribeFromReadModel(
	req *telemetryv1.SubscribeRequest,
	stream telemetryv1.TelemetryService_SubscribeServer,
	updateInterval time.Duration,
	initialSnapshot *telemetryv1.Snapshot,
) error {
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	type cursorState struct {
		seq      uint64
		tsUnixMs int64
	}
	var lastCursor cursorState
	var lastMetrics map[string]float64
	if req.GetIncludeInitialSnapshot() && initialSnapshot != nil {
		lastCursor = cursorState{
			seq:      initialSnapshot.GetCursor().GetSeq(),
			tsUnixMs: initialSnapshot.GetCursor().GetTsUnixMs(),
		}
		lastMetrics = cloneMetrics(initialSnapshot.GetMetrics())
	}

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			snap, err := s.snapshotReader.ReadSnapshot(stream.Context(), projectionworker.SnapshotIdentity{DeviceID: req.GetDeviceId()})
			if err != nil {
				s.log.Warn("subscribe snapshot read failed", "device_id", req.GetDeviceId(), "error", err.Error())
				if err := stream.Send(&telemetryv1.SubscribeResponse{
					Payload: &telemetryv1.SubscribeResponse_Heartbeat{
						Heartbeat: &telemetryv1.Heartbeat{
							DeviceId: req.GetDeviceId(),
							Cursor:   &telemetryv1.Cursor{Seq: lastCursor.seq, TsUnixMs: time.Now().UnixMilli()},
						},
					},
				}); err != nil {
					return err
				}
				continue
			}
			if snap == nil {
				if err := stream.Send(&telemetryv1.SubscribeResponse{
					Payload: &telemetryv1.SubscribeResponse_Heartbeat{
						Heartbeat: &telemetryv1.Heartbeat{
							DeviceId: req.GetDeviceId(),
							Cursor:   &telemetryv1.Cursor{Seq: lastCursor.seq, TsUnixMs: time.Now().UnixMilli()},
						},
					},
				}); err != nil {
					return err
				}
				continue
			}

			currentCursor := cursorState{
				seq:      snap.Cursor.Seq,
				tsUnixMs: snap.Cursor.TsUnixMs,
			}
			if currentCursor.tsUnixMs <= 0 {
				currentCursor.tsUnixMs = time.Now().UnixMilli()
			}
			currentMetrics := cloneMetrics(snap.Metrics)

			changed, cleared := computeDelta(lastMetrics, currentMetrics)
			hasDelta := len(changed) > 0 || len(cleared) > 0
			cursorAdvanced := currentCursor.seq > lastCursor.seq || currentCursor.tsUnixMs > lastCursor.tsUnixMs

			if hasDelta || cursorAdvanced {
				if hasDelta {
					if err := stream.Send(&telemetryv1.SubscribeResponse{
						Payload: &telemetryv1.SubscribeResponse_Delta{
							Delta: &telemetryv1.Delta{
								DeviceId: req.GetDeviceId(),
								Cursor:   &telemetryv1.Cursor{Seq: currentCursor.seq, TsUnixMs: currentCursor.tsUnixMs},
								Changed:  changed,
								Cleared:  cleared,
							},
						},
					}); err != nil {
						return err
					}
				} else {
					if err := stream.Send(&telemetryv1.SubscribeResponse{
						Payload: &telemetryv1.SubscribeResponse_Heartbeat{
							Heartbeat: &telemetryv1.Heartbeat{
								DeviceId: req.GetDeviceId(),
								Cursor:   &telemetryv1.Cursor{Seq: currentCursor.seq, TsUnixMs: currentCursor.tsUnixMs},
							},
						},
					}); err != nil {
						return err
					}
				}
			} else {
				if err := stream.Send(&telemetryv1.SubscribeResponse{
					Payload: &telemetryv1.SubscribeResponse_Heartbeat{
						Heartbeat: &telemetryv1.Heartbeat{
							DeviceId: req.GetDeviceId(),
							Cursor:   &telemetryv1.Cursor{Seq: lastCursor.seq, TsUnixMs: time.Now().UnixMilli()},
						},
					},
				}); err != nil {
					return err
				}
			}

			lastCursor = currentCursor
			lastMetrics = currentMetrics
		}
	}
}

func (s *EnergyService) buildRangeQuery(deviceID string, resolution telemetryv1.RollupResolution, fromUnixMs int64, toUnixMs int64) (telemetryquery.RangeQuery, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return telemetryquery.RangeQuery{}, status.Error(codes.InvalidArgument, "device_id required")
	}
	if _, err := uuid.Parse(deviceID); err != nil {
		return telemetryquery.RangeQuery{}, status.Error(codes.InvalidArgument, "device_id must be a UUID")
	}
	resolved, err := resolutionFromProto(resolution)
	if err != nil {
		return telemetryquery.RangeQuery{}, status.Error(codes.InvalidArgument, err.Error())
	}
	if fromUnixMs <= 0 || toUnixMs <= 0 {
		return telemetryquery.RangeQuery{}, status.Error(codes.InvalidArgument, "from_unix_ms and to_unix_ms must be positive")
	}

	from := time.UnixMilli(fromUnixMs).UTC()
	to := time.UnixMilli(toUnixMs).UTC()
	if !from.Before(to) {
		return telemetryquery.RangeQuery{}, status.Error(codes.InvalidArgument, "range must satisfy from_unix_ms < to_unix_ms")
	}

	limit, err := maxBucketsForRange(from, to, resolved, s.maxQueryBuckets)
	if err != nil {
		return telemetryquery.RangeQuery{}, status.Error(codes.InvalidArgument, err.Error())
	}
	return telemetryquery.RangeQuery{
		DeviceID:   deviceID,
		Resolution: resolved,
		From:       from,
		To:         to,
		Limit:      limit,
	}, nil
}

func (s *EnergyService) buildCompareQuery(req *telemetryv1.CompareRollupRangeRequest, current telemetryquery.RangeQuery) (telemetryquery.RangeQuery, error) {
	explicitFrom := req.GetCompareFromUnixMs()
	explicitTo := req.GetCompareToUnixMs()
	if explicitFrom > 0 || explicitTo > 0 {
		if explicitFrom <= 0 || explicitTo <= 0 {
			return telemetryquery.RangeQuery{}, status.Error(codes.InvalidArgument, "compare_from_unix_ms and compare_to_unix_ms must both be set")
		}
		return s.buildRangeQuery(req.GetDeviceId(), req.GetResolution(), explicitFrom, explicitTo)
	}
	if !req.GetUsePreviousPeriod() {
		return telemetryquery.RangeQuery{}, status.Error(codes.InvalidArgument, "comparison window required")
	}

	window := current.To.Sub(current.From)
	return s.buildRangeQuery(
		req.GetDeviceId(),
		req.GetResolution(),
		current.From.Add(-window).UnixMilli(),
		current.From.UnixMilli(),
	)
}

func authorizeDeviceAccess(ctx context.Context, store controlplane.Store, deviceID string) error {
	if store == nil {
		return nil
	}
	claims, ok := grpcmw.ClaimsFromContext(ctx)
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return nil
	}
	rows, err := store.ListUserDevices(ctx, controlplane.ListUserDevicesInput{UserSubject: claims.Subject})
	if err != nil {
		return status.Errorf(codes.Internal, "authorize telemetry device access: %v", err)
	}
	for i := range rows {
		if rows[i].DeviceID == deviceID {
			return nil
		}
	}
	return status.Error(codes.PermissionDenied, "device access denied")
}

func (s *EnergyService) mapQueryError(err error) error {
	switch err {
	case nil:
		return nil
	case telemetryquery.ErrInvalidResolution, telemetryquery.ErrInvalidRange:
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Errorf(codes.Internal, "query telemetry rollups: %v", err)
	}
}

func resolutionFromProto(value telemetryv1.RollupResolution) (telemetryquery.Resolution, error) {
	switch value {
	case telemetryv1.RollupResolution_ROLLUP_RESOLUTION_MINUTE:
		return telemetryquery.ResolutionMinute, nil
	case telemetryv1.RollupResolution_ROLLUP_RESOLUTION_HOUR:
		return telemetryquery.ResolutionHour, nil
	case telemetryv1.RollupResolution_ROLLUP_RESOLUTION_DAY:
		return telemetryquery.ResolutionDay, nil
	default:
		return telemetryquery.ResolutionUnknown, telemetryquery.ErrInvalidResolution
	}
}

func maxBucketsForRange(from, to time.Time, resolution telemetryquery.Resolution, maxBuckets int) (int, error) {
	bucketWidth := resolution.BucketDuration()
	if bucketWidth <= 0 {
		return 0, telemetryquery.ErrInvalidResolution
	}
	window := to.Sub(from)
	if window <= 0 {
		return 0, telemetryquery.ErrInvalidRange
	}
	count := int(window / bucketWidth)
	if window%bucketWidth != 0 {
		count++
	}
	if count <= 0 {
		return 0, telemetryquery.ErrInvalidRange
	}
	if count > maxBuckets {
		return 0, fmt.Errorf("query window too large for resolution (max %d buckets)", maxBuckets)
	}
	return count, nil
}

func seriesToProto(series telemetryquery.Series) *telemetryv1.RollupSeries {
	points := make([]*telemetryv1.RollupPoint, 0, len(series.Points))
	for i := range series.Points {
		points = append(points, pointToProto(series.Points[i]))
	}
	return &telemetryv1.RollupSeries{
		DeviceId:   series.DeviceID,
		Resolution: resolutionToProto(series.Resolution),
		FromUnixMs: series.From.UnixMilli(),
		ToUnixMs:   series.To.UnixMilli(),
		Points:     points,
	}
}

func pointToProto(point telemetryquery.Point) *telemetryv1.RollupPoint {
	return &telemetryv1.RollupPoint{
		BucketStartUnixMs: point.BucketStart.UnixMilli(),
		BucketEndUnixMs:   point.BucketEnd.UnixMilli(),
		SampleCount:       point.SampleCount,
		FirstTsUnixMs:     point.FirstTsUnixMs,
		LastTsUnixMs:      point.LastTsUnixMs,
		Metrics:           metricsToProto(point.Metrics),
	}
}

func metricsToProto(metrics telemetryquery.Metrics) *telemetryv1.RollupMetrics {
	if isEmptyMetrics(metrics) {
		return nil
	}
	out := &telemetryv1.RollupMetrics{}
	out.SocAvgPct = metrics.SOCAvgPct
	out.SocMinPct = metrics.SOCMinPct
	out.SocMaxPct = metrics.SOCMaxPct
	out.AcInAvgW = metrics.ACInAvgW
	out.AcInMaxW = metrics.ACInMaxW
	out.AcOutputAvgW = metrics.ACOutputAvgW
	out.AcOutputMaxW = metrics.ACOutputMaxW
	out.PvAvgW = metrics.PVAvgW
	out.PvMaxW = metrics.PVMaxW
	out.DcAvgW = metrics.DCAvgW
	out.DcMaxW = metrics.DCMaxW
	out.LoadAvgW = metrics.LoadAvgW
	out.LoadMaxW = metrics.LoadMaxW
	out.NetAvgW = metrics.NetAvgW
	out.NetMinW = metrics.NetMinW
	out.NetMaxW = metrics.NetMaxW
	out.BatteryAvgW = metrics.BatteryAvgW
	out.BatteryMinW = metrics.BatteryMinW
	out.BatteryMaxW = metrics.BatteryMaxW
	out.TempAvgC = metrics.TempAvgC
	out.TempMinC = metrics.TempMinC
	out.TempMaxC = metrics.TempMaxC
	out.SolarGeneratedWh = metrics.SolarGeneratedWh
	out.AcInputEnergyWh = metrics.ACInputEnergyWh
	out.AcOutputEnergyWh = metrics.ACOutputEnergyWh
	out.DcOutputEnergyWh = metrics.DCOutputEnergyWh
	out.LoadEnergyWh = metrics.LoadEnergyWh
	out.BatteryChargeEnergyWh = metrics.BatteryChargeEnergyWh
	out.BatteryDischargeEnergyWh = metrics.BatteryDischargeEnergyWh
	return out
}

func isEmptyMetrics(metrics telemetryquery.Metrics) bool {
	return metrics.SOCAvgPct == nil &&
		metrics.SOCMinPct == nil &&
		metrics.SOCMaxPct == nil &&
		metrics.ACInAvgW == nil &&
		metrics.ACInMaxW == nil &&
		metrics.ACOutputAvgW == nil &&
		metrics.ACOutputMaxW == nil &&
		metrics.PVAvgW == nil &&
		metrics.PVMaxW == nil &&
		metrics.DCAvgW == nil &&
		metrics.DCMaxW == nil &&
		metrics.LoadAvgW == nil &&
		metrics.LoadMaxW == nil &&
		metrics.NetAvgW == nil &&
		metrics.NetMinW == nil &&
		metrics.NetMaxW == nil &&
		metrics.BatteryAvgW == nil &&
		metrics.BatteryMinW == nil &&
		metrics.BatteryMaxW == nil &&
		metrics.TempAvgC == nil &&
		metrics.TempMinC == nil &&
		metrics.TempMaxC == nil &&
		metrics.SolarGeneratedWh == nil &&
		metrics.ACInputEnergyWh == nil &&
		metrics.ACOutputEnergyWh == nil &&
		metrics.DCOutputEnergyWh == nil &&
		metrics.LoadEnergyWh == nil &&
		metrics.BatteryChargeEnergyWh == nil &&
		metrics.BatteryDischargeEnergyWh == nil
}

func resolutionToProto(resolution telemetryquery.Resolution) telemetryv1.RollupResolution {
	switch resolution {
	case telemetryquery.ResolutionMinute:
		return telemetryv1.RollupResolution_ROLLUP_RESOLUTION_MINUTE
	case telemetryquery.ResolutionHour:
		return telemetryv1.RollupResolution_ROLLUP_RESOLUTION_HOUR
	case telemetryquery.ResolutionDay:
		return telemetryv1.RollupResolution_ROLLUP_RESOLUTION_DAY
	default:
		return telemetryv1.RollupResolution_ROLLUP_RESOLUTION_UNSPECIFIED
	}
}

func cloneMetrics(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func computeDelta(prev, curr map[string]float64) (map[string]float64, []string) {
	changed := make(map[string]float64)
	cleared := make([]string, 0)

	for key, currentValue := range curr {
		previousValue, ok := prev[key]
		if !ok || !floatEquals(previousValue, currentValue) {
			changed[key] = currentValue
		}
	}
	for key := range prev {
		if _, ok := curr[key]; !ok {
			cleared = append(cleared, key)
		}
	}
	slices.Sort(cleared)
	return changed, cleared
}

func floatEquals(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	return math.Abs(a-b) <= 1e-9
}
