# Pulse Pi 5 Appliance Task Board

Status: `PROGRESS`
Plan: `.codex/plans/pulse-pi5-appliance-ralph-loop.md`
Branch: `codex/pi5-backup-cutover-runbook`
Base commit: `eb49efc6`

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
| DONE | `platform-deploy` | Host install scripts, K3s config, `deploy/env/pi/`, appliance status, installer orchestration | plan PR merged | `make appliance-pi-validate` |
| PROGRESS | `edge-ble` | Direct gRPC transport, BLE outbox, systemd defaults, enrollment path | Phase 1 loopback API and installer | direct gRPC merged; durable outbox tests passed |
| PROGRESS | `backend-go` | Archive upload outbox, merged runtime, ingest restart safety | Phase 1 overlays | Phase 3 backup/cutover runbook in progress |
| TODO | `bff-node` | Appliance setup/auth/API adaptations if needed | backend/setup scope | pending |
| TODO | `frontend-universal` | First-user/setup UX and local-auth product states if needed | BFF/setup scope | pending |
| TODO | `qa` | Capacity burn-in, reboot/restart/GCS/BLE failure drills | implementation slices | pending |
| TODO | `product-review` | Simplicity, local-only posture, appliance acceptance walkthrough | QA evidence | pending |

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

## Blockers

- None for the backup/cutover runbook.

## Next Actions

1. Validate and open the backup/restore and planned cloud-shutdown cutover
   runbook PR.
2. Start conservative workload consolidation and capacity burn-in planning after
   the runbook lands.

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
