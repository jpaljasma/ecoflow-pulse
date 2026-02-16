# Explanation: Architecture

Ecoflow-Pulse separates concerns between API access, MQTT ingestion, state
derivation, and UI projection.

## Layers

- Ingestion
  HTTP API via `pkg/ecoflow` and MQTT transport via `pkg/ecoflowmqtt`.

- Runtime orchestration
  Connection lifecycle, retry/backoff, bounded ingress queue, read loop, and graceful shutdown.
  Implemented in `cmd/ecoflow-mqtt-sub/mqtt_runtime.go` and `cmd/ecoflow-mqtt-sub/main.go`.

- Mapping and state derivation
  Raw payload parsing + typed mapping, split by telemetry domains:
  `battery_logic.go`, `pv_logic.go`, `mppt_logic.go`.

- Projection and rendering
  View-model projection in `viewmodel.go` and table rendering in `renderer.go`.
  Dashboard writes are asynchronous through `ui_async.go` (bounded queue, drop-oldest)
  so UI output does not block telemetry processing.

- Persistence and estimates
  File-safe append sinks in `file_locking.go` are used for concurrent-safe process/thread writes.
  History/training file writes use asynchronous bounded queues so telemetry processing does
  not block on disk I/O.
  Minute-bucket history + training telemetry capture + ETA models are implemented across
  `training_csv.go`, `estimates.go`, and `estimates_profiled.go`.

## Why this shape

- minimizes coupling between transport and business mapping,
- allows per-domain telemetry improvements without changing runtime plumbing,
- keeps UI output decoupled from ingestion hot paths,
- keeps test coverage granular by domain and behavior class.
