# Platform Deploy Memory - Pulse Pi 5 Appliance

## Current focus

- Complete the GHCR image publishing and digest release artifact slice on
  `codex/pi5-appliance-images`.

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
- `.github/workflows/pi-appliance-images.yml`
- `deploy/appliance/pi5/pulse-appliance-render-release-values.sh`

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
- 2026-06-05: The installer should also fail before Helm apply when rendered
  platform or services images still use `pi-placeholder`.
- 2026-06-05: Install-specific image repositories/digests belong in a local
  release values file supplied with `--release-values`, not in committed Pi
  defaults.
- 2026-06-05: Pi GCS archive auth uses a local `pulse-services-gcs-credentials`
  secret mounted at `/var/run/pulse-gcs`, with
  `GOOGLE_APPLICATION_CREDENTIALS` supplied by `pulse-services-runtime-secret`.
- 2026-06-05: Appliance images are published manually through the
  `Pi Appliance Images` workflow as `linux/arm64` GHCR images; the workflow
  uploads a digest-pinned release values artifact for the Pi installer.
- 2026-06-05: Private GHCR pulls use optional `runtime.imagePullSecrets`, but
  the actual registry token remains a local Kubernetes secret in each appliance
  namespace.

## Open risks

- Singleton chart overlays are linted, but real Pi scheduling and memory use
  still need hardware validation after release inputs exist.
- Real K3s install behavior is partially validated on the target Pi; full Pulse
  release rollout still needs the published image artifact and local secrets.

## Next step

- After this PR merges, run the manual image workflow, install the release
  artifact on the Pi, create local secrets, and start live burn-in.
