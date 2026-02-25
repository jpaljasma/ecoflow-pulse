package replaycli

import (
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

func TestNormalizePublishTarget(t *testing.T) {
	t.Parallel()

	cases := map[NATSPublishTarget]NATSPublishTarget{
		"":           NATSPublishTargetReplay,
		"replay":     NATSPublishTargetReplay,
		"Replay":     NATSPublishTargetReplay,
		"ingest":     NATSPublishTargetIngest,
		"Ingest":     NATSPublishTargetIngest,
		"not-valid":  "",
		"projection": "",
	}
	for in, want := range cases {
		if got := normalizePublishTarget(in); got != want {
			t.Fatalf("normalizePublishTarget(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNATSPublisherSubjectForShard(t *testing.T) {
	t.Parallel()

	cfg := telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 128}
	replay := &NATSPublisher{subjectCfg: cfg.Normalized(), target: NATSPublishTargetReplay}
	ingest := &NATSPublisher{subjectCfg: cfg.Normalized(), target: NATSPublishTargetIngest}

	if got, want := replay.subjectForShard(7), "pulse.telemetry.replay.s007"; got != want {
		t.Fatalf("replay subject mismatch: got=%s want=%s", got, want)
	}
	if got, want := ingest.subjectForShard(7), "pulse.telemetry.ingest.s007"; got != want {
		t.Fatalf("ingest subject mismatch: got=%s want=%s", got, want)
	}
}
