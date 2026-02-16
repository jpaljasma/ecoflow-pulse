# Reference: Repository Layout

Top-level structure:

- `cmd/`
  - `ecoflow-mqtt-sub`: live MQTT dashboard and telemetry processing runtime.
  - `ecoflow-smoke`: smoke checks against EcoFlow API.
  - `ecoflow-server`: API server entrypoint.
- `pkg/`
  - `ecoflow`: core API client and signing.
  - `ecoflowmqtt`: MQTT subscriber primitives.
  - `ecoflowserver`: server helpers and middleware.
  - `logger`: structured logging package.
- `logs/`
  - `mqtt.log`: run log and raw payload stream.
  - `telemetry_history.jsonl`: minute telemetry persistence.
- `docs/`: developer documentation in Diataxis layout.

Key dashboard-focused files:

- `cmd/ecoflow-mqtt-sub/main.go`: entrypoint and orchestration.
- `cmd/ecoflow-mqtt-sub/mqtt_runtime.go`: MQTT connect/reconnect/read runtime.
- `cmd/ecoflow-mqtt-sub/file_locking.go`: safe append sinks and per-device lock files.
- `cmd/ecoflow-mqtt-sub/ui_async.go`: asynchronous UI output writer with bounded queue.
- `cmd/ecoflow-mqtt-sub/viewmodel.go`: dashboard projection logic.
- `cmd/ecoflow-mqtt-sub/renderer.go`: ASCII table rendering.
- `cmd/ecoflow-mqtt-sub/estimates.go`: ETA and ML estimation logic.
- `cmd/ecoflow-mqtt-sub/formatters.go`: display and unit formatting helpers.
- `cmd/ecoflow-mqtt-sub/*_logic.go`: domain-specific mapping helpers
  (battery, PV, MPPT).
