# EcoFlow Pulse — Locked Architecture & Execution Plan

**Status:** ✅ *LOCKED for development start* (Feb 2026)

EcoFlow Pulse is a resilient, multi-tier real-time monitor for streaming IoT telemetry that runs on **Web + iOS + Android** (Expo universal app) and scales from early adoption to large-scale fleets. This document is the **single source of truth** for the initial build.

---

## 1) Product scope (v1)

### Core user flows
- Social login: **Google + Facebook**
- User profile & preferences
- Device list + device detail views
- Realtime dashboard: summary stats + info cards; drill-down pages
- History:
  - today, last hour, last **3/6/12/24h**, last **7d**
  - this month, last month, this year
  - **compare vs previous period**
- Upsell modules driven by data + ML inference (online suggestions now; offline later)
- Push notifications later (device alerts / upsells) — **not day-1 critical**

### Scale & telemetry assumptions (locked)
- **12-month expected devices:** ~**1,000**
- Long-term: **10k+ devices** single region; multi-region later
- Per device telemetry: **2 msg/sec**, p95 payload **375 bytes**
- Plan for **~100 metrics** per payload (max seen ~61)
- Realtime freshness target: **250ms**, allowed to degrade under load
- Tolerable drops: **0.1%** (fits 99.9% availability)

---

## 2) What exists today (preserve)

### Monorepo
- Polyglot monorepo: **Go services + Expo universal client**

### Backend & runtime (Go)
- EcoFlow API client + signing: `pkg/ecoflow`
- MQTT ingestion/subscription: `pkg/ecoflowmqtt`
- HTTP server: `pkg/ecoflowserver`, `cmd/ecoflow-server`
- Realtime runtime/subscriber: `cmd/ecoflow-mqtt-sub`
- Operational patterns: reconnect/backoff, queueing, graceful shutdown, multi-instance safety
- Compression libs: `klauspost/compress`, `andybalholm/brotli`

### Data, telemetry & ML
- Raw logs + normalized training streams
- Aggregation inputs
- CLIs for ETA/panel models, PV fingerprint, panel DB import/backfill
- Panel dataset artifacts (`*.json|*.csv`)

### Universal app (Web/iOS/Android)
- Expo + Expo Router, React Native + react-native-web
- Tamagui
- TanStack Query, Zustand, Zod
- Telemetry engine with ring buffers + snapshot clock
- Virtualization on web, charting via `@shopify/react-native-skia`
- Media assets strategy

### CI/CD
- GitHub Actions (Go tests, frontend CI, proto CI, CodeQL, issue auto-summary)
- Makefile task orchestration

---

## 3) Locked architecture (open-source-first, swap-friendly)

### Tiering (kept)
**Expo Client (Web/iOS/Android)**  
→ **Node REST BFF (public)**  
→ **Go gRPC Data/API layer (internal)**  
→ **Data plane** (ingest/stream/projections/storage)  
→ **WebSockets Gateway (public realtime)**

### Internal gRPC baseline (ADR-0013)
All internal Go gRPC services follow a shared high-throughput baseline:
- keepalive + enforcement policy (ping-flood resistant),
- HTTP/2 transport tuning (flow-control windows, stream concurrency),
- explicit message/header limits,
- required interceptor chain:
  - request-id propagation,
  - recovery,
  - auth hook,
  - structured logging,
- reflection only in local/dev,
- graceful drain on `SIGTERM`.
- mandatory regression tests for:
  - `internal/grpcserver`,
  - `internal/grpcmw`,
  - service bootstrap packages under `cmd/` (for example `cmd/ecoflow-grpc-api` telemetry behavior).
- lock/allocation discipline on hot paths:
  - avoid per-request mutex contention unless a profiler proves it is required,
  - prefer lock-free atomics for simple monotonic counters/IDs,
  - keep immutable shared defaults for response templates to reduce alloc churn.
- goroutine/channel discipline:
  - prefer bounded channel fanout for streaming update paths,
  - define explicit drop/backpressure behavior for slow consumers,
  - avoid unbounded goroutine creation under burst loads.
- performance validation gates:
  - benchmark profiles must be derived from observed telemetry (`logs/mqtt_payload_raw-*.log`),
  - include steady-state and startup-burst scenarios,
  - keep a 10k-device synthetic soak gate with p99 latency + heap-growth thresholds.

Bootstrap packages/paths:
- `internal/grpcserver` (standardized server builder + lifecycle),
- `internal/grpcmw` (middleware scaffolding),
- `cmd/ecoflow-grpc-api` (runnable bootstrap service),
- `proto/pulse/telemetry/v1` + generated code in `gen/pulse/telemetry/v1`,
- `buf.yaml` + `buf.gen.yaml` for reproducible protobuf generation.

