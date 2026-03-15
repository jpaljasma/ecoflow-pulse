package replaycli

import "testing"

func TestNormalizeManifestObjectRefs(t *testing.T) {
	t.Parallel()

	buckets, keys := normalizeManifestObjectRefs([]ManifestObject{
		{ObjectBucket: " pulse-telemetry-raw ", ObjectKey: " /raw/a.pb.zst/ "},
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/a.pb.zst"},
		{ObjectBucket: "pulse-telemetry-raw", ObjectKey: "raw/b.pb.zst"},
		{ObjectBucket: "", ObjectKey: "raw/skip.pb.zst"},
	})

	if len(buckets) != 2 || len(keys) != 2 {
		t.Fatalf("unexpected normalized ref counts: buckets=%v keys=%v", buckets, keys)
	}
	if buckets[0] != "pulse-telemetry-raw" || keys[0] != "raw/a.pb.zst" {
		t.Fatalf("unexpected first ref: %q %q", buckets[0], keys[0])
	}
	if buckets[1] != "pulse-telemetry-raw" || keys[1] != "raw/b.pb.zst" {
		t.Fatalf("unexpected second ref: %q %q", buckets[1], keys[1])
	}
}
