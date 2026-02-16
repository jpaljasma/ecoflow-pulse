# Explanation: Architecture

Ecoflow-Pulse separates concerns between API access, MQTT ingestion, state
derivation, and UI projection.

## Layers

1. Ingestion

- HTTP API via `pkg/ecoflow`
- MQTT transport via `pkg/ecoflowmqtt`

1. Runtime orchestration

- connection lifecycle, retries, queueing, read loop, graceful shutdown
- implemented in `cmd/ecoflow-mqtt-sub/mqtt_runtime.go`

1. Mapping and state derivation

- raw payload parsing and typed mapping
- domain logic split by telemetry area:
  - `battery_logic.go`
  - `pv_logic.go`
  - `mppt_logic.go`

1. Projection and rendering

- derived dashboard values, status flags, and formatting
- view projection in `viewmodel.go`
- formatting in `formatters.go`

1. Persistence and estimates

- minute-bucket telemetry persistence
- heuristic and ML-based ETA estimation
- implemented in `estimates.go`

## Why this shape

- minimizes coupling between transport and business mapping,
- allows per-domain telemetry improvements without changing runtime plumbing,
- keeps test coverage granular by domain and by behavior class.
