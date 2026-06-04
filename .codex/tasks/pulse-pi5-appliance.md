# Pulse Pi 5 Appliance Task Board

Status: `PROGRESS`
Plan: `.codex/plans/pulse-pi5-appliance-ralph-loop.md`
Branch: `codex/pi5-appliance-phase1`
Base commit: `8aabf0bc`

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
| PROGRESS | `platform-deploy` | Host install scripts, K3s config, `deploy/env/pi/`, appliance status | plan PR merged | `make appliance-pi-validate` |
| TODO | `edge-ble` | Direct gRPC transport, BLE outbox, systemd defaults, enrollment path | Phase 1 loopback API | pending |
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

## Blockers

- None for the Phase 1 scaffold slice.

## Next Actions

1. Run full repo lint after the Phase 1 scaffold update.
2. Review rendered Pi manifests for singleton/resource-budget regressions.
3. Decide whether this Phase 1 branch should stop at scaffolding or continue
   into installer/upgrade commands before PR.
