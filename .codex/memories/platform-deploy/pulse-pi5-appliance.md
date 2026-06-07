# Platform Deploy Memory - Pulse Pi 5 Appliance

## Current focus

- Capture the post-live-rollout operator edge cases on
  `codex/pi5-gh-install-docs`: GitHub CLI install/auth on the Pi, direct
  release artifact download, chart dependency cache behavior, and first-install
  recovery guardrails.

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
- 2026-06-06: Live Pi install required temporary overrides for
  `runtime.publicApp.env.dataPlane=local` and legacy Bitnami Keycloak image
  repositories. These should be Pi overlay defaults, not operator-local
  overrides.
- 2026-06-06: `runtime.migrations.policy.rolloutEnv=appliance` is the right
  semantic value for Pi services, so the migration runner should accept
  `appliance` rather than forcing the Pi overlay to pretend it is `local`.
- 2026-06-06: Operator-run `make appliance-pi-status` should not fail the whole
  appliance when `nvme smart-log` needs root; it should warn and document the
  sudo path for a full SMART read.
- 2026-06-06: Disable upstream `natsBox` in the Pi overlay. The NATS StatefulSet
  still has two containers because `nats` plus the config reloader sidecar is
  expected.
- 2026-06-07: The preferred appliance artifact path can run entirely from the
  Pi after installing GitHub CLI from the official Debian/Raspberry Pi APT repo
  and authenticating with the headless web/device-code flow.
- 2026-06-07: Chart dependency archives under `deploy/charts/*/charts/` are a
  useful local Pi cache for repeated upgrades, but remain untracked local files
  that must not be committed.

## Open risks

- The first full Pi rollout converged, but 24h burn-in and restart/GCS outage
  drills are still pending.
- Temporary live override files on the Pi should stay out of the normal command
  path now that the hardening defaults have merged and refreshed images are
  being installed.

## Next step

- Validate the docs branch, open the PR, then use the Pi's upgraded state and
  status output to start the 24h capacity burn-in runbook.
