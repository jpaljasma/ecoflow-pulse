# ADR-0014: Provider device integration model and distributed MQTT ingest leases

**Status:** Superseded  
**Date:** 2026-02-22  
**Owners:** Jaan  
**Related:** ADR-0003, ADR-0004, ADR-0012, ADR-0013
**Superseded by:** ADR-0015 (quota archive exclusion rule only)

---

## Context
EcoFlow Pulse needs a provider-agnostic ingestion model. EcoFlow is first, but the control plane must support additional providers (for example Victron, Anker Solix, Bluetti, SolarAssistant) without redesigning core ownership and ingestion flows.

Current ingestion (`cmd/ecoflow-mqtt-sub`) is single-process oriented and uses host-local file locks. That does not satisfy distributed operation across multiple Kubernetes nodes where one and only one worker may hold an active MQTT session per serial number (SN).

We also need clear credential lifecycle semantics:
- one user can store multiple provider credentials,
- credentials can be enabled/disabled,
- credential secrets are write-only from user-facing APIs,
- EcoFlow MQTT certification is fetched for every connection/reconnection,
- device discovery is manual trigger (for now), not background automatic sync.

### Requirements / Goals
- Multi-provider credential + device model while preserving current `users/devices/user_devices` authorization primitives.
- Distributed ingest workers with global single-owner session lease per `(provider, provider_device_id)`.
- Graceful drain on disable/deactivate events (event-driven orchestration).
- At-least-once ingest is acceptable in v1; duplicates may exist and will be deduplicated later.
- Keep UUIDv7 IDs and app-managed UTC timestamps for all new tables.

### Non-goals
- Exactly-once ingest semantics in v1.
- Automatic provider discovery polling in v1.
- Public internet exposure of ingest workers.

---

## Options considered
### Option A: Keep file-lock-based `cmd/ecoflow-mqtt-sub` and scale vertically
**Pros**
- Minimal code changes.
- Reuses known runtime behavior.

**Cons**
- No cross-node lock safety.
- Hard to operate as a worker pool.
- Tight coupling between terminal UI/runtime and ingestion responsibilities.

### Option B: DB row locks as lease mechanism
**Pros**
- Uses existing Postgres dependency.
- Centralized state.

**Cons**
- Higher lock contention risk on hot reconnect loops.
- Harder to tune lease heartbeat behavior at high churn.
- Less suitable for low-latency ephemeral lease operations than in-memory lease store.

### Option C: Event-driven worker pool + Valkey leases (chosen)
**Pros**
- Natural fit for short TTL lease heartbeats.
- Works cleanly across many worker pods/nodes.
- Aligns with existing Valkey decision (ADR-0004) and NATS event-driven architecture.

**Cons**
- Adds lease protocol complexity (Lua scripts, token/fencing discipline).
- Requires clear ownership semantics between DB desired state and lease state.

---

## Decision
We will implement a provider-agnostic control-plane model and distributed ingest runtime with Valkey lease locks.

### 1) Data model additions
We will add two new control-plane tables:

1. `provider_credentials`
- `id UUID PRIMARY KEY DEFAULT uuidv7()`
- `user_id UUID NOT NULL` (FK to `users`)
- `provider TEXT NOT NULL` (initially `ecoflow`; extensible)
- `access_key_ciphertext BYTEA NOT NULL`
- `secret_key_ciphertext BYTEA NOT NULL`
- `access_key_hash BYTEA NOT NULL`
- `access_key_mask TEXT NOT NULL`
- `is_active BOOLEAN NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL`
- `updated_at TIMESTAMPTZ NOT NULL`
- unique constraint to prevent duplicate logical credential entries per user/provider

2. `provider_devices`
- `id UUID PRIMARY KEY DEFAULT uuidv7()`
- `device_id UUID NOT NULL` (FK to `devices`)
- `provider TEXT NOT NULL`
- `provider_device_id TEXT NOT NULL` (EcoFlow SN)
- `credential_id UUID NOT NULL` (FK to `provider_credentials`)
- `product_name TEXT`
- `model TEXT`
- `capabilities JSONB NOT NULL DEFAULT '{}'::jsonb`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `is_active BOOLEAN NOT NULL`
- `ingest_desired_state TEXT NOT NULL` (`active | draining | paused`)
- `created_at TIMESTAMPTZ NOT NULL`
- `updated_at TIMESTAMPTZ NOT NULL`
- `UNIQUE(provider, provider_device_id)`
- `UNIQUE(device_id, provider)`

