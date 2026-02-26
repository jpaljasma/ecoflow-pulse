package gaprepair

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	replayv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/replay/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
)

type fakeAssignmentStore struct {
	rows []controlplane.IngestAssignment
	err  error
}

func (f fakeAssignmentStore) ListIngestAssignments(_ context.Context, in controlplane.ListIngestAssignmentsInput) ([]controlplane.IngestAssignment, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]controlplane.IngestAssignment(nil), f.rows...), nil
}

type fakeCoverageStore struct {
	rows map[string]map[string]CoverageWindow
	err  error
}

func (f fakeCoverageStore) CoverageByProviderDevices(_ context.Context, provider string, _ []string, _ int64, _ int64) (map[string]CoverageWindow, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.rows == nil {
		return map[string]CoverageWindow{}, nil
	}
	if got, ok := f.rows[provider]; ok {
		return got, nil
	}
	return map[string]CoverageWindow{}, nil
}

func (f fakeCoverageStore) Close() error { return nil }

type fakeSnapshotReader struct {
	rows map[string]*projectionworker.SnapshotReadModel
	err  error
}

func (f fakeSnapshotReader) ReadSnapshot(_ context.Context, identity projectionworker.SnapshotIdentity) (*projectionworker.SnapshotReadModel, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.rows == nil {
		return nil, nil
	}
	if snap, ok := f.rows[identity.DeviceID+"|"+identity.EcoflowSN]; ok {
		return snap, nil
	}
	return nil, nil
}

type fakeRequestPublisher struct {
	requests []*replayv1.GapRepairRequest
	err      error
}

func (f *fakeRequestPublisher) PublishGapRepair(_ context.Context, req *replayv1.GapRepairRequest) error {
	if f.err != nil {
		return f.err
	}
	f.requests = append(f.requests, req)
	return nil
}

func (f *fakeRequestPublisher) Close() error { return nil }

func TestDetectorDetectAndEnqueueSnapshotMissing(t *testing.T) {
	t.Parallel()

	publisher := &fakeRequestPublisher{}
	detector, err := NewDetector(
		slog.Default(),
		fakeAssignmentStore{rows: []controlplane.IngestAssignment{{Provider: "ecoflow", ProviderDeviceID: "R351ZABAPH331057", DeviceID: "dev-1"}}},
		fakeCoverageStore{rows: map[string]map[string]CoverageWindow{"ecoflow": {"R351ZABAPH331057": {ProviderDeviceID: "R351ZABAPH331057", MinUnixMS: 1000, MaxUnixMS: 8000, ObjectCount: 2}}}},
		fakeSnapshotReader{},
		publisher,
		DetectorConfig{NowFn: func() time.Time { return time.UnixMilli(9000).UTC() }, SubjectShardCount: 128, LookbackWindow: time.Hour, LagThreshold: time.Second, MaxReplayWindow: time.Hour, MaxJobsPerCycle: 10, PollInterval: time.Second},
	)
	if err != nil {
		t.Fatalf("NewDetector returned error: %v", err)
	}
	report, err := detector.DetectAndEnqueue(context.Background())
	if err != nil {
		t.Fatalf("DetectAndEnqueue returned error: %v", err)
	}
	if report.Enqueued != 1 {
		t.Fatalf("expected one enqueued request, got=%d", report.Enqueued)
	}
	if len(publisher.requests) != 1 {
		t.Fatalf("expected one published request, got=%d", len(publisher.requests))
	}
	if got := publisher.requests[0].GetReason(); got != "snapshot_missing" {
		t.Fatalf("expected snapshot_missing reason, got=%s", got)
	}
}

