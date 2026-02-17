# How-To: Train Solar Panel Selection Model

Train a per-port panel selection model from `logs/telemetry_training.csv` and
use replay metrics to validate fit.

## Input

- telemetry CSV: `logs/telemetry_training.csv`
- optional panel map JSON (for label overrides)

## Basic Usage

```bash
go run ./cmd/ecoflow-panel-select-train
```

Default output:

- `data/solar_panels/panel_select_model.json`

## Train + Replay Explicitly

```bash
go run ./cmd/ecoflow-panel-select-train \
  -csv logs/telemetry_training.csv \
  -out data/solar_panels/panel_select_model.json \
  -replay \
  -replay-every 10 \
  -replay-min-samples 20
```

## Helper Script

```bash
./scripts/train_panel_select_model.sh
```

Optional overrides:

```bash
./scripts/train_panel_select_model.sh \
  logs/telemetry_training.csv \
  data/solar_panels/panel_select_model.json \
  panel_map.json
```

## Runtime Usage in MQTT Dashboard

Model loading is enabled by default in `cmd/ecoflow-mqtt-sub`.

Environment controls:

- `ECOFLOW_MQTT_PANEL_MODEL_ENABLED` (default `true`)
- `ECOFLOW_MQTT_PANEL_MODEL_PATH` (default `data/solar_panels/panel_select_model.json`)
- `ECOFLOW_MQTT_PANEL_MODEL_WINDOW` (default `240`)
- `ECOFLOW_MQTT_PANEL_MODEL_MIN_SAMPLES` (default `20`)

When loaded, the dashboard annotates both PV rows (`low` and `high`) with
real-time panel prediction + confidence.