### Provider integration + distributed ingest control plane (ADR-0014)
The control plane uses provider-aware entities while keeping ownership primitives (`users`, `devices`, `user_devices`) stable:
- `provider_credentials`: multi-entry per user/provider, write-only secret semantics for user-facing APIs, `is_active` lifecycle control.
- `provider_devices`: provider-specific identity + capability metadata, credential linkage, and ingest desired state (`active | draining | paused`).
- Distributed ingest workers use Valkey lease locks (`provider + provider_device_id`) with heartbeat TTL and graceful drain behavior.
- EcoFlow discovery is manual trigger in v1; MQTT certification is fetched on connect/reconnect.

### Technology choices (locked)
- **Cloud / K8s:** **GKE** (region **us-east1**) first, portable to EKS later
- **Streaming / queue:** **NATS JetStream**
- **Live cache:** **Valkey** (Redis-compatible) in **replication + Sentinel** mode
- **Control-plane DB:** **Postgres** (CloudNativePG operator)
- **Telemetry rollups/history v1:** **TimescaleDB** (Postgres extension)
- **Raw archive + replay source of truth:** object storage, **protobuf + zstd**
  - local: **MinIO**
  - prod: **GCS**
- **Auth:** **Keycloak** (OIDC) with Google + Facebook
- **S2S security:** mTLS via **Linkerd** (recommended; can be enabled after M1 if desired)
- **GitOps:** **Argo CD**
- **Observability:** OpenTelemetry Collector + Prometheus/Grafana; Loki/Tempo “lite” initially

### Why Valkey
- Open-source posture and compatibility with Redis OSS clients.
- Keeps cost/ops simple for v1 with a clean swap path to hosted caches later.

---

## 4) Data retention & replay (locked)

### Retention (locked)
- **Raw telemetry archive:** **30 days** (protobuf+zstd objects)
- **Minute rollups:** **90 days**
- **Hourly rollups:** **3 years**
- **Daily rollups:** **3 years**

### Replay (hard requirement)
MQTT is not queryable for historicals, so replay must be first-class.

Replay sources:
- **Authoritative:** raw archive objects (30d)
- **Operational:** JetStream retention (24–72h) for fast incident recovery

Replay modes:
- Per-device replay (single device or list)
- Fleet/system replay (by shard/time range)
- Gap repair (devices with holes in last 24h)

---

## 5) Realtime delivery (WebSockets)

A dedicated **WS Gateway** service:
- Authenticates WS (JWT)
- Authorizes per-device subscriptions (owner/guest RBAC)
- On subscribe:
  - sends snapshot from Valkey
  - streams deltas from NATS
- Enforces per-connection backpressure + downsampling:
  - **250ms → 500ms → 1s → key-metrics-only → paused**

This ensures stability without “circuit breakers”.

---

## 6) AuthN/AuthZ (authoritative requests)

- Client auth: Keycloak **Authorization Code + PKCE**
- Node REST BFF validates JWT via JWKS
- Node → Go gRPC forwards **user JWT** in metadata
- Go gRPC validates JWT again and enforces authz via:
  - `user_devices` mapping (`viewer | admin`)

This ensures:
- REST cannot be forged to access other users’ devices
- gRPC does not “trust” Node implicitly

### CI merge gates (required)
The `main` branch is protected by required CI checks. These check names are part of the architecture contract and must remain stable:

- `go-test`
- `frontend-ci`
- `proto-ci`
- `CodeQL`

If a workflow or check name changes, update repository branch protection/rulesets in the same change so merges are never silently unblocked.

---

## 7) Local dev principle (keep it very simple)

**Local runs on Kubernetes too** and mirrors dev behavior:
- local cluster: **k3d**
- one command to bring up platform deps + core services
- services default to in-cluster execution (parity > cleverness)

Suggested developer commands (you’ll implement in Makefile):
- `make k3d-up` — create local cluster
- `make platform-up` — install platform umbrella chart (NATS/CNPG/Valkey/Keycloak/MinIO)
- `make services-up` — install services umbrella chart
- `make dev-up` — does all three
- `make dev-down` — uninstall + optionally delete cluster

Optional (not required): Tilt for hot-reload convenience.

### When to use GKE dev (cost-min policy)
Use GKE dev only for cloud-only validation:

1. OAuth/social redirect flows on real domains/devices
2. ingress + TLS + cert-manager behavior
3. Workload Identity and External Secrets behavior
4. GCS lifecycle/retention checks for archive flows
5. autoscaling/node lifecycle behavior and Argo CD cloud sync realism

Everything else should run on local k3d first.  
When not actively testing in GKE dev, park workloads and reduce node-pool mins.

---

# 8) Milestones & status board

Legend: **TODO | PROGRESS | DONE | HELP**

