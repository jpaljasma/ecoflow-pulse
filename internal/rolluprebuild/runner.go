package rolluprebuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
	"github.com/tidwall/gjson"
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
	PVPortMinuteRows int
	PVPortHourRows   int
	PVPortDayRows    int
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

	orderedObjects := orderObjectsForRebuild(objects)
	var objectsProcessed atomic.Int64
	var missingObjects atomic.Int64
	var messagesDecoded atomic.Int64
	var messagesApplied atomic.Int64
	var quotaMessages atomic.Int64

	minuteRowsAll := make([]BucketRow, 0, 1024)
	hourRowsAll := make([]BucketRow, 0, 256)
	dayRowsAll := make([]BucketRow, 0, 64)
	pvPortMinuteRowsAll := make([]PVPortBucketRow, 0, 256)
	pvPortHourRowsAll := make([]PVPortBucketRow, 0, 128)
	pvPortDayRowsAll := make([]PVPortBucketRow, 0, 64)
	affected := make([]DeviceWindow, 0, len(objects))
	result := r.processObjectGroup(ctx, orderedObjects, &objectsProcessed, &missingObjects, report.ObjectsMatched, &messagesDecoded, &messagesApplied, &quotaMessages, toUnixMS)
	if result.err != nil {
		return report, result.err
	}
	report.ObjectsProcessed = result.objectsProcessed
	report.MissingObjects = result.missingObjects
	report.MessagesDecoded = result.messagesDecoded
	report.MessagesApplied = result.messagesApplied
	report.QuotaMessages = result.quotaMessages
	affected = append(affected, result.affected...)
	minuteRowsAll = append(minuteRowsAll, result.minuteRows...)
	hourRowsAll = append(hourRowsAll, result.hourRows...)
	dayRowsAll = append(dayRowsAll, result.dayRows...)
	pvPortMinuteRowsAll = append(pvPortMinuteRowsAll, result.pvPortMinuteRows...)
	pvPortHourRowsAll = append(pvPortHourRowsAll, result.pvPortHourRows...)
	pvPortDayRowsAll = append(pvPortDayRowsAll, result.pvPortDayRows...)
	if report.MissingObjects > 0 {
		return report, fmt.Errorf("refusing rollup replacement with %d missing archive objects; repair archive coverage or rebuild from raw logs", report.MissingObjects)
	}

	from := time.UnixMilli(fromUnixMS).UTC()
	to := time.UnixMilli(toUnixMS).UTC()
	minuteRowsAll = filterRowsForWindow(minuteRowsAll, ResolutionMinute, from, to)
	hourRowsAll = filterRowsForWindow(hourRowsAll, ResolutionHour, from, to)
	dayRowsAll = filterRowsForWindow(dayRowsAll, ResolutionDay, from, to)
	pvPortMinuteRowsAll = filterPVPortRowsForWindow(pvPortMinuteRowsAll, ResolutionMinute, from, to)
	pvPortHourRowsAll = filterPVPortRowsForWindow(pvPortHourRowsAll, ResolutionHour, from, to)
	pvPortDayRowsAll = filterPVPortRowsForWindow(pvPortDayRowsAll, ResolutionDay, from, to)
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
	if report.PVPortMinuteRows, err = r.writer.ReplacePVPortRows(ctx, ResolutionMinute, pvPortMinuteRowsAll, affected, from, to, r.chunkSize); err != nil {
		return report, err
	}
	if report.PVPortHourRows, err = r.writer.ReplacePVPortRows(ctx, ResolutionHour, pvPortHourRowsAll, affected, from, to, r.chunkSize); err != nil {
		return report, err
	}
	if report.PVPortDayRows, err = r.writer.ReplacePVPortRows(ctx, ResolutionDay, pvPortDayRowsAll, affected, from, to, r.chunkSize); err != nil {
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
	pvPortMinuteRows []PVPortBucketRow
	pvPortHourRows   []PVPortBucketRow
	pvPortDayRows    []PVPortBucketRow
	affected         []DeviceWindow
	err              error
}

type rebuildSample struct {
	sample *rollupworker.RollupSample
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
	samples := make([]rebuildSample, 0, 4096)
	dedupedEnvelopes := make([]*envelopev1.TelemetryEnvelope, 0, 4096)
	dedupedEnvelopeIndex := make(map[string]int, 4096)
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
			env := &envelopev1.TelemetryEnvelope{}
			if err := proto.Unmarshal(frame, env); err != nil {
				result.err = fmt.Errorf("unmarshal archived envelope: %w", err)
				return result
			}
			if dedupKey := rebuildEnvelopeKey(env); dedupKey != "" {
				if idx, exists := dedupedEnvelopeIndex[dedupKey]; exists {
					if shouldPreferLatestEnvelope(dedupedEnvelopes[idx], env) {
						dedupedEnvelopes[idx] = env
					}
					continue
				}
				dedupedEnvelopeIndex[dedupKey] = len(dedupedEnvelopes)
			}
			dedupedEnvelopes = append(dedupedEnvelopes, env)
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
	for _, env := range dedupedEnvelopes {
		if env == nil {
			continue
		}
		if env.GetSourceKind() == envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA {
			result.quotaMessages++
			quotaMessages.Add(1)
		}
		sample, err := rollupworker.SampleFromEnvelope(env)
		if err == nil {
			samples = append(samples, rebuildSample{sample: sample})
			device := DeviceWindow{
				Provider:         sample.Provider,
				ProviderDeviceID: sample.ProviderDeviceID,
			}
			key := device.Provider + "|" + device.ProviderDeviceID
			if _, exists := affectedSeen[key]; !exists && strings.TrimSpace(device.ProviderDeviceID) != "" {
				affectedSeen[key] = struct{}{}
				result.affected = append(result.affected, device)
			}
		} else if err != rollupworker.ErrNoRollupMetrics {
			result.err = fmt.Errorf("derive rollup sample from envelope %s: %w", env.GetEnvelopeId(), err)
			return result
		}
	}
	sort.SliceStable(samples, func(i, j int) bool {
		return rebuildSampleLess(samples[i].sample, samples[j].sample)
	})
	for _, entry := range samples {
		aggregator.ApplySample(entry.sample)
		result.messagesApplied++
		messagesApplied.Add(1)
	}
	result.affected = normalizeAffectedDevices(result.affected)
	result.minuteRows = aggregator.Rows(ResolutionMinute)
	result.hourRows = aggregator.Rows(ResolutionHour)
	result.dayRows = aggregator.Rows(ResolutionDay)
	result.pvPortMinuteRows = aggregator.PVPortRows(ResolutionMinute)
	result.pvPortHourRows = aggregator.PVPortRows(ResolutionHour)
	result.pvPortDayRows = aggregator.PVPortRows(ResolutionDay)
	return result
}

