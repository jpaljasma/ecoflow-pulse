# How-To: Train Solar Panel Selection Model

Train a per-port panel selection model from telemetry CSV and use replay
metrics to validate fit.

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

## Output Usage

The generated `data/solar_panels/panel_select_model.json` artifact is retained
for offline analysis and future runtime integrations. Keep it reproducible and
versioned in the same branch when retraining materially changes model behavior.