## Decisions log (DONE)
- **DONE:** Cloud = **GKE**
- **DONE:** Region = **us-east1**
- **DONE:** Streaming bus = **NATS JetStream**
- **DONE:** Cache = **Valkey** (replication + Sentinel)
- **DONE:** DB = Postgres + TimescaleDB (v1)
- **DONE:** Archive = **protobuf + zstd**
- **DONE:** Raw retention = **30 days**
- **DONE:** Minute rollups retention = **90 days**
- **DONE:** Hourly/daily rollups retention = **3 years**
- **DONE:** Realtime = **WebSockets**
- **DONE:** Auth = **Keycloak** w/ Google & Facebook
- **DONE:** CI merge gates = **go-test + frontend-ci + proto-ci + CodeQL**

---

## M0 — Platform baseline (GKE + local k3d parity)
**Goal:** “Small HA feel” everywhere; costs low; local dev is simple.

| Status | Task | Dependency |
|---|---|---|
| DONE | Create `/deploy` layout (charts + env values + Argo apps)<br>- [x] Implemented `/deploy` scaffold (local+dev only, namespaces agreed)<br>- [x] Added direct Argo apps (`pulse-platform`, `pulse-services`)<br>- [x] Added local k3d config at `deploy/tilt/k3d-config.yaml`<br>- [x] Wired initial platform chart dependencies (scaffold-first, disabled by default)<br>- [x] Pinned initial dependency chart versions (`nats`, `cloudnative-pg`, `valkey`, `keycloak`, `minio`)<br>- [x] Added `Chart.lock` for reproducible Helm dependency resolution | — |
| DONE | Local k3d cluster config + Make targets (`dev-up`)<br>- [x] Added k3d cluster config (`deploy/tilt/k3d-config.yaml`)<br>- [x] Implemented Make targets (`k3d-up`, `platform-up`, `platform-wait`, `services-up`, `services-wait`, `dev-up`, `dev-down`)<br>- [x] Hardened `platform-up` with retry/backoff for transient CNPG webhook race conditions<br>- [x] Enforced startup ordering in `dev-up` (`platform-wait` before `services-up`) so developers do not need double-runs<br>- [x] Documented local bringup/down usage and defaults | — |
| DONE | Argo CD bootstrapped in GKE dev<br>- [x] Add GKE Argo CD bootstrap Make targets (`argocd-bootstrap-dev`, app apply/sync validation)<br>- [x] Add Argo CD dev values profile with cost-min defaults<br>- [x] Document bootstrap + validation commands for developers<br>- [x] Validate end-to-end against real GKE dev cluster and capture output (`ecoflow-pulse-dev-260221-01`, `make argocd-dev-up`, apps reached `Synced + Healthy`) | cluster |
| DONE | ingress-nginx + cert-manager<br>- [x] Add and pin Helm dependencies (`ingress-nginx`, `cert-manager`) in `pulse-platform` umbrella chart<br>- [x] Add local/dev values scaffolding and component toggles (`components.ingressNginx`, `components.certManager`)<br>- [x] Extend `platform-wait` readiness checks for ingress + cert-manager controllers<br>- [x] Validate local render/install path (`helm dependency update`, `helm lint` with local/dev values)<br>- [x] Validate GKE dev Argo sync with cert-manager CRDs and ingress controller ready (temporary branch-target app patch + hard refresh, then restore `targetRevision=main`) | cluster |
| DONE | External Secrets Operator (staging/prod)<br>- [x] Added and pinned `external-secrets` chart dependency in `deploy/charts/pulse-platform/Chart.yaml`<br>- [x] Added component toggle scaffolding (`components.externalSecrets`) and base values in `deploy/charts/pulse-platform/values.yaml`<br>- [x] Added local/dev values wiring (`deploy/env/local/values.platform.yaml`, `deploy/env/dev/values.platform.yaml`)<br>- [x] Extended `make platform-wait` readiness checks for ESO controller/webhook/cert-controller deployments<br>- [x] Validated local render path (`helm dependency update`, `helm lint` local/dev values)<br>- [x] Validated GKE dev Argo sync + readiness and recorded output (`pulse-platform`/`pulse-services`: `Synced+Healthy`; ESO controller/webhook/cert-controller `1/1` Ready) | cluster |
| DONE | CloudNativePG operator + base Postgres<br>- [x] Enabled CloudNativePG operator in `deploy/env/local/values.platform.yaml` (`components.cloudnativepg.enabled=true`)<br>- [x] Added base CNPG Postgres cluster manifests to `pulse-platform` chart (`cloudnativepgCluster` values + `Cluster` CR + app credentials secret template)<br>- [x] Configured local base Postgres HA footprint (`instances=2`, `storage=10Gi`, resource requests/limits per Config-03 defaults)<br>- [x] Added CRD-safe reconcile workflow in `make platform-up` (wait for CNPG operator, then second Helm pass for CRD-backed resources)<br>- [x] Validated operator + cluster readiness in local k3d (`make platform-up`, `kubectl get clusters.postgresql.cnpg.io -n pulse-platform`, `kubectl get pods -n pulse-platform`)<br>- [x] Wired service-facing connection contract for `pulse-platform-core-rw` (`pulse-platform-core-contract` ConfigMap + `pulse-platform-core-connection` Secret)<br>- [x] Switched Keycloak from bundled Postgres to CNPG base Postgres (`keycloak.postgresql.enabled=false`, `keycloak.externalDatabase.*` to `pulse-platform-core-rw`) | cluster |
| DONE | TimescaleDB enablement (extension)<br>- [x] Added TimescaleDB configuration to CNPG cluster values (`cloudnativepgCluster.timescaledb.*`)<br>- [x] Enabled local TimescaleDB component wiring (`components.timescaledb.enabled=true`)<br>- [x] Added CNPG `ImageCatalog` + `imageCatalogRef` support for Timescale image selection (major `18`), plus `postInitApplicationSQL` extension bootstrap in app DB<br>- [x] Added Timescale-specific runtime UID/GID wiring and `postgresql.shared_preload_libraries: [timescaledb]` in cluster template<br>- [x] Validated cluster reconciliation after image switch (`make platform-up`, `make platform-wait`)<br>- [x] Validated extension installation and preload on primary (`SHOW shared_preload_libraries;`, `SELECT extname FROM pg_extension`) | Postgres |
| DONE | NATS JetStream (3 replicas)<br>- [x] Added + pinned NATS dependency in `pulse-platform` chart<br>- [x] Enabled NATS JetStream (3 replicas) in `deploy/env/local/values.platform.yaml`<br>- [x] Kept dev NATS config scaffolded but disabled by default<br>- [x] Validated `make platform-up` end-to-end on local k3d cluster | cluster |
| DONE | Valkey (Sentinel, 3 pods total)<br>- [x] Enabled Valkey component in `deploy/env/local/values.platform.yaml`<br>- [x] Configured replication + Sentinel topology for 3 pods total in local k3d (`replica.replicaCount=3`, `sentinel.enabled=true`)<br>- [x] Validated with `make platform-up` + pod health checks (`kubectl get sts/pods -n pulse-platform`)<br>- [x] Documented local validation commands/results | cluster |
| DONE | Keycloak (2 replicas)<br>- [x] Enabled Keycloak component in `deploy/env/local/values.platform.yaml`<br>- [x] Configured local Keycloak with 2 replicas using external CNPG Postgres (`pulse-platform-core-rw` via `keycloak.externalDatabase.*`)<br>- [x] Added local image repository overrides to `bitnamilegacy/*` to avoid pull failures during local bringup<br>- [x] Set explicit Keycloak container resources (`requests: 250m/1Gi`, `limits: 1 CPU/2Gi`) to prevent OOM restarts in k3d<br>- [x] Validated with `make platform-up` + `kubectl get pods -n pulse-platform` (both `pulse-platform-keycloak-0` and `pulse-platform-keycloak-1` Ready)<br>- [x] Documented node recovery step after Docker restarts (`docker restart k3d-pulse-local-agent-0`) | cluster |
| DONE | MinIO (local only)<br>- [x] Enabled MinIO component in `deploy/env/local/values.platform.yaml`<br>- [x] Configured local MinIO for standalone + ephemeral mode (`mode=standalone`, `replicas=1`, `persistence.enabled=false`)<br>- [x] Tuned local resource requests to schedule in k3d (`memory=512Mi`, `cpu=100m`)<br>- [x] Validated with `make platform-up` + pod health checks (`kubectl get deploy/pods -n pulse-platform`) | local |
| DONE | Observability “lite” installed<br>- [x] Added and pinned `kube-prometheus-stack` and `opentelemetry-collector` chart dependencies in `deploy/charts/pulse-platform/Chart.yaml`<br>- [x] Added `components.observabilityLite` toggle + low-footprint defaults in `deploy/charts/pulse-platform/values.yaml`<br>- [x] Added dev values profile with observability enabled and constrained resources (`deploy/env/dev/values.platform.yaml`)<br>- [x] Added required OTel collector image repository wiring for chart renderability (`otel/opentelemetry-collector-k8s`)<br>- [x] Extended `make platform-wait` readiness checks for Grafana, Prometheus operator, and OTel collector deployments<br>- [x] Validated local render path (`helm dependency update`, `helm lint` local/dev values)<br>- [x] Validated GKE dev Argo sync + readiness and recorded output (`pulse-platform`/`pulse-services`: `Synced+Healthy`; Grafana + Prometheus operator + OTel collector running; Prometheus StatefulSet ready after hard refresh) | cluster |

