# EcoFlow Pulse

<img src="apps/universal/assets/icon.png" alt="EcoFlow Pulse app icon" style="width:50%; height:auto;">

EcoFlow Pulse is a realtime energy control room for portable power systems. It
combines live provider telemetry, authenticated device access, operational MQTT
logs, historical rollups, weather context, and solar forecasting into one
operator-grade app for web, iPhone, iPad, and Android.

> [!NOTE]
> Pulse is built around provider adapters for EcoFlow, Pecron, and Anker SOLIX,
> an Expo universal client, a Node REST BFF, a Node websocket gateway, Go
> gRPC/data services, NATS JetStream, Valkey, Postgres/Timescale, object archive
> storage, and Kubernetes-first deployment. Pecron and Anker SOLIX cloud support
> are read-only and unofficial/reverse-engineered.

## Architecture At A Glance

Pulse is organized as clear layers: universal client and identity, public REST
and websocket edges, internal Go gRPC services, streaming workers, durable
state/replay storage, and Kubernetes-first operations. See
[Architecture](docs/explanation/architecture.md) for the detailed explanation.

![Pulse architecture graph](docs/assets/architecture/pulse-architecture.svg)

## Current Product

- Realtime fleet and device dashboards for solar, battery state, load flow,
  AC/DC output, pack health, Storm Guard, and detailed PV input behavior.
- Auth-aware universal app with Keycloak OIDC, Expo Authorization Code + PKCE,
  same-tab browser sign-in, persisted sessions, and reconnect-safe websocket
  subscriptions.
- `Energy` dashboard for fleet or single-device solar, load, battery, PV
  envelope, comparison windows, value estimates, and on-demand long-range
  history.
- `Energy Calendar` heatmap for fleet or device solar generation by local day,
  with generated-value totals and drill-through into the Energy dashboard.
- Realtime `Logs` console for redacted MQTT/JetStream operational logs, with
  owner-scoped access, admin-wide access, provider/device/email/serial filters,
  single-select status/type filters, freetext search, bounded row retention, and
  expandable JSON detail.
- Weather and solar outlook widgets driven by saved profile location,
  Open-Meteo forecasts, yesterday verification, measured energy truth, and
  calibrated site/device forecast models.
- Energy-impact insights for measured solar generation using versioned avoided
  emissions, lifecycle/tree, and EV driving-energy factors.
- Local and cloud connection profiles so the same client can switch API,
  websocket, OIDC, and data-plane behavior together.

### App Overview

#### Fleet Dashboard

![Pulse fleet dashboard](docs/assets/app-overview/fleet-dashboard.png)

The fleet dashboard brings solar generation, device SOC, battery capacity, live
load, PV input, and day-over-day history into one operational view across active
systems.

#### Energy Balance

![Energy balance dashboard](docs/assets/app-overview/energy-balance.png)

The energy view shows solar generation, site load, battery net flow, grid value,
self-sufficiency, SOC band movement, and scoped device/window details for the
selected local-calendar period.

#### Energy Calendar

![Energy calendar dashboard](docs/assets/app-overview/energy-calendar.png)

The calendar view shows fleet or device solar generation by local day with
month navigation, heatmap tiles, generated-value totals, and direct
drill-through into the Energy dashboard for the selected date.

#### Solar Forecast

![Solar forecast dashboard](docs/assets/app-overview/solar-forecast.png)

The forecast screen combines current weather, consent state, site/device scope,
calibrated production estimates, and a seven-day outlook so expected solar
generation is visible alongside weather context.

## Recent Shipped Work

- Added the authenticated realtime Logs console, then hardened it for admin and
  device-owner access, provider scoping, tab lifecycle, replay/live stream
  behavior, dropdown overlays, bounded row retention, and suffix-family
  `Status` / `Info` type filters.
- Stabilized detail and Energy chart loading with lazy secondary panels,
  stale-while-refresh behavior, and on-demand long-range history.
- Improved MQTT parsing and gateway/cache hot paths with focused benchmarks and
  regression coverage.
- Repaired Pecron activation, TSL preservation, cloud MQTT ingest, and provider
  onboarding flows.
- Hardened Local Edge/cloud-data switching, cloud forward health checks, direct
  Helm/GKE deploy paths, and deploy-from-main service rollouts.
- Added data-plane-aware weather caching and a shared Valkey cache layer with
  partitioned keys, tag invalidation, and stampede protection.
- Normalized additional battery voltage/current telemetry and preserved
  consistent SOC across live views.

## Supported Devices

Actively validated:

- DELTA Pro Ultra (DPU)
- DELTA 2 (D2)
- DELTA 2 Max (D2M)
- Pecron E1000LFP (`p11vxg`, read-only cloud telemetry)
- Anker SOLIX power stations and home-battery systems with mapped Cloud MQTT
  telemetry (`anker_solix`, read-only unofficial integration)

## Platform Capabilities

- Distributed ingest workers maintain one MQTT session per provider device using
  Valkey lease/fencing and publish canonical `TelemetryEnvelope` events to NATS
  JetStream.
- Projection, rollup, archive, replay, gap detection, gap repair, forecast
  verification, and scheduler workers keep realtime state, long-range history,
  raw archive data, and forecast quality loops moving independently.
- Timescale-backed minute/hour/day rollups are the authoritative history path;
  archive-to-rollup rebuild tools repair historical windows without destructive
  replay.
- The public Node BFF validates JWTs and exposes browser/mobile REST routes over
  internal Go gRPC services for telemetry, history, profile, weather, solar
  forecasts, inference, logs metadata, and device authorization.
- The dedicated websocket gateway serves initial snapshots from Valkey, streams
  NATS deltas, validates JWTs, serves redacted realtime log replay/live fanout,
  and applies staged backpressure behavior.
- Local development runs on `k3d` with Helm-managed `pulse-platform` and
  `pulse-services`; cloud deployments target GKE with GCS archive storage,
  Keycloak, multi-replica public workloads, and cloud-specific overlays.

## Technology Stack

- **Client:** Expo, React Native, Expo Router, Tamagui, Vitest, Playwright,
  Maestro.
- **Public services:** Node/TypeScript REST BFF and realtime websocket gateway.
- **Data services:** Go gRPC API, ingest, projection, archive, replay, gap
  repair, rollups, scheduler, weather, solar forecast, and inference workers.
- **Storage and messaging:** Postgres/Timescale, NATS JetStream, Valkey +
  Sentinel, MinIO locally, GCS in cloud.
- **Platform:** Helm, k3d, GKE, CloudNativePG, Keycloak, Buf/Protobuf contracts.

## Quick Start

```bash
go test ./...
make dev-up
make platform-wait services-wait
```

Useful validation targets:

- `make test-pipeline-integration`
- `make test-proto-contract`
- `make test-web-e2e`
- `MAESTRO_EXPO_URL='exp://127.0.0.1:8081' make test-mobile-e2e`
- `make lint`

Useful deploy targets:

- `make local-up`
- `make local-deploy`
- `make cloud-up`
- `make cloud-refresh`
- `make services-image-push-cloud`
- `make public-images-push-cloud`

## Documentation

Developer docs follow Diataxis under `/docs`:

- [Developer Documentation Index](docs/README.md)
- [Architecture](docs/explanation/architecture.md)
- [Architecture Plan](docs/architecture/README.md)
- [Architecture Decision Records](docs/architecture/adr/README.md)
- [Configuration Reference](docs/reference/configuration.md)
- [Commands Reference](docs/reference/commands.md)
- [Telemetry Model](docs/reference/telemetry-model.md)
- [UI Visual System](docs/explanation/ui-visual-system.md)
- [Universal App README](apps/universal/README.md)

## Repository Layout

- `apps/universal`: Expo universal dashboard for web, iOS, and Android.
- `apps/pulse-platform`: public Node REST BFF over internal gRPC services.
- `apps/pulse-realtime-gateway`: public websocket gateway for live telemetry
  and redacted realtime logs.
- `cmd/ecoflow-grpc-api`: internal gRPC API runtime.
- `cmd/ecoflow-ingest-worker`, `cmd/ecoflow-projection-worker`,
  `cmd/ecoflow-rollup-worker`, `cmd/ecoflow-archive-worker`: core telemetry
  workers.
- `cmd/ecoflow-replay-cli`, `cmd/ecoflow-gap-detector`,
  `cmd/ecoflow-gap-repair-worker`, `cmd/ecoflow-rollup-rebuild`,
  `cmd/ecoflow-archive-audit`, `cmd/ecoflow-archive-reconcile`: replay,
  repair, rebuild, and archive integrity tooling.
- `cmd/ecoflow-scheduler`, `cmd/ecoflow-solar-verifier`: background forecast
  refresh, pruning, and verification workers.
- `cmd/pulse-mqtt-emulator`, `cmd/pulse-mqtt-history-backfill`: local/dev
  telemetry emulation and history tooling.
- `internal/weatherd`, `internal/solarforecastd`, `internal/energydashboard`,
  `internal/telemetryquery`, `internal/rollupworker`: main Go service domains.
- `pkg/ecoflow`, `pkg/ecoflowmqtt`, `pkg/panelselect`: reusable provider,
  MQTT, and panel-selection packages.
