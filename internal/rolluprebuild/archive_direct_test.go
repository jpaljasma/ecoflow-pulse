package rolluprebuild

import (
	"testing"
	"time"
)

func TestParseArchiveObjectKey(t *testing.T) {
	t.Parallel()

	prefix, partitionHour, shard, err := parseArchiveObjectKey("raw/yyyy=2026/mm=03/dd=14/hh=22/shard=043/part-00001-node.pb.zst")
	if err != nil {
		t.Fatalf("parseArchiveObjectKey failed: %v", err)
	}
	if prefix != "raw" {
		t.Fatalf("prefix mismatch: got=%s want=raw", prefix)
	}
	wantHour := time.Date(2026, time.March, 14, 22, 0, 0, 0, time.UTC)
	if !partitionHour.Equal(wantHour) {
		t.Fatalf("partition hour mismatch: got=%s want=%s", partitionHour, wantHour)
	}
	if shard != 43 {
		t.Fatalf("shard mismatch: got=%d want=43", shard)
	}
}
