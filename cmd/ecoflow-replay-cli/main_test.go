package main

import (
	"testing"
	"time"
)

func TestResolveWindowDefaultsAndValidation(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	from, to, err := resolveWindow("", "", now)
	if err != nil {
		t.Fatalf("resolveWindow default error = %v", err)
	}
	if from != now.Add(-time.Hour).UnixMilli() || to != now.UnixMilli() {
		t.Fatalf("default window mismatch: from=%d to=%d", from, to)
	}
	if _, _, err := resolveWindow("2026-03-10T13:00:00Z", "2026-03-10T12:00:00Z", now); err == nil {
		t.Fatal("expected from>to validation error")
	}
}

func TestParseShardsDedupesAndSorts(t *testing.T) {
	got := parseShards("7,2,7,1,bad")
	want := []uint32{1, 2, 7}
	if len(got) != len(want) {
		t.Fatalf("len(parseShards)=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseShards[%d]=%d want=%d", i, got[i], want[i])
		}
	}
}
