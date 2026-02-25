package gaprepair

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	replayv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/replay/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

const (
	defaultDetectorPollInterval    = 30 * time.Second
	defaultDetectorPollJitter      = 0.20
	defaultDetectorLookbackWindow  = 30 * time.Minute
	defaultDetectorLagThreshold    = 90 * time.Second
	defaultDetectorWindowPadding   = 30 * time.Second
	defaultDetectorMaxReplayWindow = 30 * time.Minute
	defaultDetectorSafeDelay       = 10 * time.Second
	defaultDetectorMaxJobsPerCycle = 64
	defaultDetectorEvalWorkers     = 16
)

type plannedGapRepair struct {
	request *replayv1.GapRepairRequest
	lagMs   int64
}

type Detector struct {
	log       *slog.Logger
	store     AssignmentStore
	coverage  CoverageStore
	snapshots SnapshotReader
	publisher RequestPublisher
	cfg       DetectorConfig
	rng       *rand.Rand
}

func DefaultDetectorConfig() DetectorConfig {
	workers := runtime.GOMAXPROCS(0) * 2
	if workers < defaultDetectorEvalWorkers {
		workers = defaultDetectorEvalWorkers
	}
	if workers > 64 {
		workers = 64
	}
	return DetectorConfig{
		PollInterval:      defaultDetectorPollInterval,
		PollJitter:        defaultDetectorPollJitter,
		LookbackWindow:    defaultDetectorLookbackWindow,
		LagThreshold:      defaultDetectorLagThreshold,
		WindowPadding:     defaultDetectorWindowPadding,
		MaxReplayWindow:   defaultDetectorMaxReplayWindow,
		SafeDelay:         defaultDetectorSafeDelay,
		MaxObjectsPerJob:  0,
		MaxJobsPerCycle:   defaultDetectorMaxJobsPerCycle,
		EvaluationWorkers: workers,
		SubjectShardCount: telemetrybus.DefaultShardCount,
		NowFn:             time.Now,
	}
}

func NewDetector(
	log *slog.Logger,
	store AssignmentStore,
	coverage CoverageStore,
	snapshots SnapshotReader,
	publisher RequestPublisher,
	cfg DetectorConfig,
) (*Detector, error) {
	if log == nil {
		log = slog.Default()
	}
	if store == nil {
		return nil, errors.New("assignment store is required")
	}
	if coverage == nil {
		return nil, errors.New("coverage store is required")
	}
	if snapshots == nil {
		return nil, errors.New("snapshot reader is required")
	}
	if publisher == nil {
		return nil, errors.New("request publisher is required")
	}
	cfg = normalizeDetectorConfig(cfg)
	return &Detector{
		log:       log,
		store:     store,
		coverage:  coverage,
		snapshots: snapshots,
		publisher: publisher,
		cfg:       cfg,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func normalizeDetectorConfig(cfg DetectorConfig) DetectorConfig {
	out := cfg
	out.ProviderFilter = controlplane.NormalizeProvider(strings.TrimSpace(out.ProviderFilter))
	if out.PollInterval <= 0 {
		out.PollInterval = defaultDetectorPollInterval
	}
	if out.PollJitter < 0 {
		out.PollJitter = 0
	}
	if out.LookbackWindow <= 0 {
		out.LookbackWindow = defaultDetectorLookbackWindow
	}
	if out.LagThreshold <= 0 {
		out.LagThreshold = defaultDetectorLagThreshold
	}
	if out.WindowPadding < 0 {
		out.WindowPadding = 0
	}
	if out.MaxReplayWindow <= 0 {
		out.MaxReplayWindow = defaultDetectorMaxReplayWindow
	}
	if out.SafeDelay < 0 {
		out.SafeDelay = 0
	}
	if out.MaxJobsPerCycle <= 0 {
		out.MaxJobsPerCycle = defaultDetectorMaxJobsPerCycle
	}
	if out.EvaluationWorkers <= 0 {
		out.EvaluationWorkers = defaultDetectorEvalWorkers
	}
	if out.SubjectShardCount == 0 {
		out.SubjectShardCount = telemetrybus.DefaultShardCount
	}
	if out.NowFn == nil {
		out.NowFn = time.Now
	}
	return out
}

func (d *Detector) Run(ctx context.Context) error {
	if _, err := d.DetectAndEnqueue(ctx); err != nil {
		d.log.Warn("gap detector initial cycle failed", slog.String("error", err.Error()))
	}

	timer := time.NewTimer(d.nextPollInterval())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			if _, err := d.DetectAndEnqueue(ctx); err != nil {
				d.log.Warn("gap detector cycle failed", slog.String("error", err.Error()))
			}
			timer.Reset(d.nextPollInterval())
		}
	}
}

