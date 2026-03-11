package telemetrybus

import "testing"

func TestShardForDeviceDeterministic(t *testing.T) {
	t.Parallel()

	const shards = 128
	const device = "DEMOD2M00001057"

	a := ShardForDevice(device, shards)
	b := ShardForDevice(device, shards)
	if a != b {
		t.Fatalf("expected stable shard mapping; got %d and %d", a, b)
	}
	if a >= shards {
		t.Fatalf("expected shard in range [0,%d); got %d", shards, a)
	}
}

func TestShardForDeviceUsesDefaultShardCount(t *testing.T) {
	t.Parallel()

	shard := ShardForDevice("DEMODPU0000294", 0)
	if shard >= DefaultShardCount {
		t.Fatalf("expected shard in range [0,%d); got %d", DefaultShardCount, shard)
	}
}

func TestSubjectConfigNormalizedDefaults(t *testing.T) {
	t.Parallel()

	cfg := (SubjectConfig{}).Normalized()
	if cfg.Prefix != DefaultSubjectPrefix {
		t.Fatalf("expected default prefix %q, got %q", DefaultSubjectPrefix, cfg.Prefix)
	}
	if cfg.ShardCount != DefaultShardCount {
		t.Fatalf("expected default shard count %d, got %d", DefaultShardCount, cfg.ShardCount)
	}
}

func TestSubjectsFormat(t *testing.T) {
	t.Parallel()

	cfg := SubjectConfig{Prefix: "pulse", ShardCount: 512}
	shard := uint32(7)

	cases := map[string]string{
		"ingest":     IngestSubject(cfg, shard),
		"projection": ProjectionSubject(cfg, shard),
		"archive":    ArchiveSubject(cfg, shard),
		"replay":     ReplaySubject(cfg, shard),
		"gaprepair":  GapRepairSubject(cfg, shard),
	}

	expected := map[string]string{
		"ingest":     "pulse.telemetry.ingest.s007",
		"projection": "pulse.telemetry.projection.s007",
		"archive":    "pulse.telemetry.archive.s007",
		"replay":     "pulse.telemetry.replay.s007",
		"gaprepair":  "pulse.telemetry.gaprepair.s007",
	}

	for name, got := range cases {
		if got != expected[name] {
			t.Fatalf("%s: expected %q, got %q", name, expected[name], got)
		}
	}
}
