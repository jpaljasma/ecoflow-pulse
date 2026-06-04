# Pulse Pi 5 Appliance Task Board

Status: `PROGRESS`
Plan: `.codex/plans/pulse-pi5-appliance-ralph-loop.md`
Branch: `codex/pi5-appliance-ble-direct`
Base commit: `42de0371`

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
| PROGRESS | `edge-ble` | Direct gRPC transport, BLE outbox, systemd defaults, enrollment path | Phase 1 loopback API and installer | direct gRPC tests passed; outbox pending |
| TODO | `backend-go` | Archive upload outbox, merged runtime, ingest restart safety | Phase 1 overlays | pending |
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

## Blockers

- None for direct gRPC transport.

## Next Actions

1. Add failing tests for `PULSE_EDGE_TRANSPORT=grpc` request mapping.
2. Implement gRPC enroll, heartbeat, discovery, and telemetry calls while
   preserving REST as the default.
3. Update appliance systemd/docs defaults for loopback gRPC.
4. Leave durable outbox and `client_sample_id` for the next Phase 2 slice.

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