func (d *Detector) DetectAndEnqueue(ctx context.Context) (DetectorReport, error) {
	report := DetectorReport{}
	now := d.cfg.NowFn().UTC()
	nowUnixMS := now.UnixMilli()
	toUnixMS := now.Add(-d.cfg.SafeDelay).UnixMilli()
	if toUnixMS <= 0 || toUnixMS > nowUnixMS {
		toUnixMS = nowUnixMS
	}
	fromUnixMS := toUnixMS - d.cfg.LookbackWindow.Milliseconds()
	if fromUnixMS <= 0 {
		fromUnixMS = 1
	}

	assignments, err := d.store.ListIngestAssignments(ctx, controlplane.ListIngestAssignmentsInput{
		Provider:   d.cfg.ProviderFilter,
		ActiveOnly: true,
	})
	if err != nil {
		return report, err
	}
	report.Assignments = len(assignments)
	if len(assignments) == 0 {
		return report, nil
	}

	coverageByProvider, err := d.loadCoverage(ctx, assignments, fromUnixMS, toUnixMS)
	if err != nil {
		return report, err
	}

	plans := d.evaluateAssignments(ctx, assignments, coverageByProvider, nowUnixMS)
	report.Candidates = len(plans)
	if len(plans) == 0 {
		return report, nil
	}
	slices.SortFunc(plans, func(a, b plannedGapRepair) int {
		if a.lagMs > b.lagMs {
			return -1
		}
		if a.lagMs < b.lagMs {
			return 1
		}
		return strings.Compare(a.request.GetProviderDeviceId(), b.request.GetProviderDeviceId())
	})

	limit := d.cfg.MaxJobsPerCycle
	if limit > len(plans) {
		limit = len(plans)
	}
	report.Skipped = len(plans) - limit
	for i := 0; i < limit; i++ {
		plan := plans[i]
		if d.cfg.DryRun {
			d.log.Info("gap detector dry-run planned replay",
				slog.String("provider", plan.request.GetProvider()),
				slog.String("provider_device_id", plan.request.GetProviderDeviceId()),
				slog.Int64("from_unix_ms", plan.request.GetFromUnixMs()),
				slog.Int64("to_unix_ms", plan.request.GetToUnixMs()),
				slog.String("reason", plan.request.GetReason()),
				slog.Int64("lag_ms", plan.lagMs),
			)
			report.Enqueued++
			continue
		}
		if err := d.publisher.PublishGapRepair(ctx, plan.request); err != nil {
			report.Skipped++
			d.log.Warn("gap detector enqueue failed",
				slog.String("provider", plan.request.GetProvider()),
				slog.String("provider_device_id", plan.request.GetProviderDeviceId()),
				slog.String("error", err.Error()),
			)
			continue
		}
		report.Enqueued++
	}
	d.log.Info("gap detector cycle",
		slog.Int("assignments", report.Assignments),
		slog.Int("candidates", report.Candidates),
		slog.Int("enqueued", report.Enqueued),
		slog.Int("skipped", report.Skipped),
		slog.Int64("window_from_unix_ms", fromUnixMS),
		slog.Int64("window_to_unix_ms", toUnixMS),
	)
	return report, nil
}

func (d *Detector) loadCoverage(
	ctx context.Context,
	assignments []controlplane.IngestAssignment,
	fromUnixMS int64,
	toUnixMS int64,
) (map[string]map[string]CoverageWindow, error) {
	idsByProvider := make(map[string][]string, 4)
	for i := range assignments {
		a := sanitizeAssignment(assignments[i])
		if a.Provider == "" || a.ProviderDeviceID == "" {
			continue
		}
		idsByProvider[a.Provider] = append(idsByProvider[a.Provider], a.ProviderDeviceID)
	}

	out := make(map[string]map[string]CoverageWindow, len(idsByProvider))
	for provider, ids := range idsByProvider {
		coverage, err := d.coverage.CoverageByProviderDevices(ctx, provider, ids, fromUnixMS, toUnixMS)
		if err != nil {
			return nil, err
		}
		out[provider] = coverage
	}
	return out, nil
}

