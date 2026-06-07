# Pulse Pi 5 Appliance Task Board

Status: `PROGRESS`
Plan: `.codex/plans/pulse-pi5-appliance-ralph-loop.md`
Branch: `codex/pi5-hostport-rollout`
Base commit: `7057d1c8`

## Assumptions

- Implementation starts only after the plan-only PR merges.
- The target appliance hardware is Raspberry Pi 5 8GB, Argon NEO 5 M.2 NVMe
  case, and SanDisk Optimus GX 7100 500GB M.2 2280 SSD.
- The appliance turns off hosted Google Cloud runtime after cutover and keeps
  only GCS object storage.
- User-visible appliance docs and PR text must not include install-specific
  user, device, provider, serial, or account identifiers.

## Workstreams

| Status | Owner | Workstream | Dependency | Latest validation |
|---|---|---|---|---|
| DONE | `project-manager` | Plan-only PR, architecture tracker, Ralph-loop scaffold | user-approved plan | `make lint` |
| DONE | `platform-deploy` | Host install scripts, K3s config, `deploy/env/pi/`, appliance status, installer orchestration, image release artifact | plan PR merged | `make appliance-pi-validate` |
| PROGRESS | `edge-ble` | Direct gRPC transport, BLE outbox, systemd defaults, enrollment path | Phase 1 loopback API and installer | direct gRPC merged; durable outbox tests passed |
| PROGRESS | `backend-go` | Archive upload outbox, merged runtime, ingest restart safety | Phase 1 overlays | Phase 3 backup/cutover runbook merged |
| TODO | `bff-node` | Appliance setup/auth/API adaptations if needed | backend/setup scope | pending |
| TODO | `frontend-universal` | First-user/setup UX and local-auth product states if needed | BFF/setup scope | pending |
| PROGRESS | `qa` | Capacity burn-in, reboot/restart/GCS/BLE failure drills | implementation slices | Pi image artifact render and lint passed |
| PROGRESS | `product-review` | Simplicity, local-only posture, appliance acceptance walkthrough | QA evidence | Phase 4 keeps singleton layout until hardware evidence exists |

## Decisions

- 2026-06-04: Use Ralph Loop for implementation after the plan-only PR merges.
- 2026-06-04: Keep this PR plan-only; runtime code, Helm overlays, and scripts
  land in later iterative PRs.
- 2026-06-04: Appliance mode keeps Keycloak and makes social IdPs optional per
  install.
- 2026-06-04: BLE must ship from day one and should use direct local gRPC
  instead of the REST bridge when running on the appliance.
- 2026-06-04: Appliance capacity is sized for 1-2 users and about 10 devices.
- 2026-06-04: Phase 1 starts on `codex/pi5-appliance-phase1` after the
  plan-only PR merged to `main`.
- 2026-06-04: The first implementation slice adds Pi host scripts, K3s config,
  Pi Helm overlays, loopback gRPC hostPort chart support, and appliance
  validation targets.
- 2026-06-04: The second implementation slice completes Phase 1 with
  `pulse-appliance-install.sh`, dry-run installer coverage, and Make entry
  points for install, upgrade, wait, and status.
- 2026-06-04: Phase 2 starts on `codex/pi5-appliance-ble-direct` and keeps the
  first BLE slice focused on direct gRPC transport plus appliance service/docs.
- 2026-06-04: Pi 5 `SDRAM_BANKLOW=1` tuning is documented as an optional
  lab-only experiment because it currently depends on `rpi-update` pre-release
  firmware; appliance defaults stay on stable APT/EEPROM updates.
- 2026-06-04: Pi 5 `arm_freq=2800` with `over_voltage_delta=25000` is
  documented as an optional lab overclock candidate, not an appliance default;
  it requires no throttling across reboot, K3s, BLE, and capacity burn-in
  checks before use.
