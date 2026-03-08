# Ecoflow-Pulse

Ecoflow-Pulse is a real-time pulse monitor for EcoFlow devices, built on the
official API and MQTT telemetry streams.

## Supported Devices

Actively validated:

- DELTA 2 Max (D2M)
- DELTA Pro Ultra (DPU)

## Latest Implemented Features (Mar 2026)

- M1 closed: Keycloak OIDC + Expo PKCE auth, Node JWKS middleware, Go gRPC JWT
  auth/interceptors, and `viewer/admin` RBAC for device registry APIs.
- M2 closed: distributed ingest workers with Valkey lease/fencing, canonical
  `TelemetryEnvelope` publish to JetStream, Valkey projection snapshots, raw
  archive + manifest index, replay CLI, and targeted gap repair.
- M3 closed: Timescale minute/hour/day rollup pipeline with retention policies,
  gRPC range/compare query APIs, and public Node REST history endpoints.
- M4 closed: dedicated realtime WebSocket gateway (snapshot-on-connect + NATS
  deltas) with backpressure/downsampling ladder and Expo reconnect hardening.
- M5 in progress with key slices shipped: Testcontainers pipeline integration
  suite, Node↔Go protobuf contract tests, and Playwright web E2E smoke tests.
- Solar history UI now ships a `06:00-20:00` local comparison chart with
  10-minute buckets, prior-day overlay, and hover/tap bucket inspection.
- `Energy Impact` now estimates today-so-far avoided `CO2e`, `NOx`, and `SO2`
  plus conservative mature-tree equivalent on `/devices` and `/device/{id}`
  using versioned EPA eGRID2023 and lifecycle/tree factors plus an in-app
  explainer, with a lazy cached `Past 12 months` view in addition to the live
  `Today so far` default.

## Core Capabilities

- Live terminal dashboard for power, SOC, states, and per-pack battery telemetry.
- Expo universal dashboard (Web/iOS/Android) with auth-aware realtime + history.
- Persistent telemetry history with minute/hour/day Timescale rollups.
- Server-side prior-period comparison APIs for history windows.
- Dedicated realtime WS delivery (snapshot-first, then deltas) with graceful
  degradation under pressure.
- Durable replay model: protobuf+zstd archive, manifest index, replay CLI, and
  targeted gap repair.
- ETA estimation with MPPT, profile-specific ML, and generic ML model fallback.
- Solar telemetry and MPPT visibility (low/high inputs, volts/amps/watts, state).
- Today-so-far avoided-emissions estimate for solar generation with EPA-based
  methodology and in-app explanation.
- Solar panel detection and upgrade recommendations using panel database + model.
- Safe runtime behavior for reconnects, bounded queues, and distributed lease
  ownership across ingest worker replicas.

## Platform Features (Enabled)

- Kubernetes-first local stack (`k3d`) with Helm-managed `pulse-platform` and
  `pulse-services` releases.
- Core platform dependencies running in-cluster:
  - CloudNativePG Postgres (with Timescale extension support)
  - NATS JetStream cluster (telemetry streams + work queues)
  - Valkey cluster (live snapshots + distributed ingest leases)
  - MinIO object storage (raw telemetry archive)
  - Keycloak (auth plane, local/dev)
- Distributed ingest runtime:
  - one MQTT session globally per `(provider, provider_device_id)` via Valkey
    lease + fencing
  - EcoFlow certification-based reconnect flow per device
  - startup quota bootstrap + periodic metadata/capability refresh publish path
  - sharded `TelemetryEnvelope` publish to NATS (`pulse.telemetry.ingest.sNNN`)
- Live read model:
  - projection worker consumes NATS and writes Valkey snapshots/cursors
  - gRPC snapshot and realtime subscription flow from the same read model
- Rollup and history model:
  - rollup worker upserts minute/hour/day Timescale buckets from ingest stream
  - retention policies: minute=90d, hour/day=3y
- Public query/API layer:
  - Node REST BFF (`apps/pulse-platform`) validates JWTs and forwards history
    range/compare queries to internal gRPC
  - dedicated realtime WS gateway (`apps/pulse-realtime-gateway`) serves
    snapshot-first stream + per-session backpressure ladder
- Durable archive and replay:
  - archive worker writes protobuf+zstd objects to MinIO
  - manifest index persisted in Postgres for replay lookup
  - replay CLI supports `list-devices`, `device`, and `fleet` modes
  - gap detector + gap-repair worker perform targeted replay, not full replays
- Shipped quality gates:
  - Testcontainers pipeline integration suite (`make test-pipeline-integration`)
  - Node↔Go proto compatibility tests (`make test-proto-contract`)
  - Playwright web E2E smoke (`make test-web-e2e`)
  - Maestro mobile E2E smoke (`MAESTRO_EXPO_URL='exp://127.0.0.1:8081' make test-mobile-e2e`)

## Quick Start

```bash
go test ./...
make mqtt
```

## Platform Quick Start (Local)

```bash
make dev-up
make platform-wait services-wait
```

This starts the full platform and worker pipeline in local `k3d`.

## Documentation

Developer docs follow Diataxis under `/docs`:

- [Developer Documentation Index](docs/README.md)
- [Architecture (Locked Plan)](docs/architecture/README.md)
- [Architecture Decision Records (ADR Index)](docs/architecture/adr/README.md)
- [Run MQTT Dashboard](docs/how-to/run-mqtt-dashboard.md)
- [Configuration Reference](docs/reference/configuration.md)
- [Telemetry Model](docs/reference/telemetry-model.md)
- [Solar Avoided Emissions Reference](docs/reference/solar-avoided-emissions.md)
- [Tree Equivalent Reference](docs/reference/tree-equivalent.md)
- [Commands Reference](docs/reference/commands.md)

## Repository Layout

- `apps/pulse-platform`: public Node REST BFF over internal gRPC query APIs
- `apps/pulse-realtime-gateway`: public WS gateway (JWT auth, Valkey snapshot,
  NATS delta fanout, backpressure ladder)
- `apps/universal`: Expo universal dashboard (Web/iOS/Android)
- `cmd/ecoflow-mqtt-sub`: real-time MQTT telemetry dashboard
- `cmd/ecoflow-grpc-api`: internal high-throughput gRPC API
- `cmd/ecoflow-ingest-worker`: distributed MQTT ingest worker
- `cmd/ecoflow-projection-worker`: NATS -> Valkey live snapshot projector
- `cmd/ecoflow-rollup-worker`: ingest -> Timescale minute/hour/day rollup worker
- `cmd/ecoflow-archive-worker`: raw archive writer (protobuf+zstd to MinIO)
- `cmd/ecoflow-replay-cli`: replay CLI (`list-devices`, `device`, `fleet`)
- `cmd/ecoflow-gap-detector`: targeted gap detection and replay enqueue
- `cmd/ecoflow-gap-repair-worker`: queue consumer for targeted replay repair
- `cmd/proto-contract-fixture`: Go fixture generator for Node↔Go proto contracts
- `cmd/ecoflow-panel-db-import`: solar panel DB importer/generator
- `cmd/ecoflow-panel-select-train`: panel detection model trainer/replay
- `cmd/ecoflow-ml-train`: ETA model trainer
- `cmd/ecoflow-pv-fingerprint`: PV feature extraction
- `pkg/ecoflow`: API client
- `pkg/ecoflowmqtt`: MQTT primitives
- `pkg/panelselect`: panel selection model + runtime predictor

## Universal Dashboard

- Expo universal app docs: [`apps/universal/README.md`](apps/universal/README.md)
