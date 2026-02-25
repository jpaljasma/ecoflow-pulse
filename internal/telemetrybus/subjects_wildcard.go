package telemetrybus

import "fmt"

// IngestWildcardSubject returns the wildcard subject used by projection/archive
// consumers to subscribe to all ingest shards.
func IngestWildcardSubject(cfg SubjectConfig) string {
	cfg = cfg.Normalized()
	return fmt.Sprintf("%s.telemetry.ingest.s*", cfg.Prefix)
}

// GapRepairWildcardSubject returns the wildcard subject used by gap-repair
// workers to subscribe to all gap-repair shards.
func GapRepairWildcardSubject(cfg SubjectConfig) string {
	cfg = cfg.Normalized()
	return fmt.Sprintf("%s.telemetry.gaprepair.s*", cfg.Prefix)
}
