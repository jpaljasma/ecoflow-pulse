# Backend Go Memory - Pulse Pi 5 Appliance

## Current focus

- Document Phase 3 backup/restore and planned cloud-shutdown cutover gates.

## Files to inspect first

- `docs/how-to/run-pi5-appliance-backup-cutover.md`
- `docs/architecture/config-06-pi5-appliance.md`
- `docs/reference/commands.md`
- `deploy/env/pi/values.platform.yaml`
- `deploy/env/pi/values.services.yaml`

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

## Open risks

- Merged workers can accidentally reduce deploy safety if drain and cancellation
  are not tested carefully.
- The current backup procedure is manual documentation; follow-up automation
  can turn the same gates into a `pulse-appliance backup` command.

## Next step

- Validate docs and open the backup/cutover runbook PR.
