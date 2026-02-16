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
- continuously renders:
  - summary values (SOC, AC in, solar generated, out, net, state, updated),
  - detailed channel and battery tables,
  - minute history table,
  - ML/heuristic ETA estimate rows.

## Exit

- press `q` for graceful shutdown,
- `Ctrl+C` also performs graceful close of resources.

## Log Files

- `logs/mqtt.log`: rewritten each run; includes raw payload lines and parsed summaries.
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
ECOFLOW_MQTT_LOG_QUEUE_CAPACITY=2048 \
ECOFLOW_MQTT_HISTORY_LOAD_WINDOW_MINUTES=180 \
make mqtt
```
