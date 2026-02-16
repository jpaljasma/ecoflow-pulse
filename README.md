# Ecoflow-Pulse

Ecoflow-Pulse is the official package name.
A real-time pulse monitor for EcoFlow devices - telemetry, history, and
integrations built on the official API.

## What This Project Provides

- A production-oriented Go client for EcoFlow HTTP APIs (signing, retries,
  transport tuning, observability hooks).
- A real-time MQTT telemetry subscriber and terminal dashboard for supported devices.
- Persistent minute telemetry history for trend analysis and ML-assisted ETA estimation.

Supported and actively validated devices:

- DELTA 2 Max (D2M)
- DELTA Pro Ultra (DPU)

## Quick Start

```bash
go test ./...
make mqtt
```

## Documentation

Developer documentation is organized under `docs/` using the Diataxis framework:

- Tutorials: `docs/tutorials/`
- How-to guides: `docs/how-to/`
- Reference: `docs/reference/`
- Explanation: `docs/explanation/`

Start here:

- `docs/README.md`

## Repository Layout

- `pkg/ecoflow`: reusable EcoFlow API client
- `pkg/ecoflowserver`: HTTP server utilities and middleware
- `pkg/ecoflowmqtt`: MQTT subscriber primitives
- `cmd/ecoflow-server`: API server entrypoint
- `cmd/ecoflow-smoke`: manual API smoke checks
- `cmd/ecoflow-mqtt-sub`: real-time MQTT telemetry dashboard
