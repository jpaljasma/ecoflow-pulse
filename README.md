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

## Quick Start

```bash
go test ./...
make mqtt
```

## Documentation

Developer docs follow Diataxis under `/docs`:

- [Developer Documentation Index](docs/README.md)
- [Run MQTT Dashboard](docs/how-to/run-mqtt-dashboard.md)
- [Configuration Reference](docs/reference/configuration.md)
- [Telemetry Model](docs/reference/telemetry-model.md)
- [Commands Reference](docs/reference/commands.md)

## Repository Layout

- `cmd/ecoflow-mqtt-sub`: real-time MQTT telemetry dashboard
- `cmd/ecoflow-panel-db-import`: solar panel DB importer/generator
- `cmd/ecoflow-panel-select-train`: panel detection model trainer/replay
- `cmd/ecoflow-ml-train`: ETA model trainer
- `cmd/ecoflow-pv-fingerprint`: PV feature extraction
- `pkg/ecoflow`: API client
- `pkg/ecoflowmqtt`: MQTT primitives
- `pkg/panelselect`: panel selection model + runtime predictor

## Universal Dashboard Scaffold
- Expo universal app scaffold: [`apps/universal/README.md`](apps/universal/README.md)
