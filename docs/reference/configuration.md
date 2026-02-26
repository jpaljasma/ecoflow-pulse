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
- MQTT `client_id` is deterministic and static per serial number:
  - format: `ecoflow-pulse-<crc32(sn)>`
  - this avoids exhausting EcoFlow broker limits on unique client IDs during reconnect/test cycles
- `ECOFLOW_MQTT_IDLE_RECONNECT_AFTER` (default `120s`)
- `ECOFLOW_MQTT_STALE_AFTER` (default `30s`)
- `ECOFLOW_MQTT_LIVENESS_POLL_AFTER` (default `90s`)
- `ECOFLOW_MQTT_LIVENESS_CHECK_INTERVAL` (default `10s`)
- `ECOFLOW_MQTT_LIVENESS_POLL_TIMEOUT` (default `12s`)
- `ECOFLOW_MQTT_LIVENESS_POLL_MIN_INTERVAL` (default `60s`)
- `ECOFLOW_MQTT_QUEUE_CAPACITY` (default `64`)
- `ECOFLOW_MQTT_PRINT_PAYLOAD` (default `false`)
- `ECOFLOW_MQTT_TABLE_VIEW` (default `true` when payload print mode is disabled)
- `ECOFLOW_MQTT_UI_REFRESH_INTERVAL` (default `1s`)
- `ECOFLOW_MQTT_UI_QUEUE_CAPACITY` (default `8`)
- `ECOFLOW_MQTT_MINUTE_ROWS` (default `3`)
- `ECOFLOW_MQTT_SHOW_SOLAR_CANDIDATES` (default `false`; set `true` to render full per-port solar candidate matrix)

## Telemetry History and Fallback

- `ECOFLOW_MQTT_HISTORY_PATH` (default `logs/telemetry_history.jsonl`)
- `ECOFLOW_MQTT_HISTORY_LOAD_WINDOW_MINUTES` (default `180`)
- `ECOFLOW_MQTT_HISTORY_QUEUE_CAPACITY` (default `1024`)
- `ECOFLOW_MQTT_AUTH_REJECT_THRESHOLD` (default `3`)
- `ECOFLOW_MQTT_FALLBACK_POLL_INTERVAL` (default `15s`)
- `ECOFLOW_MQTT_FALLBACK_POLL_TIMEOUT` (default `12s`)
- `ECOFLOW_MQTT_RECONCILE_INTERVAL` (default `1m`)
- `ECOFLOW_MQTT_RECONCILE_TIMEOUT` (default `12s`)
- `ECOFLOW_MQTT_PANEL_DB_ENABLED` (default `true`)
- `ECOFLOW_MQTT_PANEL_DB_PATH` (default `data/solar_panels/solar_panel_specs_v13.index.json`)
- `ECOFLOW_MQTT_PANEL_MODEL_ENABLED` (default `true`)
- `ECOFLOW_MQTT_PANEL_MODEL_PATH` (default `data/solar_panels/panel_select_model.json`)
- `ECOFLOW_MQTT_PANEL_MODEL_WINDOW` (default `240`)
- `ECOFLOW_MQTT_PANEL_MODEL_MIN_SAMPLES` (default `20`)
- `ECOFLOW_MQTT_PANEL_MODEL_QUEUE_CAPACITY` (default `64`)
- `ECOFLOW_MQTT_PANEL_MODEL_RESULT_QUEUE_CAPACITY` (default `64`)

## Logging and Process Safety

Common service/worker logging knobs (`cmd/ecoflow-ingest-worker`, `cmd/ecoflow-projection-worker`, `cmd/ecoflow-archive-worker`, `cmd/ecoflow-gap-detector`, `cmd/ecoflow-gap-repair-worker`, `cmd/ecoflow-grpc-api`, `cmd/ecoflow-replay-cli`, `cmd/ecoflow-dev-seed`):

- `LOG_LEVEL` (default `info`)
- `LOG_ASYNC_DISABLED` (default `false`; when `true`, bypass async queue)
- `LOG_ASYNC_QUEUE_SIZE` (default `8192`; bounded queue capacity)
- `LOG_ASYNC_BYPASS_LEVEL` (default `warn`; warn/error logs bypass queue sync)
- `LOG_METRICS_INTERVAL` (default `30s`; set `0` to disable async queue metrics logs)

