# EcoFlow Pulse

<img src="apps/universal/assets/icon.png" alt="EcoFlow Pulse app icon" style="width:50%; height:auto;">

EcoFlow Pulse is a realtime energy control room for EcoFlow devices. It turns
live solar input, battery state, load flow, device health, historical power
telemetry, and forecast-aware weather and solar outlooks into a clean
operator-grade experience across web, iPhone, iPad, and Android.

> [!NOTE]
> The product is built around the official EcoFlow API, MQTT telemetry streams, and a Kubernetes-first platform that supports live snapshots, durable archive, replay, and long-range rollup history.

## Product Summary

- Realtime telemetry dashboard for solar, SOC, charge/discharge, AC/DC, load,
  pack-level state, and device health.
- Universal app experience with auth-aware realtime, history, and comparison
  views across web, iOS, and Android.
- Profile-aware weather forecast, yesterday verification, and solar outlook
  widgets backed by weather and solar forecast services.
- Operator-focused UX with snapshot-first updates, trend charts, fleet summary,
  device detail views, profile preferences, and energy-impact explainers.
- Durable platform architecture for ingest, archive, replay, rollups, and
  resilient websocket delivery, plus forecast verification and calibration.

## Supported Devices

> [!IMPORTANT]
> **Actively validated:**
> - DELTA Pro Ultra (DPU)
> - DELTA 2 (D2)
> - DELTA 2 Max (D2M)

## Latest Delivered Work (Mar 2026)

- M1 closed: Keycloak OIDC + Expo PKCE auth, Node JWKS middleware, Go gRPC JWT
  auth/interceptors, and `viewer/admin` RBAC for device registry APIs.
- M2 closed: distributed ingest workers with Valkey lease/fencing, canonical
  `TelemetryEnvelope` publish to JetStream, Valkey projection snapshots, raw
  archive + manifest index, replay CLI, and targeted gap repair.
- M3 closed: Timescale minute/hour/day rollup pipeline with retention policies,
  gRPC range/compare query APIs, and public Node REST history endpoints.
- M4 closed: dedicated realtime WebSocket gateway (snapshot-on-connect + NATS
  deltas) with backpressure/downsampling ladder and Expo reconnect hardening.
- M5 in progress with shipped slices: Testcontainers pipeline integration,
  Node↔Go protobuf contract tests, and Playwright web E2E smoke tests.
- Profile page now ships saved-location weather forecasting with 7-day outlook,
  yesterday verification, Open-Meteo attribution, and a compact current-weather
  widget.
- New internal `weatherd` service wraps Open-Meteo with canonical grid-cell
  caching, snapshots, verification fallback, bias correction, and budget-aware
  upstream access.
- New internal `solar-forecastd` service combines energy truth and weather
  forecasts into deterministic solar outlooks, rolling verification, site
  calibration, and Grafana quality dashboards.
- Universal app branding refresh: generated high-resolution iOS, Android, web,
  and social-share assets, theme-family support, and a redesigned About /
  Appearance experience.
- Solar history UI now ships a `06:00-20:00` local comparison chart with
  10-minute buckets, prior-day overlay, and hover/tap bucket inspection.
- `Energy` dashboard now ships end-to-end with fleet or single-device scope,
  local-calendar windows, comparison mode, power and energy history charts,
  battery-flow summary, PV operating envelope diagnostics, and estimated value
  cards.
- `Energy Impact` now estimates today-so-far avoided `CO2e`, `NOx`, and `SO2`,
  plus conservative mature-tree equivalent and premium-EV driving-energy miles,
  with versioned methodology and an in-app explainer.

## App Features

- Realtime fleet and device dashboards with snapshot-first websocket delivery.
- Historical minute/hour/day views backed by Timescale rollups and comparison
  APIs.
- Dedicated `Energy` dashboard for multi-window solar, load, battery, PV
  envelope, and value analysis across one device or the whole fleet.
- Dedicated profile widgets for current weather, 7-day forecast, yesterday
  weather verification, and heuristic solar outlook based on live energy truth
  plus forecast irradiance.
- Solar-specific telemetry visibility including MPPT state, watts, volts, amps,
  and local-day generation comparison.
