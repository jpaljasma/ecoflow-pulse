# Backend Go Memory - Pulse Pi 5 Appliance

## Current focus

- Add conservative Phase 4 Go runtime caps without merging processes yet.

## Files to inspect first

- `deploy/charts/pulse-services/templates/workers.yaml`
- `deploy/charts/pulse-services/templates/_helpers.tpl`
- `deploy/charts/pulse-services/values.yaml`
- `deploy/env/pi/values.services.yaml`
- `docs/how-to/run-pi5-appliance-capacity-burn-in.md`
- `docs/architecture/config-06-pi5-appliance.md`

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
- The Pi backup runbook should dump the shared `pulse-platform-core` CNPG
  database because Keycloak uses that database through `externalDatabase`.
- The Pi runbook should not reuse local `make dr-*` targets because appliance
  archive objects live in GCS, not MinIO.
- The current Phase 4 slice should cap each separate singleton process below
  its container memory limit and defer process merging until Pi burn-in data
  exists.

## Open risks

- Merged workers can accidentally reduce deploy safety if drain and cancellation
  are not tested carefully.
- The current backup procedure is manual documentation; follow-up automation
  can turn the same gates into a `pulse-appliance backup` command.
- Per-workload caps need render validation so future chart edits cannot silently
  drop Pi runtime limits.

## Next step

- Validate Helm render, appliance scripts, and docs for the runtime-cap slice.