**Acceptance criteria**
- [x] Local: `make dev-up` yields a working platform + services (`k3d-pulse-local`; local Make targets pinned to k3d context to avoid accidental GKE applies)
- [x] Dev: Argo sync yields a working platform + services (`pulse-platform` and `pulse-services` = `Synced + Healthy` in `argocd` namespace)
- [x] Failover behavior can be observed (CNPG, Valkey, NATS restarts)
  - CNPG failover validated: cluster returned to `Ready=True` with 2 healthy instances after primary restart
  - NATS validated: StatefulSet recovered to `3/3`
  - Valkey validated: StatefulSet recovered to `3/3`

---

## M1 — Identity + control plane
| Status | Task | Dependency |
|---|---|---|
| PROGRESS | Internal Go gRPC baseline bootstrap (ADR-0013)<br>- [x] Imported shared gRPC server builder (`internal/grpcserver`) with keepalive enforcement, HTTP/2 tuning, stream/message limits, reflection-gated-by-env, and graceful SIGTERM drain<br>- [x] Imported standard middleware scaffold (`internal/grpcmw`): request-id, recovery, auth hook, structured logging<br>- [x] Added bootstrap telemetry service (`proto/pulse/telemetry/v1`) + generated stubs under `gen/pulse/telemetry/v1` via Buf<br>- [x] Added runnable server entrypoint (`cmd/ecoflow-grpc-api`) with health service and telemetry registration<br>- [x] Added regression tests for `internal/grpcserver`, `internal/grpcmw`, and `cmd/ecoflow-grpc-api` telemetry service behavior to prevent baseline regressions<br>- [x] Added workload-calibrated soak/benchmark coverage and GC/pprof profiling workflow based on `logs/mqtt_payload_raw-*.log` characteristics<br>- [x] Added 10k-device synthetic fleet soak benchmark with p99 latency and heap-growth thresholds (opt-in)<br>- [ ] Replace `NoopAuthorizer` with Keycloak JWKS validation + `user_devices` RBAC enforcement at Go boundary (`ControlPlaneService`) | M0 |
| PROGRESS | Keycloak realm + Google/Facebook providers<br>- [x] Added chart-managed Keycloak realm import ConfigMap template (`deploy/charts/pulse-platform/templates/keycloak-realm-configmap.yaml`)<br>- [x] Added chart-managed social provider secret template (`deploy/charts/pulse-platform/templates/keycloak-social-secret.yaml`)<br>- [x] Wired local values to enable `keycloakConfigCli` with `existingConfigmap` + `extraEnvVarsSecret`<br>- [x] Added local verification target (`make auth-keycloak-verify-local`) to assert realm + provider presence via `kcadm`<br>- [x] Validate local end-to-end (`make platform-up` + `make platform-wait` + `make auth-keycloak-verify-local`)<br>- [x] Add dev secrets contract + provider credential bootstrap doc (`docs/how-to/configure-keycloak-social-providers-local.md`) | M0 |
| TODO | Expo PKCE auth flow | Keycloak |
| PROGRESS | Postgres schema + migrations (`users`, `devices`, `user_devices`)<br>- [x] Added initial migration scaffold under `deploy/db/migrations` (`000001_m1_control_plane_schema.up.sql`, `.down.sql`)<br>- [x] Applied UUIDv7 IDs (`uuidv7()`) across `users` and `devices`<br>- [x] Enforced `keycloak_subject` unique+required and `ecoflow_sn` unique+global<br>- [x] Added `user_devices` composite PK (`user_id`, `device_id`) with role check (`viewer/admin`)<br>- [x] Locked app-managed UTC timestamps (`created_at`, `updated_at`) with no DB defaults<br>- [x] Added migration apply/verify command path for local k3d CNPG DB (`make db-migrate-up-local`, `make db-migrate-down-local`, `make db-migrate-verify-local`, `make db-migrate-cycle-local`, `make db-migrate-e2e-local`)<br>- [x] Validated migration end-to-end against local platform DB (`make db-migrate-cycle-local` + `make db-migrate-e2e-local`; verified `uuidv7()` ID default, check constraints, uniqueness constraints, ownership join path, and app-managed timestamp columns) | M0 |
| DONE | Provider integrations + device metadata control-plane model (ADR-0014)<br>- [x] Locked architecture decision for provider-scoped credentials + provider device identity (`provider_credentials`, `provider_devices`)<br>- [x] Locked write-only secret policy for user-facing credential APIs<br>- [x] Locked manual discovery-only behavior for v1 (`DiscoverDevices()` explicit trigger)<br>- [x] Locked lease protocol choice (Valkey TTL + heartbeat + graceful drain)<br>- [x] Added migration files for `provider_credentials` + `provider_devices` (`000002_m1_provider_integrations_schema`) with UUIDv7 + UTC app-managed timestamps<br>- [x] Validate provider integration migration cycle/e2e on local CNPG (`make db-migrate-cycle-local` + `make db-migrate-e2e-local`)<br>- [x] Add control-plane APIs: credentials CRUD/list, `DiscoverDevices()`, provider-grouped `ListDevices()`<br>  - [x] Define protobuf contract for `ControlPlaneService` and generate stubs<br>  - [x] Implement service scaffolding + store abstraction in `cmd/ecoflow-grpc-api`<br>  - [x] Implement credential CRUD/list path (write-only secret semantics)<br>  - [x] Implement provider-grouped `ListDevices()` using ownership joins<br>  - [x] Add manual `DiscoverDevices()` trigger stub with provider hook contract<br>- [x] Add explicit dev seed command using `ECOFLOW_DEV_ACCESS_KEY` / `ECOFLOW_DEV_SECRET_KEY` and initial SN bindings (`cmd/ecoflow-dev-seed` + `make db-seed-dev-local`) | M1 schema + ADR-0014 |
| TODO | Node JWT middleware (JWKS) | Keycloak |
| TODO | Go gRPC JWT interceptor + authz | Keycloak |
| TODO | Device registry APIs (create/link/list) | schema + auth |
| TODO | RBAC (viewer/admin) end-to-end | schema + auth |

