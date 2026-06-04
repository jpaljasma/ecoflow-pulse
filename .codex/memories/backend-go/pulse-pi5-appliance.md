# Backend Go Memory - Pulse Pi 5 Appliance

## Current focus

- Prepare later backend slices for archive outbox, ingest restart safety, and
  optional workload consolidation.

## Files to inspect first

- `cmd/ecoflow-grpc-api/main.go`
- `cmd/ecoflow-archive-worker/main.go`
- `cmd/ecoflow-ingest-worker/main.go`
- `internal/archiveworker`
- `internal/telemetrybus`
- `internal/pipelineintegration`

## Decisions made

- Appliance mode should prefer singleton durable services over clustered
  topology.
- Workload merging is allowed only when graceful shutdown and idempotency remain
  clear.
- GCS upload must not be on the critical local ingestion path during outages.

## Open risks

- Archive upload outbox changes affect replay correctness and must fail closed
  when pending local-only objects exist.
- Merged workers can accidentally reduce deploy safety if drain and cancellation
  are not tested carefully.

## Next step

- After Phase 1 overlays exist, design the smallest archive-outbox schema and
  worker slice with targeted tests before any broader runtime merge.
