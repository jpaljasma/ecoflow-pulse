# Reference: Configuration

`.env` is auto-loaded by `ConfigFromEnvironment()` when present.

## Credentials and Environment

- `ECOFLOW_ACCESS_KEY`
- `ECOFLOW_SECRET_KEY`
- `ECOFLOW_BASE_URL` (default `https://api.ecoflow.com`)
- `ECOFLOW_ENV` (`dev|staging|prod`, default `dev`)
- `ECOFLOW_DEBUG` (default `true` in `dev`, `false` in `staging/prod`)
- `ECOFLOW_DOTENV_PATH` (default `.env`)

Environment-specific credential keys (preferred):

- `ECOFLOW_DEV_ACCESS_KEY`, `ECOFLOW_DEV_SECRET_KEY`
- `ECOFLOW_STAGING_ACCESS_KEY`, `ECOFLOW_STAGING_SECRET_KEY`
- `ECOFLOW_PROD_ACCESS_KEY`, `ECOFLOW_PROD_SECRET_KEY`

## MQTT Runtime

- `ECOFLOW_MQTT_SN`
- `ECOFLOW_MQTT_DEVICE_MATCH` (default `delta pro ultra`)
- `ECOFLOW_MQTT_TOPIC`
- `ECOFLOW_MQTT_KEEPALIVE` (default `60s`)
- `ECOFLOW_MQTT_CONNECT_TIMEOUT` (default `10s`)
- `ECOFLOW_MQTT_READ_TIMEOUT` (default `30s`)
- `ECOFLOW_MQTT_WRITE_TIMEOUT` (default `15s`)
- `ECOFLOW_MQTT_IDLE_RECONNECT_AFTER` (default `0s`, disabled)
- `ECOFLOW_MQTT_QUEUE_CAPACITY` (default `64`)
- `ECOFLOW_MQTT_PRINT_PAYLOAD` (default `false`)

## Telemetry History and Fallback

- `ECOFLOW_MQTT_HISTORY_PATH` (default `logs/telemetry_history.jsonl`)
- `ECOFLOW_MQTT_HISTORY_LOAD_WINDOW_MINUTES` (default `180`)
- `ECOFLOW_MQTT_AUTH_REJECT_THRESHOLD` (default `3`)
- `ECOFLOW_MQTT_FALLBACK_POLL_INTERVAL` (default `15s`)
- `ECOFLOW_MQTT_FALLBACK_POLL_TIMEOUT` (default `12s`)

## Server Runtime

- `ECOFLOW_SERVER_ADDR` (default `:8080`)
- `ECOFLOW_SERVER_GOMAXPROCS`
- `ECOFLOW_SERVER_COMPRESSION_MIN_BYTES` (default `1024`)
- `ECOFLOW_SERVER_GZIP_LEVEL` (default `5`)
- `ECOFLOW_SERVER_DEFLATE_LEVEL` (default `5`)
- `ECOFLOW_SERVER_BROTLI_LEVEL` (default `5`, `moderncompress` builds)
- `ECOFLOW_SERVER_ZSTD_LEVEL` (default `3`, `moderncompress` builds)