**Acceptance criteria**
- Social login works
- Device list shows only owned/shared devices
- Forged access rejected by both REST and gRPC

**Post-M1 follow-up (explicitly deferred until M1 is DONE)**
- Add GitHub `db-migrations-ci` workflow (up/verify/down/up/e2e) and make it a required check.
- Define and implement schema migration rollout path for `dev -> staging -> prod` (Argo sync hook / migration job sequencing, backup gates, forward-only policy).
- Adopt `pgroll` for safe reversible online PostgreSQL schema migrations with simultaneous multi-schema serving during transitions.
- Keep M1 implementation local-first: continue validating schema changes in local k3d/CNPG before enabling environment rollout automation.
- Add Helm/Make startup sequencing hardening for Keycloak bootstrap dependencies to reduce first-boot restart/transient not-ready events.

---

## M2 — Telemetry pipeline v1 + archive + replay
**Execution sequence (M2 closeout order)**

- [x] Step 0: `TelemetryEnvelope` + deterministic NATS subject/shard model
- [x] Step 1: Ingest durability hardening (JetStream ingest stream bootstrap + publish retry/backpressure policy)
- [ ] Step 2: Projection pipeline (`pulse.telemetry.ingest.*` → Valkey live snapshot)
  - [x] Added projection worker baseline (`cmd/ecoflow-projection-worker`)
  - [x] Added JetStream queue consumer on ingest wildcard subjects with durable/ack tuning
  - [x] Added envelope projection (numeric metrics extraction + stale/duplicate guard)
  - [x] Added Valkey live snapshot store with cluster-aware key tags and cursor sequence
  - [x] Added explicit read-side snapshot contract (`SnapshotReader` / `SnapshotReadModel`) for downstream realtime/query consumers
  - [x] Wired gRPC `TelemetryService.GetSnapshot` to use Valkey-backed snapshot reader contract when configured
  - [x] Added end-to-end checkpoint/recovery tests (worker restart + stale replay idempotency + cursor monotonicity)
  - [ ] Wire snapshot contract into downstream WS gateway/query APIs
