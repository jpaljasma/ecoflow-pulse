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
- GitHub Actions (Go tests, frontend CI, CodeQL, issue auto-summary)
- Makefile task orchestration

---

## 3) Locked architecture (open-source-first, swap-friendly)

### Tiering (kept)
**Expo Client (Web/iOS/Android)**  
→ **Node REST BFF (public)**  
→ **Go gRPC Data/API layer (internal)**  
→ **Data plane** (ingest/stream/projections/storage)  
→ **WebSockets Gateway (public realtime)**

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
- **DONE:** CI merge gates = **go-test + frontend-ci + CodeQL**

---

## M0 — Platform baseline (GKE + local k3d parity)
**Goal:** “Small HA feel” everywhere; costs low; local dev is simple.

| Status | Task | Dependency |
|---|---|---|
| DONE | Create `/deploy` layout (charts + env values + Argo apps)<br>- [x] Implemented `/deploy` scaffold (local+dev only, namespaces agreed)<br>- [x] Added direct Argo apps (`pulse-platform`, `pulse-services`)<br>- [x] Added local k3d config at `deploy/tilt/k3d-config.yaml`<br>- [x] Wired initial platform chart dependencies (scaffold-first, disabled by default)<br>- [x] Pinned initial dependency chart versions (`nats`, `cloudnative-pg`, `valkey`, `keycloak`, `minio`)<br>- [x] Added `Chart.lock` for reproducible Helm dependency resolution | — |
| DONE | Local k3d cluster config + Make targets (`dev-up`)<br>- [x] Added k3d cluster config (`deploy/tilt/k3d-config.yaml`)<br>- [x] Implemented Make targets (`k3d-up`, `platform-up`, `services-up`, `dev-up`, `dev-down`)<br>- [x] Documented local bringup/down usage and defaults | — |
| TODO | Argo CD bootstrapped in GKE dev | cluster |
| TODO | ingress-nginx + cert-manager | cluster |
| TODO | External Secrets Operator (staging/prod) | cluster |
| TODO | CloudNativePG operator + base Postgres | cluster |
| TODO | TimescaleDB enablement (extension) | Postgres |
| DONE | NATS JetStream (3 replicas)<br>- [x] Added + pinned NATS dependency in `pulse-platform` chart<br>- [x] Enabled NATS JetStream (3 replicas) in `deploy/env/local/values.platform.yaml`<br>- [x] Kept dev NATS config scaffolded but disabled by default<br>- [x] Validated `make platform-up` end-to-end on local k3d cluster | cluster |
| DONE | Valkey (Sentinel, 3 pods total)<br>- [x] Enabled Valkey component in `deploy/env/local/values.platform.yaml`<br>- [x] Configured replication + Sentinel topology for 3 pods total in local k3d (`replica.replicaCount=3`, `sentinel.enabled=true`)<br>- [x] Validated with `make platform-up` + pod health checks (`kubectl get sts/pods -n pulse-platform`)<br>- [x] Documented local validation commands/results | cluster |
| TODO | Keycloak (2 replicas) | cluster |
| TODO | MinIO (local only) | local |
| TODO | Observability “lite” installed | cluster |

**Acceptance criteria**
- Local: `make dev-up` yields a working platform + services
- Dev: Argo sync yields a working platform + services
- Failover behavior can be observed (CNPG, Valkey, NATS restarts)

---

## M1 — Identity + control plane
| Status | Task | Dependency |
|---|---|---|
| TODO | Keycloak realm + Google/Facebook providers | M0 |
| TODO | Expo PKCE auth flow | Keycloak |
| TODO | Postgres schema + migrations (`users`, `devices`, `user_devices`) | M0 |
| TODO | Node JWT middleware (JWKS) | Keycloak |
| TODO | Go gRPC JWT interceptor + authz | Keycloak |
| TODO | Device registry APIs (create/link/list) | schema + auth |
| TODO | RBAC (viewer/admin) end-to-end | schema + auth |

**Acceptance criteria**
- Social login works
- Device list shows only owned/shared devices
- Forged access rejected by both REST and gRPC

---

## M2 — Telemetry pipeline v1 + archive + replay
| Status | Task | Dependency |
|---|---|---|
| TODO | `TelemetryEnvelope` protobuf + versioning | M0 |
| TODO | NATS subject model + sharding rules | M0 |
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