func rebuildEnvelopeKey(env *envelopev1.TelemetryEnvelope) string {
	if env == nil {
		return ""
	}
	deviceID := strings.TrimSpace(env.GetDeviceId())
	if deviceID != "" && env.GetObservedTimeUnixMs() > 0 {
		if payloadKey := rebuildPayloadKey(env.GetPayload()); payloadKey != "" {
			return fmt.Sprintf(
				"sample:%s:%d:%d:%s",
				deviceID,
				env.GetObservedTimeUnixMs(),
				env.GetSourceKind(),
				payloadKey,
			)
		}
	}
	messageID := strings.TrimSpace(env.GetMessageId())
	if deviceID != "" && messageID != "" {
		return fmt.Sprintf("msg:%s:%s", deviceID, messageID)
	}
	if envelopeID := strings.TrimSpace(env.GetEnvelopeId()); envelopeID != "" {
		return "env:" + envelopeID
	}
	return ""
}

func rebuildPayloadKey(payload []byte) string {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return ""
	}
	typeCode := strings.TrimSpace(gjson.GetBytes(payload, "typeCode").String())
	addr := strings.TrimSpace(gjson.GetBytes(payload, "addr").String())
	cmdID := strings.TrimSpace(gjson.GetBytes(payload, "cmdId").Raw)
	cmdFunc := strings.TrimSpace(gjson.GetBytes(payload, "cmdFunc").Raw)
	if typeCode == "" && addr == "" && cmdID == "" && cmdFunc == "" {
		return ""
	}
	return strings.Join([]string{typeCode, addr, cmdID, cmdFunc}, "|")
}

func shouldPreferLatestEnvelope(existing *envelopev1.TelemetryEnvelope, candidate *envelopev1.TelemetryEnvelope) bool {
	if existing == nil {
		return candidate != nil
	}
	if candidate == nil {
		return false
	}
	switch {
	case candidate.GetIngestedTimeUnixMs() != existing.GetIngestedTimeUnixMs():
		return candidate.GetIngestedTimeUnixMs() > existing.GetIngestedTimeUnixMs()
	case candidate.GetObservedTimeUnixMs() != existing.GetObservedTimeUnixMs():
		return candidate.GetObservedTimeUnixMs() > existing.GetObservedTimeUnixMs()
	case candidate.GetDeviceTimeUnixMs() != existing.GetDeviceTimeUnixMs():
		return candidate.GetDeviceTimeUnixMs() > existing.GetDeviceTimeUnixMs()
	default:
		return candidate.GetEnvelopeId() > existing.GetEnvelopeId()
	}
}

func rebuildSampleLess(left *rollupworker.RollupSample, right *rollupworker.RollupSample) bool {
	switch {
	case left == nil:
		return right != nil
	case right == nil:
		return false
	case left.IngestedUnixMs != right.IngestedUnixMs:
		return left.IngestedUnixMs < right.IngestedUnixMs
	case left.EventUnixMs != right.EventUnixMs:
		return left.EventUnixMs < right.EventUnixMs
	case left.Provider != right.Provider:
		return left.Provider < right.Provider
	case left.ProviderDeviceID != right.ProviderDeviceID:
		return left.ProviderDeviceID < right.ProviderDeviceID
	case left.DeviceID != right.DeviceID:
		return left.DeviceID < right.DeviceID
	default:
		return false
	}
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

func orderObjectsForRebuild(objects []replaycli.ManifestObject) []replaycli.ManifestObject {
	if len(objects) < 2 {
		return objects
	}
	out := append([]replaycli.ManifestObject(nil), objects...)
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		switch {
		case left.TSMinUnixMS != right.TSMinUnixMS:
			return left.TSMinUnixMS < right.TSMinUnixMS
		case left.TSMaxUnixMS != right.TSMaxUnixMS:
			return left.TSMaxUnixMS < right.TSMaxUnixMS
		case left.PartitionHour != right.PartitionHour:
			return left.PartitionHour.Before(right.PartitionHour)
		case left.Shard != right.Shard:
			return left.Shard < right.Shard
		case left.ObjectBucket != right.ObjectBucket:
			return left.ObjectBucket < right.ObjectBucket
		default:
			return left.ObjectKey < right.ObjectKey
		}
	})
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

func filterPVPortRowsForWindow(rows []PVPortBucketRow, resolution Resolution, from, to time.Time) []PVPortBucketRow {
	if len(rows) == 0 {
		return nil
	}
	start, end := replacementWindowBounds(resolution, from, to)
	out := make([]PVPortBucketRow, 0, len(rows))
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
