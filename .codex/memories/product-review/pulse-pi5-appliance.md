# Product Review Memory - Pulse Pi 5 Appliance

## Current focus

- Preserve the appliance promise: local, simple, resilient, and cheap to run.

## Files to inspect first

- `docs/architecture/config-06-pi5-appliance.md`
- `.codex/plans/pulse-pi5-appliance-ralph-loop.md`
- Future first-run/setup UX docs or screens.

## Decisions made

- Appliance mode is for 1-2 user profiles and about 10 devices.
- Hosted Google Cloud runtime is turned off after migration, except GCS object
  storage.
- BLE support is part of day-one appliance value, not a later add-on.
- Backup/cutover guidance should say what to do without making the appliance
  feel like a cloud platform that needs constant operator attention.
- Local username/password Keycloak login remains mandatory; social login is
  optional per install.

## Open risks

- Over-clustering or exposing Kubernetes details in normal user flows would make
  the appliance feel harder than necessary.
- Appliance-for-others constraints require local auth to work without shared
  social OAuth clients.
- The current cutover runbook is operator-facing; future appliance-for-others
  packaging should turn the same checks into simple status and backup commands.

## Next step

- Review the backup/cutover runbook for simplicity, local-only posture, and
  clear hosted-cost shutdown boundaries.