- 2026-06-04: Real hardware stress testing settled on `arm_freq=2500` with
  `over_voltage_delta=10000` as the conservative observed overclock candidate
  with no throttling; `2800/25000` remains a more aggressive lab experiment.
- 2026-06-04: Phase 3 starts on `codex/pi5-gcs-archive-outbox` and keeps the
  first archive slice focused on SSD-backed GCS upload outbox durability.
- 2026-06-04: A Pi 5 Gen3 PCIe lab test with the Argon NEO 5 and SanDisk
  Optimus GX 7100 improved `hdparm -t /dev/nvme0n1` from about `449 MB/sec`
  to about `882 MB/sec`; Gen2 remains the appliance shipping default until
  reboot, thermal, SMART, and unsafe-shutdown checks pass on the target unit.
- 2026-06-04: Archive outbox entries carry both object bytes and manifest
  records, ACK local delivery only after fsync, and flush fail-closed if the
  manifest store is unavailable.
- 2026-06-04: Phase 3 status guard starts on
  `codex/pi5-archive-outbox-status-guard` from merged main `cfb60575`.
- 2026-06-04: Rebuilds that use archive objects must refuse to run while
  `ARCHIVE_UPLOAD_OUTBOX_DIR` has pending local entries unless an operator uses
  an explicit manual override.
- 2026-06-04: The planned cutover runbook treats the shared
  `pulse-platform-core` CNPG database as the appliance backup source for Pulse
  app data and Keycloak state, keeps GCS online, and requires an empty archive
  upload outbox before hosted runtime shutdown or archive-backed rebuilds.
- 2026-06-04: Phase 4 starts on `codex/pi5-workload-consolidation` from merged
  main `5da6afac`; the first slice applies conservative per-workload Go runtime
  caps to the current singleton services layout instead of merging processes
  before hardware burn-in evidence.
- 2026-06-05: Live Pi burn-in preparation found the merged scaffold still used
  `pi-placeholder` images and lacked a services GCS credentials mount, so the
  next slice starts on `codex/pi5-appliance-release-inputs` from merged main
  `7a6abed1`.
- 2026-06-05: The appliance installer must fail before Helm apply when rendered
  images still use `pi-placeholder`; install-specific image digests belong in a
  local release values file outside Git.
- 2026-06-05: Pi services use a local Kubernetes secret for
  `GOOGLE_APPLICATION_CREDENTIALS` plus a mounted service-account JSON secret;
  the helper script creates both after the platform CNPG app secret exists.
- 2026-06-05: Pi appliance image publishing starts on
  `codex/pi5-appliance-images` from merged main `05eb7b00`.
- 2026-06-05: The manual `Pi Appliance Images` workflow builds the three
  appliance runtime images for `linux/arm64`, pushes them to GHCR, and uploads
  a digest-pinned release values artifact.
- 2026-06-05: Private GHCR packages are supported by rendering a shared
  `runtime.imagePullSecrets` entry into both platform and services charts; the
  operator still creates the actual pull secret locally in both namespaces.
- 2026-06-06: Live Pi rollout hardening starts on
  `codex/pi5-appliance-live-hardening` from merged main `8a6dbf5d`.
- 2026-06-06: First full Pi platform/services rollout reached deployed Helm
  releases with all singleton services running. The live run found four
  defaults to harden: public-app data plane must render `local`, Pi Keycloak
  must use the working legacy Bitnami image repositories, migration rollout env
  must accept `appliance`, and operator-run status should warn instead of fail
  when NVMe SMART requires root.
- 2026-06-06: Pi steady-state disables the upstream `nats-box` toolbox
  Deployment; `pulse-platform-nats-0` still reports `2/2` because the single
  NATS pod has the `nats` container plus the chart's config reloader sidecar.
- 2026-06-07: Post-live-rollout docs slice records the Pi-side GitHub CLI
  install/authentication path, direct release artifact download, chart
  dependency cache expectations, and CNPG CRD recovery guardrails before the
  real capacity burn-in starts.
