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
- Keep M1 implementation local-first: continue validating schema changes in local k3d/CNPG before enabling environment rollout automation.

---

## M2 — Telemetry pipeline v1 + archive + replay
| Status | Task | Dependency |
|---|---|---|
| PROGRESS | `TelemetryEnvelope` protobuf + versioning<br>- [x] Added canonical ingest/archive envelope schema at `proto/pulse/envelope/v1/envelope.proto`<br>- [x] Added explicit version fields (`envelope_version`, `payload_version`) and source/encoding enums<br>- [x] Added deterministic shard metadata fields (`shard`, `shard_count`) to support replay partitioning<br>- [x] Generated Go stubs via Buf (`gen/pulse/envelope/v1`)<br>- [ ] Wire envelope builder into ingest path (`MQTT -> normalize -> NATS`) | M0 |
| PROGRESS | NATS subject model + sharding rules<br>- [x] Added deterministic shard function (`ShardForDevice`) using stable FNV-1a hashing in `internal/telemetrybus`<br>- [x] Added versioned subject taxonomy helpers for ingest/projection/archive/replay/gap-repair subjects<br>- [x] Added regression tests for subject formatting + shard determinism<br>- [ ] Wire ingest/projection/replay workers to shared `internal/telemetrybus` subject helpers | M0 |
| TODO | Ingest: MQTT → normalize → NATS | proto + NATS |
| TODO | Projection: NATS → Valkey live snapshot | NATS + Valkey |
| TODO | Archive writer: protobuf+zstd objects | storage |
| TODO | Manifest index table (Postgres) | M1 DB |
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
