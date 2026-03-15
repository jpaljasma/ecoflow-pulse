package archiveaudit

import (
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
)

func TestCompareFindsManifestAndArchiveDrift(t *testing.T) {
	t.Parallel()

	manifest := []replaycli.ManifestObject{
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/a.pb.zst"},
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/b.pb.zst"},
	}
	direct := []replaycli.ManifestObject{
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/b.pb.zst"},
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/c.pb.zst"},
	}

	report := Compare(manifest, direct)
	if report.Healthy() {
		t.Fatal("expected report to detect drift")
	}
	if report.MissingInArchiveCount != 1 || report.MissingInArchiveKeys[0] != "pulse-telemetry-raw|raw/a.pb.zst" {
		t.Fatalf("unexpected missing-in-archive report: %+v", report)
	}
	if report.MissingInManifestCount != 1 || report.MissingInManifestKeys[0] != "pulse-telemetry-raw|raw/c.pb.zst" {
		t.Fatalf("unexpected missing-in-manifest report: %+v", report)
	}
}

func TestCompareDedupesRepeatedKeys(t *testing.T) {
	t.Parallel()

	manifest := []replaycli.ManifestObject{
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/a.pb.zst"},
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/a.pb.zst"},
	}
	direct := []replaycli.ManifestObject{
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/a.pb.zst"},
	}

	report := Compare(manifest, direct)
	if !report.Healthy() {
		t.Fatalf("expected deduped inputs to be healthy: %+v", report)
	}
	if report.ManifestObjects != 1 || report.DirectObjects != 1 {
		t.Fatalf("unexpected object counts: %+v", report)
	}
}

func TestObjectsFromCompositeKeys(t *testing.T) {
	t.Parallel()

	objects, err := ObjectsFromCompositeKeys([]string{
		"pulse-telemetry-raw|raw/a.pb.zst",
		" pulse-telemetry-raw | /raw/a.pb.zst/ ",
		"pulse-telemetry-raw|raw/b.pb.zst",
	})
	if err != nil {
		t.Fatalf("ObjectsFromCompositeKeys() error = %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 deduped objects, got %d", len(objects))
	}
	if objects[0].ObjectBucket != "pulse-telemetry-raw" || objects[0].ObjectKey != "raw/a.pb.zst" {
		t.Fatalf("unexpected first object: %+v", objects[0])
	}
	if objects[1].ObjectBucket != "pulse-telemetry-raw" || objects[1].ObjectKey != "raw/b.pb.zst" {
		t.Fatalf("unexpected second object: %+v", objects[1])
	}
}

func TestObjectsFromCompositeKeysRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := ObjectsFromCompositeKeys([]string{"not-a-composite-key"}); err == nil {
		t.Fatal("expected invalid composite key error")
	}
}
