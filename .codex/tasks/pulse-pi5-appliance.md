# Pulse Pi 5 Appliance Task Board

Status: `PLAN-ONLY PR`
Plan: `.codex/plans/pulse-pi5-appliance-ralph-loop.md`
Branch: `codex/pi5-appliance`
Base commit: `fb41daa1`

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
| TODO | `platform-deploy` | Host install scripts, K3s config, `deploy/env/pi/`, appliance status | plan PR merged | pending |
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

## Blockers

- Product/runtime implementation is intentionally blocked until this plan PR
  merges.

## Next Actions

1. Push the plan-only PR and verify the rendered PR body.
2. After merge, start Phase 1 on a fresh implementation branch.
3. Keep this task board current during every Ralph-loop slice.
