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
- Phase 4 should keep the appliance boring and robust: cap the current
  singleton services first, then merge only after burn-in evidence shows the
  current layout is comfortably within the 8 GB Pi budget.
- 2026-06-05: First deploy must fail early with clear operator guidance when
  release images or local secrets are missing; crash-looping placeholder pods
  would make appliance setup feel unreliable.
- 2026-06-05: Local release values and runtime secret helpers keep appliance
  setup simple while avoiding committed image pins, provider material, or GCS
  service-account JSON.

## Open risks

- Over-clustering or exposing Kubernetes details in normal user flows would make
  the appliance feel harder than necessary.
- Appliance-for-others constraints require local auth to work without shared
  social OAuth clients.
- The current cutover runbook is operator-facing; future appliance-for-others
  packaging should turn the same checks into simple status and backup commands.
- Merging too early could make support harder because one process restart would
  take out unrelated roles; the product bar is lower operator burden, not fewer
  Kubernetes objects at any cost.
- Image publishing and secret setup are now the main pre-burn-in product risks:
  the target path should be one local values file, one secret helper command,
  one upgrade command, then status.

## Next step

- Review the release-inputs slice for fail-fast setup, clear local-only secret
  handling, and a direct path back to burn-in.
