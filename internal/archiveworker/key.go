package archiveworker

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

var writerIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func envelopePartitionTime(env *envelopev1.TelemetryEnvelope, fallback time.Time) time.Time {
	if env == nil {
		return fallback.UTC()
	}
	ts := env.GetIngestedTimeUnixMs()
	if ts <= 0 {
		ts = env.GetObservedTimeUnixMs()
	}
	if ts <= 0 {
		ts = env.GetDeviceTimeUnixMs()
	}
	if ts <= 0 {
		return fallback.UTC()
	}
	return time.UnixMilli(ts).UTC()
}

func sanitizeWriterID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "writer"
	}
	raw = writerIDSanitizer.ReplaceAllString(raw, "-")
	raw = strings.Trim(raw, "-")
	if raw == "" {
		return "writer"
	}
	return raw
}

func buildArchiveObjectKey(prefix string, partition time.Time, shard uint32, part int, writerID string) string {
	partition = partition.UTC()
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = "raw"
	}
	if part <= 0 {
		part = 1
	}
	return fmt.Sprintf(
		"%s/yyyy=%04d/mm=%02d/dd=%02d/hh=%02d/shard=%03d/part-%05d-%s.pb.zst",
		prefix,
		partition.Year(),
		int(partition.Month()),
		partition.Day(),
		partition.Hour(),
		shard,
		part,
		sanitizeWriterID(writerID),
	)
}
