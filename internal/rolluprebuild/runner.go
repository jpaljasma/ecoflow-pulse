package rolluprebuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/envelopededup"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
	"google.golang.org/protobuf/proto"
)

const (
	defaultObjectReadTimeout = 30 * time.Second
	progressLogEveryObjects  = 100
	defaultParallelWorkers   = 4
)

type Runner struct {
	log          *slog.Logger
	manifest     replaycli.ManifestStore
	objectReader replaycli.ObjectReader
	writer       *PostgresWriter
	chunkSize    int
	parallelism  int
}

type Report struct {
	ObjectsMatched   int
	ObjectsProcessed int
	MissingObjects   int
	ObjectBytes      int64
	ObjectRecords    int
	MessagesDecoded  int
	MessagesApplied  int
	QuotaMessages    int
	MinuteRows       int
	HourRows         int
	DayRows          int
	StartedAt        time.Time
	FinishedAt       time.Time
}

func NewRunner(log *slog.Logger, manifest replaycli.ManifestStore, objectReader replaycli.ObjectReader, writer *PostgresWriter, chunkSize int, parallelism int) (*Runner, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest store is required")
	}
	if objectReader == nil {
		return nil, fmt.Errorf("object reader is required")
	}
	if writer == nil {
		return nil, fmt.Errorf("postgres writer is required")
	}
	if log == nil {
		log = slog.Default()
	}
	if chunkSize <= 0 {
		chunkSize = defaultReplaceChunkSize
	}
	if parallelism <= 0 {
		parallelism = minPositive(runtime.GOMAXPROCS(0), defaultParallelWorkers)
	}
	return &Runner{
		log:          log,
		manifest:     manifest,
		objectReader: objectReader,
		writer:       writer,
		chunkSize:    chunkSize,
		parallelism:  parallelism,
	}, nil
}

