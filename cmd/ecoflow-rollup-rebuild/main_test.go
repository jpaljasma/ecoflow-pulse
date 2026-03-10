package main

import (
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/rolluprebuild"
)

func TestResolveWindowDefaultsAndValidation(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	from, to, err := resolveWindow("", "", now)
	if err != nil {
		t.Fatalf("resolveWindow default error = %v", err)
	}
	if from != now.Add(-48*time.Hour).UnixMilli() || to != now.UnixMilli() {
		t.Fatalf("default window mismatch: from=%d to=%d", from, to)
	}
	if _, _, err := resolveWindow("2026-03-10T12:00:00Z", "2026-03-10T12:00:00Z", now); err == nil {
		t.Fatal("expected invalid equal window")
	}
}

func TestDiffMinuteSummariesSortsKeys(t *testing.T) {
	pre := map[string]rolluprebuild.BucketWindowSummary{
		"B": {TotalBuckets: 1},
	}
	post := map[string]rolluprebuild.BucketWindowSummary{
		"A": {TotalBuckets: 2},
	}
	got := diffMinuteSummaries(pre, post)
	if len(got) != 2 {
		t.Fatalf("diff len=%d want=2", len(got))
	}
	if got[0].ProviderDeviceID != "A" || got[1].ProviderDeviceID != "B" {
		t.Fatalf("expected sorted provider ids, got=%q then %q", got[0].ProviderDeviceID, got[1].ProviderDeviceID)
	}
}