- Auth-aware universal client with Keycloak OIDC, Expo PKCE, persisted session
  hydration, and reconnect-safe realtime subscriptions.
- App-wide connection profiles (`Local` / `Cloud`) so the same client can
  switch API, websocket, and OIDC configuration together.
- `Energy Impact` insights using versioned EPA eGRID2023, lifecycle/tree, and
  EV-consumption factors.
- Theme-family support (`Original` / `New`) with system-controlled light/dark
  mode and modern app/share branding assets.

## Platform Features

### Local and deployment model

- Kubernetes-first local stack (`k3d`) with Helm-managed `pulse-platform` and
  `pulse-services` releases.
- Multi-replica local public edge and services for safer rollout validation and
  less disruption during restarts.
- One-command local bring-up and targeted redeploy paths including
  `make local-up`, `make local-deploy`, `make cloud-up`,
  `make cloud-refresh`, and `make services-image-push-cloud`.

### Data plane

- Distributed ingest runtime with one MQTT session globally per
  `(provider, provider_device_id)` via Valkey lease + fencing.
- EcoFlow certification-based reconnect flow per device plus startup quota
  bootstrap and periodic metadata/capability refresh publish path.
- Sharded `TelemetryEnvelope` publish to NATS
  (`pulse.telemetry.ingest.sNNN`).
- Projection worker consumes NATS and writes Valkey live snapshots/cursors.
- Rollup worker upserts minute/hour/day Timescale buckets from ingest streams.
- Archive worker writes protobuf+zstd objects to MinIO with manifest index in
  Postgres for replay lookup.
- Replay CLI plus gap detector/gap-repair workers for targeted repair instead of
  destructive full replay.

### Query and delivery layer

- Internal Go gRPC API for high-throughput snapshot, history, and control-plane
  operations, now including weather and solar forecast domains.
- Public Node REST BFF (`apps/pulse-platform`) for JWT validation and browser /
  app query access, including profile-scoped weather and solar outlook routes.
- Dedicated realtime WebSocket gateway (`apps/pulse-realtime-gateway`) with
  snapshot-on-connect, delta fanout, and staged backpressure degradation.

## Technology Stack

### Client and UX

- Expo + React Native + Expo Router
- Tamagui design system
- Expo Image for cacheable product and app imagery
- Vitest, Playwright, and Maestro for app validation

### Services and APIs

- Go for ingest, gRPC API, projection, archive, replay, gap repair, and rollups
- Node/TypeScript for the public REST BFF and realtime WebSocket gateway
- Buf / Protobuf contracts shared across Node and Go

### Platform and storage

- CloudNativePG Postgres with Timescale support
- NATS JetStream for telemetry streams and work queues
- Valkey + Sentinel for live snapshots and distributed lease ownership
- MinIO for local object archive and GCS for the hosted cloud archive
- Keycloak for auth across local/dev/cloud
- Helm + k3d for local cluster lifecycle

## Quality and Operability

- Testcontainers pipeline integration suite:
  `make test-pipeline-integration`
- Node↔Go proto compatibility tests:
  `make test-proto-contract`
- Playwright web E2E smoke:
  `make test-web-e2e`
- Maestro mobile E2E smoke:
  `MAESTRO_EXPO_URL='exp://127.0.0.1:8081' make test-mobile-e2e`
- Durable replay model, bounded queues, distributed lease ownership, and
  reconnect-aware websocket delivery designed for operational resilience

## Quick Start

```bash
go test ./...
make dev-up
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
- [Configuration Reference](docs/reference/configuration.md)
- [Telemetry Model](docs/reference/telemetry-model.md)
- [Solar Avoided Emissions Reference](docs/reference/solar-avoided-emissions.md)
- [Tree Equivalent Reference](docs/reference/tree-equivalent.md)
- [EV Database Report Reference](docs/reference/ev-us-europe-database-report.md)
- [Commands Reference](docs/reference/commands.md)

## Repository Layout

- `apps/pulse-platform`: public Node REST BFF over internal gRPC query APIs
- `apps/pulse-realtime-gateway`: public WS gateway (JWT auth, Valkey snapshot,
  NATS delta fanout, backpressure ladder)
- `apps/universal`: Expo universal dashboard (Web/iOS/Android)
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
