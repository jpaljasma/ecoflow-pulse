# Backend Go Memory - Pulse Pi 5 Appliance

## Current focus

- Implement the Phase 3 pending archive upload outbox status/rebuild guard.

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
- Archive-backed rebuilds must fail closed while local archive upload outbox
  entries are pending; raw-log rebuilds can bypass this because they do not
  depend on authoritative object-storage coverage.

## Open risks

- Backup/restore and planned cloud-shutdown cutover docs still need to land
  before appliance cutover.
- Merged workers can accidentally reduce deploy safety if drain and cancellation
  are not tested carefully.

## Next step

- Validate the pending-outbox guard and then move to the backup/cutover runbook.
