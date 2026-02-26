package telemetrybus

import "testing"

func TestIngestWildcardSubject(t *testing.T) {
	t.Parallel()
	got := IngestWildcardSubject(SubjectConfig{Prefix: "pulse", ShardCount: 128})
	if got != "pulse.telemetry.ingest.*" {
		t.Fatalf("unexpected ingest wildcard subject: %s", got)
	}
}

func TestGapRepairWildcardSubject(t *testing.T) {
	t.Parallel()
	got := GapRepairWildcardSubject(SubjectConfig{Prefix: "pulse", ShardCount: 128})
	if got != "pulse.telemetry.gaprepair.*" {
		t.Fatalf("unexpected gap-repair wildcard subject: %s", got)
	}
}
