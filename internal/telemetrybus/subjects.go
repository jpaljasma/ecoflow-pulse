package telemetrybus

import (
	"fmt"
	"hash/fnv"
)

const (
	DefaultSubjectPrefix = "pulse"
	DefaultShardCount    = 128
)

// SubjectConfig defines naming/sharding rules for JetStream subjects.
type SubjectConfig struct {
	Prefix     string
	ShardCount uint32
}

// Normalized returns config with safe defaults.
func (c SubjectConfig) Normalized() SubjectConfig {
	cfg := c
	if cfg.Prefix == "" {
		cfg.Prefix = DefaultSubjectPrefix
	}
	if cfg.ShardCount == 0 {
		cfg.ShardCount = DefaultShardCount
	}
	return cfg
}

// ShardForDevice deterministically maps a device id to shard index [0, shardCount).
func ShardForDevice(deviceID string, shardCount uint32) uint32 {
	if shardCount == 0 {
		shardCount = DefaultShardCount
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(deviceID))
	return hasher.Sum32() % shardCount
}

// IngestSubject receives normalized ingest envelopes.
func IngestSubject(cfg SubjectConfig, shard uint32) string {
	cfg = cfg.Normalized()
	return fmt.Sprintf("%s.telemetry.ingest.s%03d", cfg.Prefix, shard)
}

// ProjectionSubject is consumed by projection workers to build live snapshots.
func ProjectionSubject(cfg SubjectConfig, shard uint32) string {
	cfg = cfg.Normalized()
	return fmt.Sprintf("%s.telemetry.projection.s%03d", cfg.Prefix, shard)
}

// ArchiveSubject is consumed by archive writers.
func ArchiveSubject(cfg SubjectConfig, shard uint32) string {
	cfg = cfg.Normalized()
	return fmt.Sprintf("%s.telemetry.archive.s%03d", cfg.Prefix, shard)
}

// ReplaySubject is used for replay stream fanout.
func ReplaySubject(cfg SubjectConfig, shard uint32) string {
	cfg = cfg.Normalized()
	return fmt.Sprintf("%s.telemetry.replay.s%03d", cfg.Prefix, shard)
}

// GapRepairSubject queues targeted repair requests.
func GapRepairSubject(cfg SubjectConfig, shard uint32) string {
	cfg = cfg.Normalized()
	return fmt.Sprintf("%s.telemetry.gaprepair.s%03d", cfg.Prefix, shard)
}
