package gaprepair

import (
	"context"
	"time"

	replayv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/replay/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
)

type AssignmentStore interface {
	ListIngestAssignments(ctx context.Context, in controlplane.ListIngestAssignmentsInput) ([]controlplane.IngestAssignment, error)
}

type SnapshotReader interface {
	ReadSnapshot(ctx context.Context, identity projectionworker.SnapshotIdentity) (*projectionworker.SnapshotReadModel, error)
}

type CoverageWindow struct {
	ProviderDeviceID string
	MinUnixMS        int64
	MaxUnixMS        int64
	ObjectCount      int
}

type CoverageStore interface {
	CoverageByProviderDevices(
		ctx context.Context,
		provider string,
		providerDeviceIDs []string,
		fromUnixMS int64,
		toUnixMS int64,
	) (map[string]CoverageWindow, error)
	Close() error
}

type RequestPublisher interface {
	PublishGapRepair(ctx context.Context, req *replayv1.GapRepairRequest) error
	Close() error
}

type ReplayRunner interface {
	ReplayDevices(ctx context.Context, request replaycli.ReplayRequest) (replaycli.ReplayReport, error)
}

type DetectorConfig struct {
	ProviderFilter    string
	PollInterval      time.Duration
	PollJitter        float64
	LookbackWindow    time.Duration
	LagThreshold      time.Duration
	WindowPadding     time.Duration
	MaxReplayWindow   time.Duration
	SafeDelay         time.Duration
	MaxObjectsPerJob  int
	MaxJobsPerCycle   int
	EvaluationWorkers int
	SubjectShardCount uint32
	DryRun            bool
	NowFn             func() time.Time
}

type DetectorReport struct {
	Assignments int
	Candidates  int
	Enqueued    int
	Skipped     int
}

type WorkerConfig struct {
	StreamName        string
	QueueGroup        string
	Durable           string
	AckWait           time.Duration
	MaxAckPending     int
	ProcessTimeout    time.Duration
	DrainTimeout      time.Duration
	DefaultMaxObjects int
}