- [x] Step 3: Raw archive writer (protobuf+zstd objects to MinIO/GCS)
  - [x] Added archive worker baseline (`cmd/ecoflow-archive-worker`)
  - [x] Added JetStream queue consumer on ingest wildcard subjects with durable/manual-ack semantics
  - [x] Added shard/hour object partitioning with protobuf length-delimited frames + zstd compression
  - [x] Added MinIO-compatible object writer (`internal/archiveworker`) with metadata/checksum tagging
  - [x] Added archive worker docs + Make target (`make archive-worker`)
  - [x] Add object-store integration tests (MinIO/GCS parity + failure injection)
- [x] Step 4: Manifest index persistence (Postgres metadata for replay lookup)
  - [x] Added migration `000003_m2_archive_manifest_schema` with UUIDv7 primary key, app-managed UTC timestamps, object uniqueness, and replay lookup indexes
  - [x] Added Postgres manifest store (`internal/archiveworker/manifest_store.go`) with idempotent upsert on `(object_bucket, object_key)`
  - [x] Wired archive writer flush path to persist manifest metadata after object write and before ACK
  - [x] Added unit coverage for manifest normalization/validation and manifest write failure (NAK/retry path)
  - [x] Extended local migration verification to include manifest table + constraints
- [ ] Step 5: Replay CLI (device list + shard/time replay modes)
- [ ] Step 6: Gap detector + replay enqueue workflow

