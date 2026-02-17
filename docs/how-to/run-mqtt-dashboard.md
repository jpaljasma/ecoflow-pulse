# How-to: Run the MQTT Dashboard

This guide focuses on running and operating the live telemetry dashboard.

## Start

```bash
make mqtt
```

Equivalent direct command:

```bash
go run ./cmd/ecoflow-mqtt-sub
```

## Runtime Behavior

- subscribes to `/open/{certificateAccount}/{sn}/quota`,
- initializes from `GetDeviceAllQuota()` at startup,
- periodically reconciles with `GetDeviceAllQuota()` while running,
- marks MQTT as stale after short silence windows (`ECOFLOW_MQTT_STALE_AFTER`),
- runs a liveness REST poll after longer silence (`ECOFLOW_MQTT_LIVENESS_POLL_AFTER`),
- uses a bounded MQTT ingress queue (drop-oldest when full),
- renders dashboard output through an asynchronous UI writer queue (drop-oldest when full),
- persists minute history and training CSV through asynchronous bounded writer queues,
- runs panel detection on a dedicated worker goroutine with bounded input/output queues (drop-oldest when full),
  - continuously renders:
  - summary values (SOC, AC in, solar generated, out, net, state, updated),
  - detailed channel and battery tables,
  - a realtime `Solar Recommendations` table with per-port:
    - detected panel setup,
    - add-panels recommendation,
      - when one PV port is undetected/idle but the peer port has a detected setup,
        add-panels can mirror peer panel sizing to the local MPPT limit (instead
        of waiting forever for local detection),
    - upgrade-panels recommendations (`#1` and `#2`) selected from all safe
      panel DB candidates for the port,
    - cold-weather Voc safety guardrails for recommendation series sizing
      (conservative +20% Voc rise),
    - series/parallel recommendation layouts (`xSxP`) constrained by MPPT
      voltage/current limits,
    - shoulder-hours-aware ranking so clipped oversizing is evaluated with a
      solar-curve benefit term (not only flat clip-at-max),
    - complexity-aware ranking so simpler wiring/layout options are preferred
      when projected energy gain is close (fewer panels and less mixed S+P),
    - if upgrade `#1` is clipped, upgrade `#2` prefers a non-clipping
      alternative,
    - if one PV port has no viable primary recommendation, it can bootstrap
      recommendation `#1` from the other port's detected panel model and
      auto-size to local MPPT limits,
    - alternate recommendations prefer different panel models first; same-model
      alternates are only used when clipping behavior materially differs,
    - battery charge ETA impact (`#1` and `#2`) under sunny-condition capacity assumptions,
    - conservative bifacial ETA uplift (+15% ETA-effective PV watts) when the
      detected/recommended panel is bifacial,
    - all-ports combined ETA impact summary rows (spanning PV columns),
  - an optional `Solar Candidate Matrix` table with all safe candidates per PV
    input (enabled with `ECOFLOW_MQTT_SHOW_SOLAR_CANDIDATES=true`), including
    Voc/Vmp/Imp/Isc, safe series range, best S/P layout, complexity score, and
    potential.
  - minute history table,
  - per-port solar panel prediction labels and confidence (when panel model is loaded),
  - ML/heuristic ETA estimate rows.

## Exit

- press `q` for graceful shutdown,
- `Ctrl+C` also performs graceful close of resources.

## Log Files

- `logs/mqtt.log`: rewritten each run; includes raw payload lines and parsed summaries.
- `logs/mqtt_payload_raw-YYYY-MM-DD.log`: append-safe daily-rotated raw MQTT replay stream (`topic + payload_raw`) for offline replays/training imports.
- `logs/telemetry_history.jsonl`: append-only minute buckets used for warm
  startup and trends.

## Useful Environment Overrides

```bash
ECOFLOW_MQTT_SN=R351ZABAPH331057 \
ECOFLOW_MQTT_QUEUE_CAPACITY=64 \
ECOFLOW_MQTT_IDLE_RECONNECT_AFTER=120s \
ECOFLOW_MQTT_STALE_AFTER=30s \
ECOFLOW_MQTT_LIVENESS_POLL_AFTER=90s \
ECOFLOW_MQTT_UI_QUEUE_CAPACITY=8 \
ECOFLOW_MQTT_UI_REFRESH_INTERVAL=1s \
ECOFLOW_MQTT_SHOW_SOLAR_CANDIDATES=false \
ECOFLOW_MQTT_LOG_QUEUE_CAPACITY=2048 \
ECOFLOW_MQTT_RAW_PAYLOAD_LOG=true \
ECOFLOW_MQTT_RAW_PAYLOAD_LOG_PATH=logs/mqtt_payload_raw.log \
ECOFLOW_MQTT_RAW_PAYLOAD_LOG_QUEUE_CAPACITY=2048 \
ECOFLOW_MQTT_HISTORY_LOAD_WINDOW_MINUTES=180 \
ECOFLOW_MQTT_PANEL_MODEL_PATH=data/solar_panels/panel_select_model.json \
ECOFLOW_MQTT_PANEL_MODEL_QUEUE_CAPACITY=64 \
ECOFLOW_MQTT_PANEL_MODEL_RESULT_QUEUE_CAPACITY=64 \
make mqtt
```