Existing `devices` and `user_devices` stay as canonical ownership/authz tables.

### 2) Credential policy
- User-facing read APIs are write-only for secrets (never return plaintext `secret_key`).
- Internal trusted services may resolve decrypted credentials for provider API calls.
- Credential `is_active=false` triggers graceful ingest drain for mapped provider devices.

### 3) Discovery + device APIs (manual-first)
- `DiscoverDevices(user_id, provider, credential_id)` is explicit/manual trigger only.
- `GetProviderCredentials(user_id, provider)` returns credential metadata (masked), and internal variant supports secure materialization for workers.
- `ListDevices(user_id)` returns active user devices grouped by provider.
- `GetMQTTCertification(provider_device_id)` (provider adapter method) is invoked for every MQTT connect/reconnect.

### 4) Distributed lease lock protocol (Valkey)
Cluster-aware keying policy:
- use versioned prefixes (`pulse:v1:*`),
- use hash tags only for key groups that must be atomic together,
- avoid forcing all ingest keys into a single hash slot.

Lease key format (atomic group by provider-device):
- `pulse:v1:ingest:lease:{provider|provider_device_id}`
- `pulse:v1:ingest:session:{provider|provider_device_id}`
- `pulse:v1:ingest:fence:{provider|provider_device_id}`

Non-atomic/global queues and indexes:
- do not share the same hash tag as per-device lease keys,
- use independent/sharded key spaces (for example `...:pending:{shard-00}` to `...:pending:{shard-63}`) to avoid hot slots.

Lease behavior:
- acquire via Lua (token + fencing counter + TTL set atomically in same hash slot),
- renew via Lua only when token matches,
- release via Lua only when token matches.

Lease client baseline (implementation):
- use official `valkey-go` cluster-aware client,
- disable client-side caching for lock operations explicitly (`DisableCache=true`),
- enable topology refresh + MOVED/ASK handling with bounded redirect settings,
- keep manager-level per-call retry overrides for transient network/cluster errors.

Initial timing:
- lease TTL: `45s`
- heartbeat: `15s` with jitter
- parameters are configurable and tuned by soak/load observations.

### 5) Worker runtime
A new ingest worker service (replacing `cmd/ecoflow-mqtt-sub` runtime for production ingest) will:
- subscribe to assignment/control events,
- claim leases for eligible provider devices,
- start one MQTT session per leased provider device,
- publish envelopes/events downstream,
- gracefully drain and release lease on deactivation/disable/shutdown.

Worker startup/reconcile concurrency defaults:
- startup worker pool default: `4 * GOMAXPROCS`, clamped to `[8, 64]`,
- startup queue default: `start_workers * 8`,
- both are overridable via environment (`INGEST_START_WORKERS`, `INGEST_START_QUEUE_SIZE`).

These defaults target high connection churn bursts while keeping bounded pod-level
resource usage and avoiding unbounded goroutine growth.

### 6) Horizontal autoscaling policy (recommended)
The ingest worker HPA policy is explicitly two-level:
1. in-pod bounded startup pool handles local bursts first,
2. pod replicas scale out when sustained pressure remains.

Recommended baseline in dev/prod:
- HPA `minReplicas=2`, `maxReplicas=24`,
- scale-out on CPU `65%` and memory `70%`,
- fast scale-up (`100%` or `+8 pods` per 60s),
- conservative scale-down (`20%` or `-2 pods` per 60s, 15m stabilization).

Reference manifest:
- `deploy/env/dev/recommended/pulse-services-go-ingest-hpa.recommended.yaml`

Recommended follow-up custom metrics for autoscaling:
- unassigned active device count,
- reconcile duration p95 vs poll interval,
- lease acquire latency p95.

### 7) Development seed policy
Initial seed is explicit-only (no automatic startup seed):
- read `ECOFLOW_DEV_ACCESS_KEY` / `ECOFLOW_DEV_SECRET_KEY`,
- tie to user `jpaljasma@gmail.com`,
- register initial SNs:
  - `R351ZABAPH331057`
  - `Y711ZABA9H2P0294`.

