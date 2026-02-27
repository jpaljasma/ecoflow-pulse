# Ecoflow-Pulse

Ecoflow-Pulse is a real-time pulse monitor for EcoFlow devices, built on the
official API and MQTT telemetry streams.

## Supported Devices

Actively validated:

- DELTA 2 Max (D2M)
- DELTA Pro Ultra (DPU)

## Core Capabilities

- Live terminal dashboard for power, SOC, states, and per-pack battery telemetry.
- Persistent telemetry history (minute buckets + training CSV) for analysis.
- ETA estimation with MPPT, profile-specific ML, and generic ML model fallback.
- Solar telemetry and MPPT visibility (low/high inputs, volts/amps/watts, state).
- Solar panel detection and upgrade recommendations using panel database + model.
- Safe runtime behavior for reconnects, bounded queues, and multi-instance lock handling.

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
  - sharded `TelemetryEnvelope` publish to NATS (`pulse.telemetry.ingest.sNNN`)
- Live read model:
  - projection worker consumes NATS and writes Valkey snapshots/cursors
  - gRPC snapshot and realtime subscription flow from the same read model
- Durable archive and replay:
  - archive worker writes protobuf+zstd objects to MinIO
  - manifest index persisted in Postgres for replay lookup
  - replay CLI supports `list-devices`, `device`, and `fleet` modes
  - gap detector + gap-repair worker perform targeted replay, not full replays

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
- [Commands Reference](docs/reference/commands.md)

## Repository Layout

- `cmd/ecoflow-mqtt-sub`: real-time MQTT telemetry dashboard
- `cmd/ecoflow-grpc-api`: internal high-throughput gRPC API
- `cmd/ecoflow-ingest-worker`: distributed MQTT ingest worker
- `cmd/ecoflow-projection-worker`: NATS -> Valkey live snapshot projector
- `cmd/ecoflow-archive-worker`: raw archive writer (protobuf+zstd to MinIO)
- `cmd/ecoflow-replay-cli`: replay CLI (`list-devices`, `device`, `fleet`)
- `cmd/ecoflow-gap-detector`: targeted gap detection and replay enqueue
- `cmd/ecoflow-gap-repair-worker`: queue consumer for targeted replay repair
- `cmd/ecoflow-panel-db-import`: solar panel DB importer/generator
- `cmd/ecoflow-panel-select-train`: panel detection model trainer/replay
- `cmd/ecoflow-ml-train`: ETA model trainer
- `cmd/ecoflow-pv-fingerprint`: PV feature extraction
- `pkg/ecoflow`: API client
- `pkg/ecoflowmqtt`: MQTT primitives
- `pkg/panelselect`: panel selection model + runtime predictor

## Universal Dashboard Scaffold
- Expo universal app scaffold: [`apps/universal/README.md`](apps/universal/README.md)
