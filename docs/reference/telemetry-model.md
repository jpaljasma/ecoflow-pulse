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
- solar panel recommendations (per PV port):
  - detected setup from runtime panel model,
  - add-panels upsell recommendation (headroom to device PV limit),
  - if a port is undetected/idle while the peer PV port has a detected setup,
    add-panels can mirror peer per-panel sizing to local MPPT limits,
  - upgrade recommendation selected from all safe panel DB candidates (not only best/alt metadata),
  - cold-weather Voc safety is enforced when selecting panel candidates/series
    counts (conservative +20% Voc rise factor) to avoid unsafe MPPT voltage spikes,
  - upgrade suggestions include series/parallel layouts (`xSxP`) and enforce MPPT
    voltage/current limits,
  - recommendation ranking applies a shoulder-hours uplift term to account for
    oversizing benefit under real solar curves (not only flat max-watt clipping),
  - recommendation ranking applies a complexity score so lower-complexity
    topologies (fewer panels, fewer parallel branches, and less mixed S+P wiring)
    are preferred when energy gain is marginal,
  - EcoFlow `125W Bifacial Modular` layouts get a reduced complexity weight in
    ranking (1-4P setups are treated as easier to deploy),
  - if upgrade `#1` is clipping, upgrade `#2` is forced to a non-clipping
    alternative when one exists,
  - if a port has no viable primary recommendation but another port has a
    detected panel setup, recommendation `#1` falls back to that detected panel
    model and auto-sizes layout to the current MPPT limits,
  - alternates prefer panel-model diversity; same-model alternates are only used
    when clipping behavior materially changes,
  - detailed candidate matrix renders every safe panel candidate per PV input,
    including electrical data (Voc/Vmp/Imp/Isc, safe series range, best layout,
    complexity score, and expected clipped/non-clipped potential),
  - projected battery charge ETA impact (primary and second-best upgrade paths),
    using the same shoulder-hours uplift model as recommendation ranking (instead
    of flat clipped max-watt assumptions),
  - conservative bifacial ETA adjustment (+15% ETA-effective PV watts when the
    detected/recommended panel is bifacial),
  - all-ports combined ETA impact summary rows when multiple PV ports are present.

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
- Runtime model updates are gated by per-port PV voltage presence. When PV voltage
  is absent, the dashboard keeps the last known panel detection/recommendation
  instead of re-running prediction on no-input samples.

The dashboard recommendation table combines runtime panel predictions with
device PV capability limits and panel DB compatibility metadata loaded at startup.

## Raw MQTT Replay Log

For replay/training pipelines, MQTT payload ingress can be mirrored to a dedicated
daily-rotated append-safe log:

- base path: `ECOFLOW_MQTT_RAW_PAYLOAD_LOG_PATH` (default `logs/mqtt_payload_raw.log`)
- runtime files: `logs/mqtt_payload_raw-YYYY-MM-DD.log`
- payload format: timestamped `topic=... payload_raw=...` lines
- safety: same chunked file-lock append strategy as other runtime logs, safe for
  multi-thread and multi-process append.

## Units

- Instantaneous dashboard channel values are watts (`W` or `kW`).
- Minute table values are energy per minute (`Wh` or `kWh`).

## Supported Device Behaviors

Validated mappings and dashboard behavior are currently maintained for:

- DELTA 2 Max (D2M)
- DELTA Pro Ultra (DPU)
