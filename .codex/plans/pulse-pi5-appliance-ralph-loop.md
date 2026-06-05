# Pulse Pi 5 Appliance Ralph-Loop Plan

Status: Implementation in progress
Last updated: 2026-06-05

## Goal

Migrate Pulse into a local Raspberry Pi 5 appliance profile that can run on an
8GB Raspberry Pi 5 with an Argon NEO 5 M.2 NVMe case and a 500GB SanDisk Optimus
GX 7100 SSD, while keeping Google Cloud resources off except for GCS object
storage.

The implementation must proceed iteratively with Ralph Loop after this plan PR
merges. Each implementation slice updates the task board, the relevant role
memory, architecture docs, and validation evidence before moving to the next
slice.

## Source Of Truth

- User-approved migration plan from 2026-06-04.
- `docs/architecture/README.md` locked architecture and milestone board.
- `docs/architecture/config-02-local-k3d-simple.md` for Kubernetes-local
  parity.
- `docs/architecture/config-03-platform-ha-defaults.md` for defaults that must
  be simplified for appliance mode.
- `docs/architecture/config-04-data-retention-replay.md` and ADR-0006 for raw
  archive and replay durability.
- ADR-0014 for provider ingest leases and graceful drain.
- Existing Pi BLE collector docs and service packaging.

## Locked Decisions

- Runtime target is single-node K3s on Raspberry Pi OS Lite 64-bit.
- BLE runs on the host under systemd from day one and writes directly to the
  in-cluster Pulse API over loopback gRPC.
- Keycloak stays in the appliance. Local username/password auth is mandatory;
  social login is optional per install.
- Default LAN name is `https://pulse.home.arpa`; Plan B is a real domain with
  split-horizon/private DNS and no public ingress.
- Google Cloud runtime is shut down after cutover; GCS object storage remains.
- Databases, cache, and queues are singletons. No clustered CNPG, Valkey
  Sentinel topology, or multi-replica NATS in appliance mode.
- Raw archive upload uses local SSD spillover before GCS upload so GCS outages
  do not interrupt local ingestion.
- The steady-state capacity target is 1-2 user profiles and about 10 devices.
- PCIe Gen 2 is the appliance default. Gen 3 is not shipped.
- Implementation starts only after this plan-only PR merges.

## Role Roster

- `project-manager`: owns task board, phase boundaries, PR hygiene, and
  progress updates.
- `platform-deploy`: owns Pi OS tuning, K3s config, Helm values, installer,
  status, and upgrade flow.
- `backend-go`: owns merged runtime design, archive outbox, MQTT restart
  safety, worker lifecycle, and Go validation.
- `edge-ble`: owns host BLE service, direct gRPC transport, outbox retry, and
  enrollment behavior.
- `bff-node`: owns any public setup/auth/API adaptations needed for appliance
  onboarding and local hostname behavior.
- `frontend-universal`: owns first-user/appliance UX, local auth assumptions,
  and any visible product states.
- `qa`: owns slice-level validation, capacity burn-in, failure drills, and
  regression tracking.
- `product-review`: owns appliance simplicity, privacy, local-only posture, and
  acceptance walkthroughs.

## Progress Output Format

Use this short update after each coherent slice:

```text
Progress
- done:
- in flight:
- next:
- tests:
- blockers:
- cost note:
```

## Work Breakdown

### Phase 0: Plan PR

- Add this Ralph-loop plan and the canonical task board.
- Add per-role memories for the appliance execution loop.
- Add the Pi appliance architecture config and milestone tracking row.
- Validate Markdown and create a PR with no product/runtime code.

### Phase 1: Host And K3s Foundation

- Add appliance host scripts for Raspberry Pi OS Lite 64-bit, NVMe boot checks,
  sysctl, journald, zram, fstrim, and K3s config.
- Add `deploy/env/pi/` platform and services overlays with singleton,
  conservative resource defaults.
- Add `pulse-appliance status` with host, K3s, Helm, Keycloak, GCS, and disk
  health checks.
- Validate chart render/lint and script shell checks.

### Phase 2: BLE Direct Ingest

- Add direct gRPC transport to `pulse-edge-collector` while keeping REST
  supported.
- Add durable BLE outbox and `client_sample_id` idempotency for edge telemetry
  retries.
- Add host systemd unit defaults for loopback gRPC, bounded memory, and BLE
  restart safety.
- Validate collector restart, Pulse restart, blocked API, and duplicate-retry
  behavior.

### Phase 3: Archive And Cutover Durability

- Add local archive upload outbox so local pipeline ACKs after SSD fsync and
  uploads to GCS asynchronously.
- Keep GCS manifest authoritative for uploaded remote objects only.
- Add backup/restore and migration runbook for planned hosted-cloud downtime.
- Validate blocked GCS for 24h, restore from backup, and fail-closed rebuilds
  while outbox is pending.

### Phase 4: Workload Consolidation

- Publish digest-pinned `linux/arm64` appliance images and local release values
  before live burn-in.
- Merge appliance workloads only where it reduces memory/process overhead
  without weakening shutdown correctness.
- Prefer a `pulse-core` style Go runtime that can run ingest, projection,
  rollup, archive, inference, scheduler, gRPC, and energy modules together.
- Keep public web and realtime separate unless measurement proves otherwise.
- Validate graceful shutdown, NATS durability, and race-sensitive worker paths.

### Phase 5: Appliance Acceptance

- Run capacity burn-in with 10 devices, 1-2 users, BLE enabled, and GCS uploads
  intermittently blocked.
- Verify steady RSS under 4.8GiB, at least 1GiB host headroom, no throttling,
  and disk warning below 80GiB free.
- Complete product review for local-only domain, Keycloak auth, setup UX, and
  cloud-shutdown readiness.

## Validation Expectations

- Plan-only PR: `make lint`.
- Deploy slices: `helm lint deploy/charts/pulse-platform -f deploy/env/pi/values.platform.yaml`
  and `helm lint deploy/charts/pulse-services -f deploy/env/pi/values.services.yaml`.
- Go slices: targeted `go test` for touched packages plus `make test-race` for
  worker shutdown, ingest, archive, and edge retry changes.
- Frontend/BFF slices: targeted workspace tests and typecheck for changed
  packages.
- Appliance acceptance: host status checks, reboot drills, restart drills, GCS
  outage drill, BLE retry/idempotency drill, and capacity burn-in.