func (d *Detector) evaluateAssignments(
	ctx context.Context,
	assignments []controlplane.IngestAssignment,
	coverageByProvider map[string]map[string]CoverageWindow,
	nowUnixMS int64,
) []plannedGapRepair {
	workers := d.cfg.EvaluationWorkers
	if workers > len(assignments) {
		workers = len(assignments)
	}
	if workers <= 0 {
		workers = 1
	}
	jobs := make(chan controlplane.IngestAssignment, len(assignments))
	plans := make(chan plannedGapRepair, len(assignments))
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for a := range jobs {
				providerCoverage := coverageByProvider[a.Provider]
				coverage, ok := providerCoverage[a.ProviderDeviceID]
				if !ok {
					continue
				}
				snap, err := d.snapshots.ReadSnapshot(ctx, projectionworker.SnapshotIdentity{
					DeviceID:  a.DeviceID,
					EcoflowSN: a.ProviderDeviceID,
				})
				if err != nil {
					d.log.Warn("gap detector snapshot read failed",
						slog.String("provider", a.Provider),
						slog.String("provider_device_id", a.ProviderDeviceID),
						slog.String("error", err.Error()),
					)
					continue
				}
				cursorTS := int64(0)
				if snap != nil {
					cursorTS = snap.Cursor.TsUnixMs
				}
				plan, ok := d.planRepair(a, coverage, cursorTS, nowUnixMS)
				if ok {
					plans <- plan
				}
			}
		}()
	}
	for i := range assignments {
		jobs <- sanitizeAssignment(assignments[i])
	}
	close(jobs)
	wg.Wait()
	close(plans)

	out := make([]plannedGapRepair, 0, len(assignments))
	for plan := range plans {
		out = append(out, plan)
	}
	return out
}

func (d *Detector) planRepair(
	a controlplane.IngestAssignment,
	coverage CoverageWindow,
	cursorTS int64,
	nowUnixMS int64,
) (plannedGapRepair, bool) {
	if coverage.MaxUnixMS <= 0 || coverage.MinUnixMS <= 0 || coverage.MaxUnixMS < coverage.MinUnixMS {
		return plannedGapRepair{}, false
	}
	if cursorTS > 0 && coverage.MaxUnixMS <= cursorTS+d.cfg.LagThreshold.Milliseconds() {
		return plannedGapRepair{}, false
	}
	from := coverage.MinUnixMS
	reason := "snapshot_missing"
	if cursorTS > 0 {
		reason = "projection_lag"
		from = cursorTS - d.cfg.WindowPadding.Milliseconds()
	}
	if from < coverage.MinUnixMS {
		from = coverage.MinUnixMS
	}
	to := coverage.MaxUnixMS
	if to > nowUnixMS {
		to = nowUnixMS
	}
	if to <= from {
		return plannedGapRepair{}, false
	}
	if maxWindowMS := d.cfg.MaxReplayWindow.Milliseconds(); maxWindowMS > 0 && (to-from) > maxWindowMS {
		from = to - maxWindowMS
		if from < coverage.MinUnixMS {
			from = coverage.MinUnixMS
		}
		reason += "_window_clamped"
	}
	if to <= from {
		return plannedGapRepair{}, false
	}

	maxObjects := int32(0)
	if d.cfg.MaxObjectsPerJob > 0 {
		if d.cfg.MaxObjectsPerJob > math.MaxInt32 {
			maxObjects = math.MaxInt32
		} else {
			maxObjects = int32(d.cfg.MaxObjectsPerJob)
		}
	}
	requestID, err := uuid.NewV7()
	if err != nil {
		return plannedGapRepair{}, false
	}
	shard := telemetrybus.ShardForDevice(a.ProviderDeviceID, d.cfg.SubjectShardCount)
	request := &replayv1.GapRepairRequest{
		RequestId:        requestID.String(),
		Provider:         a.Provider,
		DeviceId:         a.DeviceID,
		ProviderDeviceId: a.ProviderDeviceID,
		Shard:            shard,
		ShardCount:       d.cfg.SubjectShardCount,
		FromUnixMs:       from,
		ToUnixMs:         to,
		Reason:           reason,
		DetectedAtUnixMs: nowUnixMS,
		MaxObjects:       maxObjects,
	}
	lagMS := to - cursorTS
	if cursorTS <= 0 {
		lagMS = to - from
	}
	if lagMS < 0 {
		lagMS = 0
	}
	return plannedGapRepair{request: request, lagMs: lagMS}, true
}

func (d *Detector) nextPollInterval() time.Duration {
	if d.cfg.PollJitter <= 0 {
		return d.cfg.PollInterval
	}
	base := float64(d.cfg.PollInterval)
	jitterRange := base * d.cfg.PollJitter
	if jitterRange <= 0 {
		return d.cfg.PollInterval
	}
	jitter := (d.rng.Float64()*2 - 1) * jitterRange
	next := time.Duration(base + jitter)
	if next < time.Second {
		return time.Second
	}
	return next
}

func sanitizeAssignment(a controlplane.IngestAssignment) controlplane.IngestAssignment {
	a.Provider = controlplane.NormalizeProvider(a.Provider)
	a.ProviderDeviceID = strings.ToUpper(strings.TrimSpace(a.ProviderDeviceID))
	a.DeviceID = strings.TrimSpace(a.DeviceID)
	return a
}