- 2026-06-07: Live Pi upgrade proved the singleton loopback gRPC hostPort
  deployment cannot use the default surge strategy on a one-node appliance.
  `go-grpc-api` must roll with `maxSurge: 0` and `maxUnavailable: 1`; BLE
  safety comes from startup wait plus durable outbox replay, not from running
  two hostPort pods at once.

## Blockers

- Real 24h burn-in can start after the hostPort rollout fix lands, refreshed
  `linux/arm64` image artifacts are installed on the Pi, and the appliance is
  upgraded without temporary local patches.

## Next Actions

1. Open the Pi hostPort rollout PR.
2. After merge, rerun the manual Pi image workflow and install the refreshed
   `pulse-pi-release-values` artifact on the Pi.
3. Re-run appliance upgrade without temporary deployment patches.
4. Run real Pi 24h capacity burn-in with target 10-device load.
5. Decide which workloads can safely merge only after burn-in, restart, and GCS
   outage evidence.

## Validation Evidence

- 2026-06-04: `go test ./cmd/pulse-edge-collector -count=1` passed after
  adding direct gRPC transport tests.
- 2026-06-04: `go test ./cmd/ecoflow-grpc-api ./internal/edgecollector
  -count=1` passed for adjacent edge ingest/gRPC packages.
- 2026-06-04: `make pulse-edge-pi5-bundle` passed and rebuilt the Pi 5
  linux/arm64 collector bundle with the updated systemd unit.
- 2026-06-04: The Pi 5 bundle target now builds with `GOARM64=v8.2`,
  `CGO_ENABLED=0`, `-trimpath`, and stripped symbols by default.
- 2026-06-04: PR feedback follow-up added in-process startup heartbeat retry
  for loopback gRPC so cold K3s startup does not exhaust systemd start limits.
- 2026-06-04: Review follow-up validation passed with
  `go test ./cmd/pulse-edge-collector -count=1`,
  `go test ./cmd/ecoflow-grpc-api ./internal/edgecollector -count=1`,
  `make pulse-edge-pi5-bundle`, `make lint`, and
  `make appliance-pi-validate`.
- 2026-06-04: `make lint` and `make appliance-pi-validate` passed before PR
  closeout.
- 2026-06-04: Phase 2 durable outbox slice starts on
  `codex/pi5-appliance-ble-outbox` from merged main `5669a1d6`.
- 2026-06-04: Durable outbox slice adds secret-free disk entries, startup and
  heartbeat replay, stable `client_sample_id`, and deterministic edge envelope
  ids for downstream dedupe.
- 2026-06-04: Durable outbox validation passed with `go test
  ./cmd/pulse-edge-collector ./cmd/ecoflow-grpc-api ./internal/edgecollector
  ./internal/envelopededup ./internal/rollupworker ./internal/archiveworker
  -count=1`.
- 2026-06-04: Final branch validation passed with `make pulse-edge-pi5-bundle`,
  `make test-proto-contract`, `make appliance-pi-validate`, and `make lint`.
- 2026-06-04: Review follow-up preserved `clientSampleId` through the REST edge
  collector path so REST outbox retries keep the same downstream dedupe identity
  as gRPC retries.
- 2026-06-04: REST sample-id follow-up validation passed with `go test
  ./cmd/pulse-edge-collector -count=1`, `go test ./cmd/ecoflow-grpc-api
  ./internal/edgecollector -count=1`, `npm run -w apps/pulse-platform test --
  edge_routes.test.ts`, `npm run -w apps/pulse-platform typecheck`,
  `make pulse-edge-pi5-bundle`, `make appliance-pi-validate`, and `make lint`.
