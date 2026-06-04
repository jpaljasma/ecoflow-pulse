# Platform Deploy Memory - Pulse Pi 5 Appliance

## Current focus

- Prepare for the Phase 1 host/K3s/appliance overlay slice after the plan PR
  merges.

## Files to inspect first

- `deploy/charts/pulse-platform/values.yaml`
- `deploy/charts/pulse-services/values.yaml`
- `deploy/env/local/values.platform.yaml`
- `deploy/env/local/values.services.yaml`
- `deploy/pulse-edge/pulse-edge-collector.service`
- `docs/architecture/config-06-pi5-appliance.md`

## Decisions made

- Appliance mode is single-node K3s, not k3d.
- Pi host tuning and K3s config should be generated or installed by appliance
  scripts, not hidden in prose only.
- PCIe Gen 2 is the default for the Argon/SanDisk NVMe path.

## Open risks

- CNPG, NATS, Valkey, and Keycloak chart defaults are larger than the Pi 5 RAM
  budget and need explicit appliance overlays.
- Host-level status checks need to fail clearly on throttling, low disk, or
  unsafe NVMe conditions before Pulse appears healthy.

## Next step

- Add `deploy/env/pi/`, host install/status scripts, and chart lint coverage in
  the first implementation branch.