| Status | Task | Dependency |
|---|---|---|
| PROGRESS | `TelemetryEnvelope` protobuf + versioning<br>- [x] Added canonical ingest/archive envelope schema at `proto/pulse/envelope/v1/envelope.proto`<br>- [x] Added explicit version fields (`envelope_version`, `payload_version`) and source/encoding enums<br>- [x] Added deterministic shard metadata fields (`shard`, `shard_count`) to support replay partitioning<br>- [x] Generated Go stubs via Buf (`gen/pulse/envelope/v1`)<br>- [x] Wired envelope builder into ingest path (`MQTT -> TelemetryEnvelope -> NATS`) via `internal/ingestworker/envelope_builder.go` | M0 |
| PROGRESS | NATS subject model + sharding rules<br>- [x] Added deterministic shard function (`ShardForDevice`) using stable FNV-1a hashing in `internal/telemetrybus`<br>- [x] Added versioned subject taxonomy helpers for ingest/projection/archive/replay/gap-repair subjects<br>- [x] Added regression tests for subject formatting + shard determinism<br>- [x] Added NATS envelope publisher and shared publish helper (`internal/telemetrybus/publisher.go`) for ingest subjects<br>- [x] Wired projection worker subscription to shared `internal/telemetrybus` wildcard ingest subject helper<br>- [ ] Wire replay workers to shared `internal/telemetrybus` subject helpers | M0 |
| PROGRESS | Distributed MQTT ingest worker pool + global lease locks (ADR-0014)<br>- [x] Locked one-session-per-`(provider, provider_device_id)` rule for distributed workers<br>- [x] Locked Valkey lease protocol baseline (`TTL=45s`, `heartbeat=15s`, tuned by soak testing)<br>- [x] Locked graceful drain behavior for deactivation/credential disable events (event-driven)<br>- [x] Implement provider adapter contract (`EcoFlow` first) for connect/reconnect certification flow<br>  - [x] Added lean real EcoFlow adapter (`internal/provideradapter`) with `DiscoverDevices` + per-device certification guard<br>  - [x] Wired `cmd/ecoflow-grpc-api` to register the real EcoFlow discoverer by default<br>  - [x] Added unit tests for adapter mapping/validation/certification behavior and opt-in seeded integration verification<br>- [x] Implement Valkey Lua lease manager (acquire/renew/release with token + fencing)<br>  - [x] Added ADR-0014 keying implementation (`lease/session/fence`) with cluster hash-tag safety in `internal/ingestlease`<br>  - [x] Added token-checked Lua scripts for acquire/renew/release + graceful drain support (`TTL=45s`, `heartbeat=15s` defaults)<br>  - [x] Added `valkey-go` client bootstrap tuned for lease path resilience (cluster refresh + MOVED/ASK handling + cache explicitly disabled for locks)<br>  - [x] Added concurrency-heavy unit tests (single-winner contention, fencing increment, heartbeat drain/release lifecycle)<br>- [x] Implement worker assignment loop and session lifecycle manager (multi-replica safe)<br>  - [x] Added internal assignment poller + reconciler (`internal/ingestworker`) keyed by `(provider, provider_device_id)`<br>  - [x] Polls `provider_devices` and starts sessions only when `is_active=true`, `credential.is_active=true`, and `ingest_desired_state=active`<br>  - [x] Claims global lease before session start and maintains lease heartbeat until session stop<br>  - [x] Stops sessions cleanly on `draining` / `paused` / credential disable / device disable / assignment removal<br>  - [x] Added runnable distributed worker entrypoint (`cmd/ecoflow-ingest-worker`)<br>  - [x] Added concrete startup pool defaults (`start_workers=clamp(4*GOMAXPROCS,8,64)`, `start_queue_size=start_workers*8`) with env overrides (`INGEST_START_WORKERS`, `INGEST_START_QUEUE_SIZE`)<br>  - [x] Added recommended HPA policy manifest for ingest workers (`deploy/env/dev/recommended/pulse-services-go-ingest-hpa.recommended.yaml`)<br>- [ ] Replace production ingest path dependency on host-local file lock runtime | M0 + ADR-0014 |
| PROGRESS | Ingest: MQTT → normalize → NATS<br>- [x] Ingest worker now resolves MQTT certification per connect/reconnect and opens one MQTT session per leased provider device<br>- [x] MQTT payloads are normalized into `TelemetryEnvelope` (UUIDv7 envelope id, shard metadata, labels, payload encoding, source/type mapping)<br>- [x] Envelopes are published to `pulse.telemetry.ingest.sNNN` via `internal/telemetrybus` NATS publisher<br>- [x] Added session-runner/publisher/envelope unit tests to prevent ingest regressions<br>- [x] Step 1.1 JetStream stream bootstrap (`PULSE_TELEMETRY_INGEST`) from worker startup<br>- [x] Step 1.2 NATS publish retry/backpressure policy (bounded timeout + jittered retries)<br>- [x] Step 1.3 Worker env knobs/docs for JetStream bootstrap + publish retry tuning<br>- [x] Step 1.4 Failure-mode tests (stream add/update/no-op + retry success/exhaust/cancel) | proto + NATS |
| PROGRESS | Projection: NATS → Valkey live snapshot<br>- [x] Added projection worker command (`cmd/ecoflow-projection-worker`) for distributed consumer runtime<br>- [x] Added JetStream queue consumer with durable/manual-ack semantics on ingest wildcard subjects<br>- [x] Added telemetry projection store (`internal/projectionworker`) that merges numeric metrics into per-device live snapshots<br>- [x] Added stale/duplicate envelope guards (envelope id + ingest timestamp checks)<br>- [x] Added cluster-aware Valkey keying for atomic snapshot/cursor groups (`{did:*}` / `{sn:*}` tags)<br>- [x] Added explicit downstream read-model contract (`SnapshotReader` / `SnapshotReadModel`) and wired gRPC snapshot reads to it when Valkey is configured<br>- [x] Added projection replay/gap-repair contract validation + end-to-end checkpoint/recovery tests (worker restart + stale replay idempotency + cursor monotonicity)<br>- [ ] Add WS/query consumer integration over the same snapshot read-model contract | NATS + Valkey |
| PROGRESS | Archive writer: protobuf+zstd objects<br>- [x] Added archive worker runtime (`cmd/ecoflow-archive-worker`)<br>- [x] Added archive pipeline package (`internal/archiveworker`) with JetStream durable consumer + graceful drain<br>- [x] Added shard/hour object keying (`raw/yyyy=.../hh=.../shard=.../part-...pb.zst`)<br>- [x] Added length-delimited protobuf framing + zstd compression and per-object metadata/checksum<br>- [x] Added unit coverage for ack/nak/term paths, interval flush, and frame decode verification<br>- [x] Added object-store integration tests (MinIO contract + failure injection, parity-ready harness) | storage |
| PROGRESS | Manifest index table (Postgres)<br>- [x] Added `archive_object_manifest` schema migration with replay lookup indexes<br>- [x] Added archive manifest upsert store and worker wiring<br>- [x] Added unit tests for manifest normalization + flush failure semantics<br>- [x] Added local migration verify checks for manifest constraints | M1 DB |
| TODO | Replay CLI: device list + shard/time | archive + manifest |
| TODO | Gap detector + replay enqueue | projections |

