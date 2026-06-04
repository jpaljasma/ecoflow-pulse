# Platform Deploy Memory - Pulse Pi 5 Appliance

## Current focus

- Complete the Phase 1 host/K3s/appliance deploy foundation on
  `codex/pi5-appliance-phase2`.

## Files to inspect first

- `deploy/charts/pulse-platform/values.yaml`
- `deploy/charts/pulse-services/values.yaml`
- `deploy/env/local/values.platform.yaml`
- `deploy/env/local/values.services.yaml`
- `deploy/pulse-edge/pulse-edge-collector.service`
- `docs/architecture/config-06-pi5-appliance.md`
- `deploy/appliance/pi5/pulse-appliance-host-prepare.sh`
- `deploy/appliance/pi5/pulse-appliance-install.sh`
- `deploy/appliance/pi5/pulse-appliance-status.sh`
- `deploy/appliance/pi5/test-install-dry-run.sh`
- `deploy/env/pi/values.platform.yaml`
- `deploy/env/pi/values.services.yaml`

## Decisions made

- Appliance mode is single-node K3s, not k3d.
- Pi host tuning and K3s config should be generated or installed by appliance
  scripts, not hidden in prose only.
- PCIe Gen 2 is the default for the Argon/SanDisk NVMe path.
- CNPG Postgres parameters, Go runtime env hooks, and loopback gRPC hostPort
  support belong in chart templates so Pi overlays render without post-render
  patching.
- Pi ingress uses `pulse.home.arpa` and host-network ingress-nginx because K3s
  `servicelb` is disabled.
- 2026-06-04: Appliance install/upgrade orchestration lives in
  `deploy/appliance/pi5/pulse-appliance-install.sh`; Make targets wrap install,
  upgrade, wait, and status using `APPLIANCE_PI_INSTALL_ARGS`.
- 2026-06-04: The installer fails before services rollout if
  `pulse-services/pulse-services-runtime-secret` is missing, keeping GCS and
  provider credentials install-specific.

## Open risks

- Singleton chart overlays are linted, but real Pi scheduling and memory use
  still need hardware validation.
- Real K3s install behavior still needs hardware validation.

## Next step

- Hand off to `edge-ble` for direct gRPC and durable retry work.
