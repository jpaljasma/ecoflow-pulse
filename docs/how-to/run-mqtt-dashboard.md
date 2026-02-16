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
ECOFLOW_MQTT_HISTORY_LOAD_WINDOW_MINUTES=180 \
make mqtt
```
