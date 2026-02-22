package main

import (
	"context"
	"log/slog"
	"time"

	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TelemetryService is a bootstrap implementation.
// In M2 you will back this with NATS (deltas) + Valkey (snapshots) + Timescale (history).
type TelemetryService struct {
	telemetryv1.UnimplementedTelemetryServiceServer
	log *slog.Logger
}

var defaultSnapshotMetrics = map[string]float64{
	"soc":       50,
	"watts_in":  0,
	"watts_out": 0,
}

func NewTelemetryService(log *slog.Logger) *TelemetryService {
	return &TelemetryService{log: log}
}

func (s *TelemetryService) GetSnapshot(ctx context.Context, req *telemetryv1.GetSnapshotRequest) (*telemetryv1.GetSnapshotResponse, error) {
	if req.GetDeviceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id required")
	}

	nowMs := time.Now().UnixMilli()
	// TODO: fetch from Valkey (live snapshot).
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

	// First message: snapshot (if requested).
	if req.GetIncludeInitialSnapshot() {
		resp, err := s.GetSnapshot(stream.Context(), &telemetryv1.GetSnapshotRequest{DeviceId: req.DeviceId})
		if err != nil {
			return err
		}
		if err := stream.Send(&telemetryv1.TelemetryUpdate{Payload: &telemetryv1.TelemetryUpdate_Snapshot{Snapshot: resp.Snapshot}}); err != nil {
			return err
		}
	}

	// Bootstrap: heartbeat stream (replace with NATS-backed deltas).
	// Demonstrates server-streaming, cancellation, and keepalive-friendly periodic writes.
	ticker := time.NewTicker(1 * time.Second)
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
			if err := stream.Send(&telemetryv1.TelemetryUpdate{Payload: &telemetryv1.TelemetryUpdate_Heartbeat{Heartbeat: hb}}); err != nil {
				return err
			}
		}
	}
}
