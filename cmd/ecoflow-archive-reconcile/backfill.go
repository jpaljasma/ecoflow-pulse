package main

import (
	"context"
	"fmt"
	"hash/crc32"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/archiveworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"google.golang.org/protobuf/proto"
)

const (
	backfillContentType = "application/x-protobuf+zstd"
	backfillCompression = "zstd"
	backfillProvider    = "unknown"
	backfillWriterID    = "archive-reconcile"
)

func missingObjectsFromDirect(directObjects []replaycli.ManifestObject, keys []string) ([]replaycli.ManifestObject, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	objectsByKey := make(map[string]replaycli.ManifestObject, len(directObjects))
	for _, object := range directObjects {
		composite := compositeObjectKey(object.ObjectBucket, object.ObjectKey)
		if composite == "" {
			continue
		}
		objectsByKey[composite] = object
	}
	out := make([]replaycli.ManifestObject, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		bucket, objectKey, err := splitCompositeObjectKey(key)
		if err != nil {
			return nil, err
		}
		composite := compositeObjectKey(bucket, objectKey)
		if _, exists := seen[composite]; exists {
			continue
		}
		object, ok := objectsByKey[composite]
		if !ok {
			return nil, fmt.Errorf("missing direct object metadata for %s", composite)
		}
		seen[composite] = struct{}{}
		out = append(out, object)
	}
	return out, nil
}

func backfillMissingManifestRows(
	ctx context.Context,
	reader replaycli.ObjectReader,
	store archiveworker.ManifestStore,
	objects []replaycli.ManifestObject,
	now time.Time,
) (int, error) {
	if len(objects) == 0 {
		return 0, nil
	}
	for i, object := range objects {
		record, err := buildManifestRecord(ctx, reader, object, now)
		if err != nil {
			return i, err
		}
		if err := store.UpsertObjectManifest(ctx, record); err != nil {
			return i, err
		}
	}
	return len(objects), nil
}

func buildManifestRecord(
	ctx context.Context,
	reader replaycli.ObjectReader,
	object replaycli.ManifestObject,
	now time.Time,
) (archiveworker.ManifestRecord, error) {
	body, err := reader.ReadObject(ctx, object.ObjectBucket, object.ObjectKey)
	if err != nil {
		return archiveworker.ManifestRecord{}, fmt.Errorf("read archive object %s/%s: %w", object.ObjectBucket, object.ObjectKey, err)
	}
	record, err := buildManifestRecordFromBody(object, body, now)
	if err != nil {
		return archiveworker.ManifestRecord{}, fmt.Errorf("build manifest record for %s/%s: %w", object.ObjectBucket, object.ObjectKey, err)
	}
	return record, nil
}

func buildManifestRecordFromBody(
	object replaycli.ManifestObject,
	body []byte,
	now time.Time,
) (archiveworker.ManifestRecord, error) {
	frames, err := replaycli.DecodeEnvelopeFrames(body)
	if err != nil {
		return archiveworker.ManifestRecord{}, err
	}
	if len(frames) == 0 {
		return archiveworker.ManifestRecord{}, fmt.Errorf("archive object has no envelope frames")
	}

	deviceIDs := make(map[string]struct{})
	providerDeviceIDs := make(map[string]struct{})
	providers := make(map[string]struct{})
	tsMin := int64(0)
	tsMax := int64(0)
	shardCount := object.ShardCount

	for _, frame := range frames {
		var env envelopev1.TelemetryEnvelope
		if err := proto.Unmarshal(frame, &env); err != nil {
			return archiveworker.ManifestRecord{}, fmt.Errorf("unmarshal telemetry envelope: %w", err)
		}
		recordTS := envelopeTimestampUnixMilli(&env, object.PartitionHour)
		if tsMin == 0 || recordTS < tsMin {
			tsMin = recordTS
		}
		if tsMax == 0 || recordTS > tsMax {
			tsMax = recordTS
		}
		if value := strings.TrimSpace(env.GetDeviceId()); value != "" {
			deviceIDs[value] = struct{}{}
		}
		if provider := strings.ToLower(strings.TrimSpace(envelopeProvider(&env))); provider != "" {
			providers[provider] = struct{}{}
		}
		if providerDeviceID := normalizeProviderDeviceID(envelopeProviderDeviceID(&env)); providerDeviceID != "" {
			providerDeviceIDs[providerDeviceID] = struct{}{}
		}
		if candidate := env.GetShardCount(); candidate > shardCount {
			shardCount = candidate
		}
	}

	if shardCount == 0 {
		shardCount = telemetrybus.DefaultShardCount
	}
	if object.Shard >= shardCount {
		shardCount = object.Shard + 1
	}

	partitionHour := object.PartitionHour.UTC().Truncate(time.Hour)
	if partitionHour.IsZero() {
		return archiveworker.ManifestRecord{}, fmt.Errorf("archive object partition hour is required")
	}
	writerID := parseWriterIDFromObjectKey(object.ObjectKey)
	sizeBytes := object.ObjectSizeBytes
	if sizeBytes <= 0 {
		sizeBytes = int64(len(body))
	}

	return archiveworker.ManifestRecord{
		Provider:          segmentProvider(providers),
		Shard:             object.Shard,
		ShardCount:        shardCount,
		PartitionHour:     partitionHour,
		TSMinUnixMS:       tsMin,
		TSMaxUnixMS:       tsMax,
		RecordCount:       len(frames),
		ObjectBucket:      strings.TrimSpace(object.ObjectBucket),
		ObjectKey:         strings.Trim(strings.TrimSpace(object.ObjectKey), "/"),
		ObjectSizeBytes:   sizeBytes,
		ContentType:       backfillContentType,
		Compression:       backfillCompression,
		ChecksumCRC32:     fmt.Sprintf("%08x", crc32.ChecksumIEEE(body)),
		WriterID:          writerID,
		DeviceIDs:         sortedKeys(deviceIDs),
		ProviderDeviceIDs: sortedKeys(providerDeviceIDs),
		CreatedAt:         now.UTC(),
		UpdatedAt:         now.UTC(),
	}, nil
}

