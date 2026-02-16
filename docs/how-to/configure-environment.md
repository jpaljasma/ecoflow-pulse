# How-to: Configure Environment and Credentials

Use this guide to configure Ecoflow-Pulse for local development or operations.

## 1. Create `.env`

```bash
cat > .env <<'EOF'
ECOFLOW_ACCESS_KEY=your_access_key
ECOFLOW_SECRET_KEY=your_secret_key
ECOFLOW_ENV=prod
ECOFLOW_BASE_URL=https://api.ecoflow.com
EOF
```

## 2. Set Target Device for MQTT

Add one of:

- explicit serial:

```bash
ECOFLOW_MQTT_SN=Y711ZABA9H2P0294
```

- name matching fallback:

```bash
ECOFLOW_MQTT_DEVICE_MATCH=delta pro ultra
```

If both are set, `ECOFLOW_MQTT_SN` is authoritative.

## 3. Optional Runtime Tuning

Common MQTT tuning keys:

- `ECOFLOW_MQTT_KEEPALIVE`
- `ECOFLOW_MQTT_CONNECT_TIMEOUT`
- `ECOFLOW_MQTT_READ_TIMEOUT`
- `ECOFLOW_MQTT_WRITE_TIMEOUT`
- `ECOFLOW_MQTT_QUEUE_CAPACITY`
- `ECOFLOW_MQTT_IDLE_RECONNECT_AFTER`

History and fallback:

- `ECOFLOW_MQTT_HISTORY_PATH`
- `ECOFLOW_MQTT_HISTORY_LOAD_WINDOW_MINUTES`
- `ECOFLOW_MQTT_AUTH_REJECT_THRESHOLD`
- `ECOFLOW_MQTT_FALLBACK_POLL_INTERVAL`
- `ECOFLOW_MQTT_FALLBACK_POLL_TIMEOUT`

## 4. Verify

```bash
go run ./cmd/ecoflow-smoke
```

If smoke passes, configuration is valid enough for dashboard startup.
