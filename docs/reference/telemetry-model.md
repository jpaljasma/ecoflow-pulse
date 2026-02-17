# Reference: Telemetry Model

## MQTT Topic

Primary device-to-app telemetry topic:

- `/open/{certificateAccount}/{sn}/quota`

## Snapshot Domains

The dashboard snapshot aggregates telemetry into these domains:

- device summary (SOC, in/out/net, state, updated),
- channels (AC in, PV low/high/total, AC out, DC out, XT150 in/out),
- pack-level battery data (up to 5 packs for DPU),
- status flags (AC/DC/USB/EV/passthrough/grounded/fan/preconditioning),
- ETA and ML estimate outputs.

## Minute History Buckets

Minute buckets aggregate many samples into one row.

Stored metrics:

- SOC average (percent),
- solar generated (Wh/min),
- AC input (Wh/min),
- AC output (Wh/min),
- DC output (Wh/min),
- battery charge energy (Wh/min),
- total input and total output (Wh/min),
- net (Wh/min).

Persistence file:

- `logs/telemetry_history.jsonl`

## Training Telemetry Capture

ML training data is persisted as CSV for offline model tuning.

- file: `logs/telemetry_training.csv`
- capture cadence: every power-related MQTT update, plus periodic sampled rows
  using `ECOFLOW_MQTT_TRAINING_CSV_INTERVAL` + jitter.
- writes are append-safe across processes and threads.

PV fingerprint features can be generated from training telemetry:

- command: `go run ./cmd/ecoflow-pv-fingerprint`
- output file: `logs/pv_fingerprint.csv`
- scope: per `device_sn + product_name + port(low/high)`
- includes median and max-based power/voltage/current features for panel modeling.

Panel selection model can be trained from telemetry:

- command: `go run ./cmd/ecoflow-panel-select-train`
- output file: `data/solar_panels/panel_select_model.json`
- replay mode reports per-port prediction accuracy and confidence.

When loaded by `cmd/ecoflow-mqtt-sub`, PV low/high rows include:

- predicted panel setup label,
- confidence score,
- sample count used by the runtime tracker.

## Units

- Instantaneous dashboard channel values are watts (`W` or `kW`).
- Minute table values are energy per minute (`Wh` or `kWh`).

## Supported Device Behaviors

Validated mappings and dashboard behavior are currently maintained for:

- DELTA 2 Max (D2M)
- DELTA Pro Ultra (DPU)