func envelopeTimestampUnixMilli(env *envelopev1.TelemetryEnvelope, fallbackPartition time.Time) int64 {
	if env == nil {
		return fallbackPartition.UTC().UnixMilli()
	}
	if ts := env.GetIngestedTimeUnixMs(); ts > 0 {
		return ts
	}
	if ts := env.GetObservedTimeUnixMs(); ts > 0 {
		return ts
	}
	if ts := env.GetDeviceTimeUnixMs(); ts > 0 {
		return ts
	}
	return fallbackPartition.UTC().UnixMilli()
}

func envelopeProvider(env *envelopev1.TelemetryEnvelope) string {
	if env == nil {
		return ""
	}
	if labels := env.GetLabels(); len(labels) > 0 {
		return labels["provider"]
	}
	return ""
}

func envelopeProviderDeviceID(env *envelopev1.TelemetryEnvelope) string {
	if env == nil {
		return ""
	}
	if labels := env.GetLabels(); len(labels) > 0 {
		if value := strings.TrimSpace(labels["provider_device_id"]); value != "" {
			return value
		}
	}
	return env.GetEcoflowSn()
}

func normalizeProviderDeviceID(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func segmentProvider(providers map[string]struct{}) string {
	if len(providers) == 0 {
		return backfillProvider
	}
	if len(providers) == 1 {
		for provider := range providers {
			return provider
		}
	}
	return "mixed"
}

func sortedKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func parseWriterIDFromObjectKey(objectKey string) string {
	base := path.Base(strings.Trim(strings.TrimSpace(objectKey), "/"))
	if !strings.HasSuffix(base, ".pb.zst") {
		return backfillWriterID
	}
	base = strings.TrimSuffix(base, ".pb.zst")
	if !strings.HasPrefix(base, "part-") {
		return backfillWriterID
	}
	remainder := strings.TrimPrefix(base, "part-")
	index := strings.IndexByte(remainder, '-')
	if index < 0 || index == len(remainder)-1 {
		return backfillWriterID
	}
	partNumber := remainder[:index]
	if _, err := strconv.Atoi(partNumber); err != nil {
		return backfillWriterID
	}
	writerID := strings.TrimSpace(remainder[index+1:])
	if writerID == "" {
		return backfillWriterID
	}
	return writerID
}

func compositeObjectKey(bucket string, objectKey string) string {
	bucket = strings.TrimSpace(bucket)
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	if bucket == "" || objectKey == "" {
		return ""
	}
	return bucket + "|" + objectKey
}

func splitCompositeObjectKey(raw string) (string, string, error) {
	composite := strings.TrimSpace(raw)
	bucket, objectKey, ok := strings.Cut(composite, "|")
	bucket = strings.TrimSpace(bucket)
	objectKey = strings.Trim(strings.TrimSpace(objectKey), "/")
	if !ok || bucket == "" || objectKey == "" {
		return "", "", fmt.Errorf("invalid composite object key %q", raw)
	}
	return bucket, objectKey, nil
}