func (r *Runner) Close() error {
	var firstErr error
	if r.writer != nil {
		if err := r.writer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.objectReader != nil {
		if err := r.objectReader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.manifest != nil {
		if err := r.manifest.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *Runner) RebuildFleet(ctx context.Context, query replaycli.FleetQuery) (Report, error) {
	objects, err := r.manifest.ListByFleetRange(ctx, query)
	if err != nil {
		return Report{}, fmt.Errorf("query manifests for rollup rebuild: %w", err)
	}
	objects = dedupeManifestObjects(objects)
	return r.rebuildObjects(ctx, objects, query.FromUnixMS, query.ToUnixMS)
}

func (r *Runner) RebuildObjects(ctx context.Context, objects []replaycli.ManifestObject, fromUnixMS, toUnixMS int64) (Report, error) {
	objects = dedupeManifestObjects(objects)
	return r.rebuildObjects(ctx, objects, fromUnixMS, toUnixMS)
}

func (r *Runner) RebuildDevices(ctx context.Context, query replaycli.DeviceQuery) (Report, error) {
	objects, err := r.manifest.ListByDevices(ctx, query)
	if err != nil {
		return Report{}, fmt.Errorf("query manifests for device rollup rebuild: %w", err)
	}
	objects = dedupeManifestObjects(objects)
	return r.rebuildObjects(ctx, objects, query.FromUnixMS, query.ToUnixMS)
}

func (r *Runner) rebuildObjects(ctx context.Context, objects []replaycli.ManifestObject, fromUnixMS, toUnixMS int64) (Report, error) {
	report := Report{
		StartedAt:      time.Now().UTC(),
		ObjectsMatched: len(objects),
	}
	for _, object := range objects {
		report.ObjectBytes += object.ObjectSizeBytes
		report.ObjectRecords += object.RecordCount
	}
	if len(objects) == 0 {
		report.FinishedAt = time.Now().UTC()
		return report, nil
	}

	groups := groupObjectsByShard(objects)
	results := make(chan shardResult, len(groups))
	sema := make(chan struct{}, maxPositive(1, r.parallelism))
	var wg sync.WaitGroup
	var objectsProcessed atomic.Int64
	var missingObjects atomic.Int64
	var messagesDecoded atomic.Int64
	var messagesApplied atomic.Int64
	var quotaMessages atomic.Int64

	for _, group := range groups {
		group := group
		wg.Add(1)
		go func() {
			defer wg.Done()
			sema <- struct{}{}
			defer func() { <-sema }()
			result := r.processObjectGroup(ctx, group, &objectsProcessed, &missingObjects, report.ObjectsMatched, &messagesDecoded, &messagesApplied, &quotaMessages, toUnixMS)
			results <- result
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	minuteRowsAll := make([]BucketRow, 0, 1024)
	hourRowsAll := make([]BucketRow, 0, 256)
	dayRowsAll := make([]BucketRow, 0, 64)
	affected := make([]DeviceWindow, 0, len(objects))
	for result := range results {
		if result.err != nil {
			return report, result.err
		}
		report.ObjectsProcessed += result.objectsProcessed
		report.MissingObjects += result.missingObjects
		report.MessagesDecoded += result.messagesDecoded
		report.MessagesApplied += result.messagesApplied
		report.QuotaMessages += result.quotaMessages
		affected = append(affected, result.affected...)
		minuteRowsAll = append(minuteRowsAll, result.minuteRows...)
		hourRowsAll = append(hourRowsAll, result.hourRows...)
		dayRowsAll = append(dayRowsAll, result.dayRows...)
	}
	if report.MissingObjects > 0 {
		return report, fmt.Errorf("refusing rollup replacement with %d missing archive objects; repair archive coverage or rebuild from raw logs", report.MissingObjects)
	}

	from := time.UnixMilli(fromUnixMS).UTC()
	to := time.UnixMilli(toUnixMS).UTC()
	minuteRowsAll = filterRowsForWindow(minuteRowsAll, ResolutionMinute, from, to)
	hourRowsAll = filterRowsForWindow(hourRowsAll, ResolutionHour, from, to)
	dayRowsAll = filterRowsForWindow(dayRowsAll, ResolutionDay, from, to)
	var err error
	if report.MinuteRows, err = r.writer.ReplaceRows(ctx, ResolutionMinute, minuteRowsAll, affected, from, to, r.chunkSize); err != nil {
		return report, err
	}
	if report.HourRows, err = r.writer.ReplaceRows(ctx, ResolutionHour, hourRowsAll, affected, from, to, r.chunkSize); err != nil {
		return report, err
	}
	if report.DayRows, err = r.writer.ReplaceRows(ctx, ResolutionDay, dayRowsAll, affected, from, to, r.chunkSize); err != nil {
		return report, err
	}
	report.FinishedAt = time.Now().UTC()
	return report, nil
}

func collectAffectedDevices(objects []replaycli.ManifestObject) []DeviceWindow {
	out := make([]DeviceWindow, 0, len(objects))
	for _, object := range objects {
		for _, providerDeviceID := range object.ProviderDeviceIDs {
			out = append(out, DeviceWindow{
				Provider:         object.Provider,
				ProviderDeviceID: providerDeviceID,
			})
		}
	}
	return out
}

type shardResult struct {
	objectsProcessed int
	missingObjects   int
	messagesDecoded  int
	messagesApplied  int
	quotaMessages    int
	minuteRows       []BucketRow
	hourRows         []BucketRow
	dayRows          []BucketRow
	affected         []DeviceWindow
	err              error
}

func (r *Runner) processObjectGroup(
	ctx context.Context,
	objects []replaycli.ManifestObject,
	objectsProcessed *atomic.Int64,
	missingObjects *atomic.Int64,
	objectsMatched int,
	messagesDecoded *atomic.Int64,
	messagesApplied *atomic.Int64,
	quotaMessages *atomic.Int64,
	toUnixMS int64,
) shardResult {
	aggregator := NewAggregator()
	result := shardResult{
		affected: collectAffectedDevices(objects),
	}
	affectedSeen := make(map[string]struct{}, len(result.affected))
	seenEnvelopeKeys := make(map[string]struct{}, 4096)
	for _, device := range result.affected {
		affectedSeen[device.Provider+"|"+device.ProviderDeviceID] = struct{}{}
	}
	for _, object := range objects {
		objectCtx, cancel := context.WithTimeout(ctx, defaultObjectReadTimeout)
		body, err := r.objectReader.ReadObject(objectCtx, object.ObjectBucket, object.ObjectKey)
		cancel()
		if err != nil {
			if isMissingArchiveObjectError(err) {
				result.missingObjects++
				missingObjects.Add(1)
				r.log.Warn(
					"missing archive object blocks rollup rebuild",
					slog.String("bucket", object.ObjectBucket),
					slog.String("key", object.ObjectKey),
					slog.String("error", err.Error()),
				)
				continue
			}
			result.err = fmt.Errorf("read archive object %s/%s: %w", object.ObjectBucket, object.ObjectKey, err)
			return result
		}
		result.objectsProcessed++
		processed := objectsProcessed.Add(1)
		frames, err := replaycli.DecodeEnvelopeFrames(body)
		if err != nil {
			result.err = fmt.Errorf("decode archive object %s/%s: %w", object.ObjectBucket, object.ObjectKey, err)
			return result
		}
		for _, frame := range frames {
			if err := ctx.Err(); err != nil {
				result.err = err
				return result
			}
			result.messagesDecoded++
			messagesDecoded.Add(1)
			var env envelopev1.TelemetryEnvelope
			if err := proto.Unmarshal(frame, &env); err != nil {
				result.err = fmt.Errorf("unmarshal archived envelope: %w", err)
				return result
			}
			if dedupKey := strings.TrimSpace(envelopededup.Key(&env)); dedupKey != "" {
				if _, exists := seenEnvelopeKeys[dedupKey]; exists {
					continue
				}
				seenEnvelopeKeys[dedupKey] = struct{}{}
			}
			if env.GetSourceKind() == envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA {
				result.quotaMessages++
				quotaMessages.Add(1)
			}
			sample, err := rollupworker.SampleFromEnvelope(&env)
			if err == nil {
				aggregator.ApplySample(sample)
				device := DeviceWindow{
					Provider:         sample.Provider,
					ProviderDeviceID: sample.ProviderDeviceID,
				}
				key := device.Provider + "|" + device.ProviderDeviceID
				if _, exists := affectedSeen[key]; !exists && strings.TrimSpace(device.ProviderDeviceID) != "" {
					affectedSeen[key] = struct{}{}
					result.affected = append(result.affected, device)
				}
				result.messagesApplied++
				messagesApplied.Add(1)
			} else if err != rollupworker.ErrNoRollupMetrics {
				result.err = fmt.Errorf("derive rollup sample from envelope %s: %w", env.GetEnvelopeId(), err)
				return result
			}
		}
		if processed%progressLogEveryObjects == 0 {
			r.log.Info("rollup rebuild progress",
				slog.Int64("objects_processed", processed),
				slog.Int("objects_matched", objectsMatched),
				slog.Int64("messages_decoded", messagesDecoded.Load()),
				slog.Int64("messages_applied", messagesApplied.Load()),
			)
		}
	}
	aggregator.Finalize(time.UnixMilli(toUnixMS).UTC())
	result.affected = normalizeAffectedDevices(result.affected)
	result.minuteRows = aggregator.Rows(ResolutionMinute)
	result.hourRows = aggregator.Rows(ResolutionHour)
	result.dayRows = aggregator.Rows(ResolutionDay)
	return result
}

func isMissingArchiveObjectError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return !errors.Is(err, context.Canceled) &&
		(strings.Contains(message, "specified key does not exist") ||
			strings.Contains(message, "no such key") ||
			strings.Contains(message, "not found"))
}

func groupObjectsByShard(objects []replaycli.ManifestObject) [][]replaycli.ManifestObject {
	if len(objects) == 0 {
		return nil
	}
	byShard := make(map[uint32][]replaycli.ManifestObject)
	keys := make([]uint32, 0, len(objects))
	seen := make(map[uint32]struct{})
	for _, object := range objects {
		if _, ok := seen[object.Shard]; !ok {
			seen[object.Shard] = struct{}{}
			keys = append(keys, object.Shard)
		}
		byShard[object.Shard] = append(byShard[object.Shard], object)
	}
	// shard order only matters for deterministic result merging; per-shard object
	// order is preserved from the manifest query.
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([][]replaycli.ManifestObject, 0, len(keys))
	for _, key := range keys {
		out = append(out, byShard[key])
	}
	return out
}

func dedupeManifestObjects(objects []replaycli.ManifestObject) []replaycli.ManifestObject {
	if len(objects) < 2 {
		return objects
	}
	seen := make(map[string]struct{}, len(objects))
	out := make([]replaycli.ManifestObject, 0, len(objects))
	for _, object := range objects {
		key := strings.TrimSpace(object.ObjectBucket) + "|" + strings.Trim(strings.TrimSpace(object.ObjectKey), "/")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, object)
	}
	return out
}

func minPositive(a, b int) int {
	switch {
	case a <= 0:
		return b
	case b <= 0:
		return a
	case a < b:
		return a
	default:
		return b
	}
}

func maxPositive(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func filterRowsForWindow(rows []BucketRow, resolution Resolution, from, to time.Time) []BucketRow {
	if len(rows) == 0 {
		return nil
	}
	start, end := replacementWindowBounds(resolution, from, to)
	out := make([]BucketRow, 0, len(rows))
	for _, row := range rows {
		if row.BucketStart.Before(start) || !row.BucketStart.Before(end) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func replacementWindowBounds(resolution Resolution, from, to time.Time) (time.Time, time.Time) {
	from = from.UTC()
	to = to.UTC()
	switch resolution {
	case ResolutionMinute:
		return from.Truncate(time.Minute), to
	case ResolutionHour:
		return from.Truncate(time.Hour), to.Truncate(time.Hour).Add(time.Hour)
	case ResolutionDay:
		start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
		end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
		return start, end
	default:
		return from, to
	}
}