func TestDetectorSkipsWhenProjectionCaughtUp(t *testing.T) {
	t.Parallel()

	publisher := &fakeRequestPublisher{}
	detector, err := NewDetector(
		slog.Default(),
		fakeAssignmentStore{rows: []controlplane.IngestAssignment{{Provider: "ecoflow", ProviderDeviceID: "R351ZABAPH331057", DeviceID: "dev-1"}}},
		fakeCoverageStore{rows: map[string]map[string]CoverageWindow{"ecoflow": {"R351ZABAPH331057": {ProviderDeviceID: "R351ZABAPH331057", MinUnixMS: 1000, MaxUnixMS: 8000, ObjectCount: 2}}}},
		fakeSnapshotReader{rows: map[string]*projectionworker.SnapshotReadModel{"dev-1|R351ZABAPH331057": {Cursor: projectionworker.SnapshotCursor{TsUnixMs: 7950}}}},
		publisher,
		DetectorConfig{NowFn: func() time.Time { return time.UnixMilli(9000).UTC() }, SubjectShardCount: 128, LookbackWindow: time.Hour, LagThreshold: 100 * time.Millisecond, MaxReplayWindow: time.Hour, MaxJobsPerCycle: 10, PollInterval: time.Second},
	)
	if err != nil {
		t.Fatalf("NewDetector returned error: %v", err)
	}
	report, err := detector.DetectAndEnqueue(context.Background())
	if err != nil {
		t.Fatalf("DetectAndEnqueue returned error: %v", err)
	}
	if report.Enqueued != 0 || report.Candidates != 0 {
		t.Fatalf("expected no gap candidates, report=%+v", report)
	}
}

func TestDetectorLimitsJobsPerCycleByLag(t *testing.T) {
	t.Parallel()

	assignments := []controlplane.IngestAssignment{
		{Provider: "ecoflow", ProviderDeviceID: "SN-1", DeviceID: "dev-1"},
		{Provider: "ecoflow", ProviderDeviceID: "SN-2", DeviceID: "dev-2"},
	}
	coverage := map[string]map[string]CoverageWindow{"ecoflow": {
		"SN-1": {ProviderDeviceID: "SN-1", MinUnixMS: 1000, MaxUnixMS: 9000, ObjectCount: 1},
		"SN-2": {ProviderDeviceID: "SN-2", MinUnixMS: 1000, MaxUnixMS: 7000, ObjectCount: 1},
	}}
	snaps := map[string]*projectionworker.SnapshotReadModel{
		"dev-1|SN-1": {Cursor: projectionworker.SnapshotCursor{TsUnixMs: 1000}},
		"dev-2|SN-2": {Cursor: projectionworker.SnapshotCursor{TsUnixMs: 6500}},
	}
	publisher := &fakeRequestPublisher{}
	detector, err := NewDetector(
		slog.Default(),
		fakeAssignmentStore{rows: assignments},
		fakeCoverageStore{rows: coverage},
		fakeSnapshotReader{rows: snaps},
		publisher,
		DetectorConfig{NowFn: func() time.Time { return time.UnixMilli(10_000).UTC() }, SubjectShardCount: 128, LookbackWindow: time.Hour, LagThreshold: 100 * time.Millisecond, MaxReplayWindow: time.Hour, MaxJobsPerCycle: 1, PollInterval: time.Second},
	)
	if err != nil {
		t.Fatalf("NewDetector returned error: %v", err)
	}
	report, err := detector.DetectAndEnqueue(context.Background())
	if err != nil {
		t.Fatalf("DetectAndEnqueue returned error: %v", err)
	}
	if report.Candidates != 2 {
		t.Fatalf("expected 2 candidates, got=%d", report.Candidates)
	}
	if report.Enqueued != 1 {
		t.Fatalf("expected 1 enqueued request, got=%d", report.Enqueued)
	}
	if len(publisher.requests) != 1 {
		t.Fatalf("expected 1 published request, got=%d", len(publisher.requests))
	}
	if got := publisher.requests[0].GetProviderDeviceId(); got != "SN-1" {
		t.Fatalf("expected largest lag device SN-1 first, got=%s", got)
	}
}

func TestDetectorReturnsStoreError(t *testing.T) {
	t.Parallel()

	publisher := &fakeRequestPublisher{}
	detector, err := NewDetector(
		slog.Default(),
		fakeAssignmentStore{err: errors.New("db down")},
		fakeCoverageStore{},
		fakeSnapshotReader{},
		publisher,
		DetectorConfig{NowFn: time.Now, SubjectShardCount: 128, PollInterval: time.Second},
	)
	if err != nil {
		t.Fatalf("NewDetector returned error: %v", err)
	}
	if _, err := detector.DetectAndEnqueue(context.Background()); err == nil {
		t.Fatalf("expected detect error")
	}
}
