# Backend Go Memory - Pulse Pi 5 Appliance

## Current focus

- Implement the first Phase 3 archive upload outbox slice for appliance GCS
  durability.

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
- Archive delivery ACKs in appliance mode only after the compressed object and
  manifest record are fsynced into the local outbox.
- Outbox flush records the remote object in the manifest only after successful
  upload, and fails closed when a manifest-backed entry has no manifest store.

## Open risks

- Rebuild/status tooling still needs an explicit pending-outbox guard before
  appliance cutover.
- Merged workers can accidentally reduce deploy safety if drain and cancellation
  are not tested carefully.

## Next step

- Follow up with the pending-outbox status/rebuild guard and backup/cutover
  runbook before broader workload consolidation.