- 2026-06-04: Phase 3 archive outbox foundation validation passed with
  `go test ./internal/archiveworker ./cmd/ecoflow-archive-worker -count=1`,
  `helm lint deploy/charts/pulse-services -f deploy/env/pi/values.services.yaml`,
  `helm lint deploy/charts/pulse-services -f deploy/env/local/values.services.yaml`,
  Pi Helm render inspection for `ARCHIVE_UPLOAD_OUTBOX_*` and the
  `go-archive-outbox` PVC, `make appliance-pi-validate`,
  `go test -race ./internal/archiveworker ./cmd/ecoflow-archive-worker
  -count=1`, `make test-race`, `make lint`, and `git diff --check`.
- 2026-06-04: Review follow-up prevents restart/offline archive outbox
  overwrite by rejecting an enqueue when the pending object key already exists;
  validation passed with `go test ./internal/archiveworker
  ./cmd/ecoflow-archive-worker -count=1`, `go test -race
  ./internal/archiveworker ./cmd/ecoflow-archive-worker -count=1`,
  `make test-race`, `make lint`, and `git diff --check`.
- 2026-06-04: Phase 3 status/rebuild guard validation passed with
  `go test ./internal/archiveworker ./cmd/ecoflow-archive-outbox-status
  ./cmd/ecoflow-rollup-rebuild -count=1`, `make appliance-pi-validate`,
  `go test -race ./internal/archiveworker ./cmd/ecoflow-rollup-rebuild
  ./cmd/ecoflow-archive-outbox-status -count=1`, `make test-race`,
  `make lint`, and `git diff --check`.
- 2026-06-04: Phase 3 backup/cutover runbook validation passed with
  `make lint`, `git diff --check`, and the duplicate editor-backup file scan.
- 2026-06-04: Review follow-up added an empty archive upload outbox gate before
  same-Pi restore scales services down; validation passed with `make lint`,
  `git diff --check`, and the duplicate editor-backup file scan.
- 2026-06-04: Phase 4 runtime-cap slice validation passed with
  `make appliance-pi-validate`, `make lint`, `git diff --check`, the duplicate
  editor-backup file scan, and Helm render inspection for Pi `GOMAXPROCS` /
  `GOMEMLIMIT` values.
- 2026-06-05: Pi image publishing slice validation passed with
  `make appliance-pi-validate` and `make lint`; the release render test now
  verifies digest image references, optional pull secrets, and the GCS mount.
- 2026-06-06: Live Pi rollout reached deployed `pulse-platform` and
  `pulse-services` Helm releases with all services pods `1/1 Running`,
  loopback gRPC reachable, archive outbox clear, about 5.4GiB memory available,
  and no throttling. `make appliance-pi-status` only failed because NVMe SMART
  needed root privileges.
- 2026-06-06: Live-hardening validation passed with
  `go test ./internal/dbmigrate -count=1`, `make appliance-pi-validate`,
  `make lint`, and `git diff --check`; duplicate editor-backup scan returned
  no files.
- 2026-06-06: Review follow-up tightened NVMe SMART status handling so only
  actual permission-denied failures are downgraded for non-root operators;
  validation passed with the targeted NVMe fixtures, `make appliance-pi-validate`,
  `make lint`, and `git diff --check`.
- 2026-06-07: Pi GitHub CLI/artifact docs slice validation passed with
  `make lint`, `git diff --check`, and the duplicate editor-backup file scan.
- 2026-06-07: HostPort rollout red test failed as expected with
  `bash deploy/appliance/pi5/test-go-runtime-render.sh` before the Pi overlay
  set `go-grpc-api` to no-surge rollout.
- 2026-06-07: Live Pi upgrade succeeded after patching `go-grpc-api` to
  no-surge rollout; Helm `pulse-services` reached deployed revision 5 and all
  services deployments rolled out.
- 2026-06-07: HostPort rollout validation passed with the red/green
  `bash deploy/appliance/pi5/test-go-runtime-render.sh` path,
  `make appliance-pi-validate`, `make lint`, `git diff --check`, and the
  duplicate editor-backup file scan.
