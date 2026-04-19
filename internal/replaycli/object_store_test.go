package replaycli

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

func TestArchiveListingHoursExcludesExactEndHour(t *testing.T) {
	t.Parallel()

	got := archiveListingHours(
		time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC),
	)
	if len(got) == 0 {
		t.Fatalf("archiveListingHours() returned no hours")
	}
	wantLast := time.Date(2026, time.April, 14, 23, 0, 0, 0, time.UTC)
	if !got[len(got)-1].Equal(wantLast) {
		t.Fatalf("last hour mismatch: got=%s want=%s", got[len(got)-1], wantLast)
	}
}

func TestArchiveListingHoursIncludesPartialEndHour(t *testing.T) {
	t.Parallel()

	got := archiveListingHours(
		time.Date(2026, time.March, 20, 0, 15, 0, 0, time.UTC),
		time.Date(2026, time.March, 20, 2, 30, 0, 0, time.UTC),
	)
	want := []time.Time{
		time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 20, 1, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 20, 2, 0, 0, 0, time.UTC),
	}
	if len(got) != len(want) {
		t.Fatalf("hour count mismatch: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Fatalf("hour[%d] mismatch: got=%s want=%s", i, got[i], want[i])
		}
	}
}
