package main

import (
	"context"
	"log/slog"
	"math"
	"slices"
	"strings"
	"time"

	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TelemetryService is a bootstrap implementation.
// In M2 you will back this with NATS (deltas) + Valkey (snapshots) + Timescale (history).
type TelemetryService struct {
	telemetryv1.UnimplementedTelemetryServiceServer
	log            *slog.Logger
	snapshotReader projectionworker.SnapshotReader
}

var defaultSnapshotMetrics = map[string]float64{
	"soc":       50,
	"watts_in":  0,
	"watts_out": 0,
}

const (
	defaultSubscribeUpdateHz uint32 = 4
	maxSubscribeUpdateHz     uint32 = 50
)

func NewTelemetryService(log *slog.Logger) *TelemetryService {
	return NewTelemetryServiceWithSnapshotReader(log, nil)
}

func NewTelemetryServiceWithSnapshotReader(log *slog.Logger, reader projectionworker.SnapshotReader) *TelemetryService {
	return &TelemetryService{
		log:            log,
		snapshotReader: reader,
	}
}

func (s *TelemetryService) GetSnapshot(ctx context.Context, req *telemetryv1.GetSnapshotRequest) (*telemetryv1.GetSnapshotResponse, error) {
	if req.GetDeviceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id required")
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
		// Immutable shared metrics map for bootstrap responses.
		// This avoids per-call map allocation on the hot path.
		Metrics: defaultSnapshotMetrics,
	}

	return &telemetryv1.GetSnapshotResponse{Snapshot: snap}, nil
}

func (s *TelemetryService) Subscribe(req *telemetryv1.SubscribeRequest, stream telemetryv1.TelemetryService_SubscribeServer) error {
	if req.GetDeviceId() == "" {
		return status.Error(codes.InvalidArgument, "device_id required")
	}

	var initialSnapshot *telemetryv1.Snapshot
	// First message: snapshot (if requested).
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

	// Bootstrap fallback when read-model is not configured: heartbeat stream.
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

func (s *TelemetryService) subscribeFromReadModel(
	req *telemetryv1.SubscribeRequest,
	stream telemetryv1.TelemetryService_SubscribeServer,
	updateInterval time.Duration,
	initialSnapshot *telemetryv1.Snapshot,
) error {
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	var lastCursor telemetryv1.Cursor
	var lastMetrics map[string]float64
	if req.GetIncludeInitialSnapshot() && initialSnapshot != nil {
		lastCursor = telemetryv1.Cursor{
			Seq:      initialSnapshot.GetCursor().GetSeq(),
			TsUnixMs: initialSnapshot.GetCursor().GetTsUnixMs(),
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
							Cursor:   &telemetryv1.Cursor{Seq: lastCursor.Seq, TsUnixMs: time.Now().UnixMilli()},
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
							Cursor:   &telemetryv1.Cursor{Seq: lastCursor.Seq, TsUnixMs: time.Now().UnixMilli()},
						},
					},
				}); err != nil {
					return err
				}
				continue
			}

			currentCursor := telemetryv1.Cursor{
				Seq:      snap.Cursor.Seq,
				TsUnixMs: snap.Cursor.TsUnixMs,
			}
			if currentCursor.TsUnixMs <= 0 {
				currentCursor.TsUnixMs = time.Now().UnixMilli()
			}
			currentMetrics := cloneMetrics(snap.Metrics)

			changed, cleared := computeDelta(lastMetrics, currentMetrics)
			hasDelta := len(changed) > 0 || len(cleared) > 0
			cursorAdvanced := currentCursor.Seq > lastCursor.Seq || currentCursor.TsUnixMs > lastCursor.TsUnixMs

			if hasDelta || cursorAdvanced {
				if hasDelta {
					if err := stream.Send(&telemetryv1.SubscribeResponse{
						Payload: &telemetryv1.SubscribeResponse_Delta{
							Delta: &telemetryv1.Delta{
								DeviceId: req.GetDeviceId(),
								Cursor:   &telemetryv1.Cursor{Seq: currentCursor.Seq, TsUnixMs: currentCursor.TsUnixMs},
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
								Cursor:   &telemetryv1.Cursor{Seq: currentCursor.Seq, TsUnixMs: currentCursor.TsUnixMs},
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
							Cursor:   &telemetryv1.Cursor{Seq: lastCursor.Seq, TsUnixMs: time.Now().UnixMilli()},
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

func cloneMetrics(in map[string]float64) map[string]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func computeDelta(previous, current map[string]float64) (map[string]float64, []string) {
	changed := make(map[string]float64)
	for k, v := range current {
		prev, ok := previous[k]
		if !ok || math.Float64bits(prev) != math.Float64bits(v) {
			changed[k] = v
		}
	}

	var cleared []string
	for k := range previous {
		if _, ok := current[k]; !ok {
			cleared = append(cleared, k)
		}
	}
	if len(cleared) > 1 {
		slices.Sort(cleared)
	}
	if len(changed) == 0 {
		changed = nil
	}
	if len(cleared) == 0 {
		cleared = nil
	}
	return changed, cleared
}