Ingest payload debug knobs (`cmd/ecoflow-ingest-worker`):

- `INGEST_MQTT_LOG_PAYLOAD_DEBUG` (default `false`; payload logging stays off hot path)
- `INGEST_MQTT_LOG_PAYLOAD_SAMPLE_EVERY` (default `100`; sampled debug payload logging interval)

- `ECOFLOW_MQTT_LOG_PATH` (default `logs/mqtt.log`, file truncated on startup)
- `ECOFLOW_MQTT_LOG_QUEUE_CAPACITY` (default `2048`)
- `ECOFLOW_MQTT_RAW_PAYLOAD_LOG` (default `true`)
- `ECOFLOW_MQTT_RAW_PAYLOAD_LOG_PATH` (default `logs/mqtt_payload_raw.log`, stored as daily files: `...-YYYY-MM-DD.log`)
- `ECOFLOW_MQTT_RAW_PAYLOAD_LOG_QUEUE_CAPACITY` (default `2048`)
- `ECOFLOW_MQTT_LOCK_DIR` (default `logs/locks`, per-device serial instance lock files)
- `ECOFLOW_MQTT_TRAINING_CSV` (default `true`)
- `ECOFLOW_MQTT_TRAINING_CSV_PATH` (default `logs/telemetry_training.csv`)
- `ECOFLOW_MQTT_TRAINING_CSV_INTERVAL` (default `10s`)
- `ECOFLOW_MQTT_TRAINING_CSV_JITTER` (default `0.2`)
- `ECOFLOW_MQTT_TRAINING_CSV_QUEUE_CAPACITY` (default `4096`)

## Server Runtime

- `ECOFLOW_SERVER_ADDR` (default `:8080`)
- `ECOFLOW_SERVER_GOMAXPROCS`
- `ECOFLOW_SERVER_COMPRESSION_MIN_BYTES` (default `1024`)
- `ECOFLOW_SERVER_GZIP_LEVEL` (default `5`)
- `ECOFLOW_SERVER_DEFLATE_LEVEL` (default `5`)
- `ECOFLOW_SERVER_BROTLI_LEVEL` (default `5`, `moderncompress` builds)
- `ECOFLOW_SERVER_ZSTD_LEVEL` (default `3`, `moderncompress` builds)

## Internal gRPC API Runtime

- `PULSE_ENV` (`local|dev|staging|prod`, default `local`)
- `GRPC_LISTEN_ADDR` (default from grpc server profile; typically `:9090` in local/dev)
- `CONTROL_PLANE_DB_DSN`
  - when set to a non-empty DSN, `cmd/ecoflow-grpc-api` uses Postgres-backed control-plane storage,
  - when unset (or whitespace), service falls back to in-memory control-plane storage for local bootstrap/testing.

## Explicit Dev Seed (`cmd/ecoflow-dev-seed`)

- `ECOFLOW_DEV_ACCESS_KEY` (required)
- `ECOFLOW_DEV_SECRET_KEY` (required)
- `CONTROL_PLANE_DB_DSN` (required)
- `ECOFLOW_DEV_USER_SUBJECT` (default `jpaljasma@gmail.com`)
- `ECOFLOW_DEV_USER_EMAIL` (default = `ECOFLOW_DEV_USER_SUBJECT`)
- `ECOFLOW_DEV_PROVIDER` (default `ecoflow`)
- `ECOFLOW_DEV_SEED_SNS` (comma/whitespace-delimited serials, default `R351ZABAPH331057,Y711ZABA9H2P0294`)

## Universal App (Expo)

- `EXPO_PUBLIC_API_URL`
- `EXPO_PUBLIC_WS_URL`
- `EXPO_PUBLIC_MOCK_LOG_URL`
- `EXPO_PUBLIC_MOCK_TRAINING_URL`
- `EXPO_PUBLIC_ASSET_BASE_URL`
  - optional absolute base URL for product/brand images in native and web,
  - when unset on web, app uses `/public` local paths,
  - large product images can be remote URI-based and are cached via `expo-image` (`memory-disk`),
  - brand/logo assets are bundled local app assets for smooth top-bar and menu rendering.
