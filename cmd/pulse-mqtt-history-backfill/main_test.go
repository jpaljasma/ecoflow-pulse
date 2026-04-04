package main

import (
	"net/url"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
)

func TestResolveBackfillWindowDefaultsToLocalDay(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("EEST", 3*60*60)
	now := time.Date(2026, time.April, 3, 21, 17, 42, 0, location)
	from, to, err := resolveBackfillWindow("", "", now)
	if err != nil {
		t.Fatalf("resolveBackfillWindow() error = %v", err)
	}
	if want := time.Date(2026, time.April, 3, 0, 0, 0, 0, location); !from.Equal(want) {
		t.Fatalf("from mismatch: got=%s want=%s", from, want)
	}
	if want := time.Date(2026, time.April, 3, 21, 18, 0, 0, location); !to.Equal(want) {
		t.Fatalf("to mismatch: got=%s want=%s", to, want)
	}
}

func TestBuildReplayURLUsesWindowAndStep(t *testing.T) {
	t.Parallel()

	cfg := config{
		emulatorURL:    "http://127.0.0.1:18080",
		sampleInterval: time.Minute,
		from:           time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC),
		to:             time.Date(2026, time.April, 3, 10, 2, 0, 0, time.UTC),
	}

	rawURL, err := buildReplayURL(cfg, cfg.from, cfg.to)
	if err != nil {
		t.Fatalf("buildReplayURL() error = %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Path != "/replay" {
		t.Fatalf("path = %q, want /replay", parsed.Path)
	}
	if got := parsed.Query().Get("from"); got != "2026-04-03T10:00:00Z" {
		t.Fatalf("from query = %q", got)
	}
	if got := parsed.Query().Get("to"); got != "2026-04-03T10:02:00Z" {
		t.Fatalf("to query = %q", got)
	}
	if got := parsed.Query().Get("step"); got != "1m0s" {
		t.Fatalf("step query = %q", got)
	}
}

func TestDiffManifestObjectsReturnsOnlyReplayObjects(t *testing.T) {
	t.Parallel()

	pre := []replaycli.ManifestObject{
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/part-00001.pb.zst"},
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/part-00002.pb.zst"},
	}
	post := []replaycli.ManifestObject{
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/part-00001.pb.zst"},
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/part-00002.pb.zst"},
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/part-00003.pb.zst"},
	}
	got := diffManifestObjects(pre, post)
	if len(got) != 1 {
		t.Fatalf("len(diffManifestObjects) = %d, want 1", len(got))
	}
	if got[0].ObjectKey != "raw/part-00003.pb.zst" {
		t.Fatalf("replay object key = %q", got[0].ObjectKey)
	}
}
