package telemetrybus

import "fmt"

// IngestWildcardSubject returns the wildcard subject used by projection/archive
// consumers to subscribe to all ingest shards.
func IngestWildcardSubject(cfg SubjectConfig) string {
	cfg = cfg.Normalized()
	return fmt.Sprintf("%s.telemetry.ingest.s*", cfg.Prefix)
}