### 8) EcoFlow quota bootstrap and refresh policy
Quota bootstrap is part of the ingest contract for EcoFlow provider devices:
- workers must call `GetDeviceAllQuota(provider_device_id)` when a new ingest session starts,
- workers must refresh quota periodically with jitter while the session is alive and must attempt a best-effort quota refresh on MQTT stale/read-failure reconnect paths so projection/read models retain last-known state even when broker traffic becomes sparse,
- quota output is treated as a first-class ingest source and published into the telemetry pipeline with `source="quota"` while steady-state MQTT frames use `source="mqtt"`,
- quota payloads must normalize into the same downstream telemetry `params` shape as MQTT-derived frames so projection, rollups, and history remain source-agnostic,
- quota-derived capability/metadata snapshots must update `provider_devices.capabilities` and `provider_devices.metadata`,
- ingest workers must emit explicit quota bootstrap/refresh success and failure counters so sparse-MQTT sessions can be diagnosed without relying on payload archive inspection,
- quota frames are excluded from raw replay archive storage in v1 while still participating in projection, read models, rollups, and history.

---

## Rationale
This keeps control-plane identity/ownership stable while introducing provider-level integration entities that scale to additional vendors. Valkey leases provide low-latency distributed lock behavior with TTL safety and heartbeats, which is a better operational fit than host-local locks for multi-node worker pools.

Graceful drain as event-driven desired state avoids abrupt disconnect churn and provides deterministic worker behavior when credentials/devices are disabled.

---

## Consequences
### Positive
- Clean path to multi-provider ingestion.
- Global one-worker-per-provider-device guarantee with fault-tolerant lease expiry.
- Explicit manual discovery behavior for dev and early production.

### Negative / Tradeoffs
- Added operational complexity (lease scripts, worker orchestration).
- Additional control-plane schema and secure secret handling requirements.

### Risks & mitigations
- **Risk:** Lease split-brain on worker/network faults.  
  **Mitigation:** token-checked renew/release + short TTL + fencing counters.
- **Risk:** Duplicate payloads in at-least-once path.  
  **Mitigation:** defer strict dedup to archive/projection keys and replay repair workflow.
- **Risk:** Secret exposure in APIs/logs.  
  **Mitigation:** write-only API contract, masked outputs, and structured log redaction.

---

## Implementation plan
1. Add migration for `provider_credentials` + `provider_devices`.
2. Add control-plane repository/service methods:
   - credentials CRUD + masked list,
   - manual discovery trigger,
   - list devices grouped by provider.
3. Add provider adapter contract (EcoFlow first).
4. Build distributed ingest worker with Valkey lease manager and event-driven assignment loop.
5. Add explicit dev seed command and docs.

### Rollout / Migration
- Keep current schema and APIs working.
- Introduce provider tables with backward-compatible reads.
- Migrate ingest runtime in phases from `cmd/ecoflow-mqtt-sub` to worker service.

### Observability
- metrics:
  - lease acquire/renew/release totals and failures,
  - active sessions by provider,
  - connect/reconnect counters,
  - per-device ingest lag.
- logs:
  - structured lease lifecycle events with provider/device IDs,
  - drain reason and timing.
- alerts:
  - lease churn spikes,
  - reconnect storm,
  - no-active-worker for active provider devices.

### Security / Compliance
- secrets encrypted at rest, never returned to user clients after write.
- provider credentials scoped to owning user.
- worker credential access restricted to trusted service boundary.

---

## Acceptance criteria
- Provider credentials support multi-entry per user/provider with write-only secret reads.
- Manual discovery persists provider device metadata and links to canonical devices.
- At most one active MQTT session per `(provider, provider_device_id)` across worker replicas.
- Disable/deactivate transitions execute graceful drain.

---

## Follow-ups
- [ ] Add end-to-end dedup strategy at projection/archive stages.
- [ ] Generalize provider adapters beyond EcoFlow.
- [ ] Add chaos tests for lease loss, worker crash, and reconnect storms.
- [ ] Add autoscaling custom metrics pipeline (Prometheus Adapter/KEDA) for
  `ingest_unassigned_active_devices`, `ingest_reconcile_duration_p95_seconds`,
  and `ingest_lease_acquire_latency_p95_seconds`.
- [x] Extend provider metadata APIs/tests so quota-derived capabilities and metadata can be surfaced without provider-specific parsing at read time.