**Acceptance criteria**
- Replay can rebuild live state for last 24h
- Targeted gap repair works without replaying everything

---

## M3 — Rollups + history queries + comparisons
| Status | Task | Dependency |
|---|---|---|
| TODO | Timescale hypertables + indexes | M0 |
| TODO | Rollup pipeline (minute/hour/day) | M2 |
| TODO | Retention jobs: minute 90d, hour/day 3y | Timescale |
| TODO | Go gRPC query APIs (range + compare) | rollups |
| TODO | Node REST endpoints → gRPC | gRPC |

**Acceptance criteria**
- All time windows supported
- Prior-period comparison returned in a single call (server-side)

---

## M4 — WebSockets gateway + UX hardening
| Status | Task | Dependency |
|---|---|---|
| TODO | WS Gateway (auth, authz, subscribe protocol) | M1 |
| TODO | Snapshot-on-connect (Valkey) + deltas (NATS) | M2 |
| TODO | Backpressure + downsampling ladder | WS |
| TODO | Expo client WS integration + reconnection UX | WS |

**Acceptance criteria**
- 250ms updates when possible
- Graceful degradation under pressure
- Robust reconnect/resubscribe

---

## M5 — Testing + operability + DR-lite
| Status | Task | Dependency |
|---|---|---|
| TODO | Integration tests (Testcontainers: Postgres/Valkey/NATS/MinIO) | M2 |
| TODO | Contract tests (Node↔Go proto compatibility) | M1/M2 |
| TODO | E2E web (Playwright) | M4 |
| TODO | E2E mobile smoke (Maestro) | M4 |
| TODO | Load tests (k6): ingest + WS + query | M4 |
| TODO | Backups/restore drills + runbooks | M0+M1 |
| TODO | Alerts: lag/replay/auth/archive failures | M0 |

**Acceptance criteria**
- Releases gated by a small E2E suite
- Restore drills succeed
- Failure simulations show clean recovery

---

## M6 — Online ML recommendations (v1)
| Status | Task | Dependency |
|---|---|---|
| TODO | Define online inference contract (gRPC) | M2 |
| TODO | Insights stream/projection | M2 |
| TODO | Wire upsell modules to inference output | M4 |

---

# 9) Lock statement

**This plan is locked** for development start:
- build the platform baseline (M0)
- then identity/control plane (M1)
- then telemetry+replay (M2)
- then rollups+history (M3)
- then realtime UX (M4)
- then tests/ops hardening (M5)

Any changes should be logged as a deliberate decision (with rationale) and rolled into this document.
