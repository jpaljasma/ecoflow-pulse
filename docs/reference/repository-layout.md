# Reference: Repository Layout

Top-level structure:

- `cmd/`
  - `ecoflow-mqtt-sub`: live MQTT dashboard and telemetry processing runtime.
  - `ecoflow-pv-fingerprint`: PV feature extraction from training telemetry CSV.
  - `ecoflow-panel-select-train`: panel selection model training + replay.
  - `ecoflow-smoke`: smoke checks against EcoFlow API.
  - `ecoflow-server`: API server entrypoint.
- `pkg/`
  - `ecoflow`: core API client and signing.
  - `ecoflowmqtt`: MQTT subscriber primitives.
  - `panelselect`: panel selection model, feature tracker, and predictor.
  - `ecoflowserver`: server helpers and middleware.
  - `logger`: structured logging package.
- `logs/`
  - `mqtt.log`: run log and raw payload stream.
  - `telemetry_history.jsonl`: minute telemetry persistence.
  - `pv_fingerprint.csv`: generated per-port PV fingerprint features.
- `data/solar_panels/`
  - `solar_panel_specs_v13.index.json`: compact panel capabilities index
    with derived fields such as `module_efficiency_pct` and
    `module_efficiency_source` (`reported`, `notes`, `estimated_*`), and
    `purchase_link`.
  - `panel_purchase_links_v13.json`: curated panel-id to purchase-link override map
    used during panel DB regeneration.
  - `panel_select_model.json`: trained panel selection model artifact.
- `deploy/`
  - `charts/pulse-platform`: platform umbrella chart scaffold.
  - `charts/pulse-services`: services umbrella chart scaffold.
  - `env/local` and `env/dev`: values files for local/dev deploys.
  - `argocd/apps`: direct Argo CD apps (`pulse-platform`, `pulse-services`).
  - `tilt/k3d-config.yaml`: k3d local cluster config.
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
