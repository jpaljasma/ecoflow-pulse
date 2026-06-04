# Platform Deploy Memory - Pulse Pi 5 Appliance

## Current focus

- Implement the Phase 1 host/K3s/appliance overlay slice on
  `codex/pi5-appliance-phase1`.

## Files to inspect first

- `deploy/charts/pulse-platform/values.yaml`
- `deploy/charts/pulse-services/values.yaml`
- `deploy/env/local/values.platform.yaml`
- `deploy/env/local/values.services.yaml`
- `deploy/pulse-edge/pulse-edge-collector.service`
- `docs/architecture/config-06-pi5-appliance.md`
- `deploy/appliance/pi5/pulse-appliance-host-prepare.sh`
- `deploy/appliance/pi5/pulse-appliance-status.sh`
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

## Open risks

- Singleton chart overlays are linted, but real Pi scheduling and memory use
  still need hardware validation.
- The first slice does not install K3s or deploy Helm automatically yet; it
  lays down the config and validation pieces.

## Next step

- Run full repo lint, then decide whether to add installer orchestration in this
  branch or keep it as the next Ralph-loop slice.
