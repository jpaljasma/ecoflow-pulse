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
- `INGEST_QUOTA_FETCH_TIMEOUT` (default `10s`; per-call timeout for `GetDeviceAllQuota()` bootstrap/refresh pulls)
- `INGEST_QUOTA_REFRESH_INTERVAL` (default `30s`; periodic quota refresh cadence while an MQTT session is alive)
- `INGEST_QUOTA_REFRESH_JITTER` (default `0.20`; proportional jitter applied to periodic quota refresh scheduling)
- `INGEST_QUOTA_METRICS_INTERVAL` (default `30s`; jittered aggregate quota refresh metrics log interval, set `0` to disable)

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
- `GRPC_AUTH_MODE` (`noop|keycloak`, default `noop`)
  - `noop`: development-only pass-through auth mode.
  - `keycloak`: validates bearer JWTs via Keycloak OIDC/JWKS and injects claims into gRPC context.
- `KEYCLOAK_ISSUER_URL` (required when `GRPC_AUTH_MODE=keycloak`)
- `KEYCLOAK_AUDIENCE` (optional; when set, JWT audience must match)
- `GRPC_AUTH_ALLOW_MISSING_JWT` (default `false`; optional only for controlled local bootstrap)
- `CONTROL_PLANE_DB_DSN`
  - when set to a non-empty DSN, `cmd/ecoflow-grpc-api` uses Postgres-backed control-plane storage,
  - when unset (or whitespace), service falls back to in-memory control-plane storage for local bootstrap/testing.

## Pulse Platform Node REST BFF (`apps/pulse-platform`)

- `PULSE_PLATFORM_HOST` (default `0.0.0.0`)
- `PULSE_PLATFORM_PORT` (default `18081`; standalone/debug-only port when running the BFF outside the cluster)
- `GRPC_API_ADDR` (default `127.0.0.1:9090`; internal Go gRPC API target)
- `GRPC_API_DEADLINE_MS` (default `10000`)
- `PULSE_PLATFORM_DEV_SUBJECT` (optional in local noop mode; recommended for local UI work so the BFF can resolve the current user's devices without request headers)
- `PULSE_PLATFORM_PUBLIC_PRECONNECT_ORIGINS` (optional comma/whitespace-delimited browser-facing origins for `Link: rel=preconnect` / `dns-prefetch` headers when API/WS are cross-origin)
- `PULSE_PLATFORM_HISTORY_RATE_LIMIT_MAX` (default `120`; per-IP budget for authenticated history endpoints)
- `PULSE_PLATFORM_HISTORY_RATE_LIMIT_WINDOW_MS` (default `60000`; rate-limit window for authenticated history endpoints)
- `NODE_AUTH_MODE` (`noop|keycloak`, default `noop`)
  - `noop`: local/dev mode, bearer token optional, forwarded only if present.
  - `keycloak`: validate bearer JWT using the shared Node JWKS package before forwarding to gRPC.
- `KEYCLOAK_ISSUER_URL` (required when `NODE_AUTH_MODE=keycloak`)
- `KEYCLOAK_AUDIENCE` (required when `NODE_AUTH_MODE=keycloak`)
- `KEYCLOAK_JWKS_URL` (optional override)
- `KEYCLOAK_ALLOW_MISSING_JWT` (default `false`; only for controlled local bootstrap)

## Pulse Realtime WebSocket Gateway (`apps/pulse-realtime-gateway`)

- `PULSE_REALTIME_GATEWAY_HOST` (default `0.0.0.0`)
- `PULSE_REALTIME_GATEWAY_PORT` (default `8082`; standalone/debug-only port when running the gateway outside the cluster)
- `GRPC_API_ADDR` (default `127.0.0.1:9090`; internal Go gRPC API target for device authz)
- `GRPC_API_DEADLINE_MS` (default `10000`)
- `GRPC_RECONNECT_BASE_MS` (default `250`)
- `GRPC_RECONNECT_MAX_MS` (default `2000`)
- `NATS_URLS` (comma/whitespace-delimited; default `nats://127.0.0.1:4222`)
- `VALKEY_ADDRS` (comma/whitespace-delimited; default `127.0.0.1:6379`; for local standalone gateway debugging, prefer a stable node port-forward such as `127.0.0.1:6380`)
- `VALKEY_USERNAME` (optional)
- `VALKEY_PASSWORD` (optional)
- `PROJECTION_KEY_PREFIX` (default `pulse:projection`; Valkey live snapshot key prefix)
- `TELEMETRY_SUBJECT_PREFIX` (default `pulse.telemetry`; NATS subject prefix used for ingest delta fanout)
- `WS_DELIVERY_FAST_INTERVAL_MS` (default `250`)
- `WS_DELIVERY_STEADY_INTERVAL_MS` (default `500`)
- `WS_DELIVERY_SLOW_INTERVAL_MS` (default `1000`)
- `WS_DELIVERY_HIGH_WATERMARK` (default `8`; coalesced delta threshold before stage promotion)
- `WS_BUFFERED_AMOUNT_HIGH_WATER_BYTES` (default `262144`; websocket bufferedAmount threshold for pressure)
- `WS_QUIET_TICKS_TO_RECOVER` (default `4`; quiet intervals required before stage recovery)
- `NODE_AUTH_MODE` (`noop|keycloak`, default `noop`)
  - `noop`: local/dev mode, token optional.
  - `keycloak`: validate websocket bearer JWT via shared JWKS auth before opening the session.
- `KEYCLOAK_ISSUER_URL` (required when `NODE_AUTH_MODE=keycloak`)
- `KEYCLOAK_AUDIENCE` (required when `NODE_AUTH_MODE=keycloak`)
- `KEYCLOAK_JWKS_URL` (optional override)
- `KEYCLOAK_ALLOW_MISSING_JWT` (default `false`; only for controlled local bootstrap)

Runtime behavior:
- the gateway authorizes device access through the internal Go gRPC API,
- serves the initial snapshot from Valkey projection state,
- then streams live deltas from NATS with staged backpressure degradation.

## Rollup Worker (`cmd/ecoflow-rollup-worker`)

- `ROLLUP_DB_DSN` (optional override; falls back to `CONTROL_PLANE_DB_DSN`)
- `CONTROL_PLANE_DB_DSN` (required when `ROLLUP_DB_DSN` is unset)
- `NATS_URLS`
- `TELEMETRY_SUBJECT_PREFIX`
- `TELEMETRY_SHARD_COUNT`
- `ROLLUP_INGEST_STREAM_NAME` (default `PULSE_TELEMETRY_INGEST`)
- `ROLLUP_CONSUMER_DURABLE` (default `rollup-timeseries-v1`)
- `ROLLUP_QUEUE_GROUP` (default `rollup-timeseries`)
- `ROLLUP_ACK_WAIT` (default `30s`)
- `ROLLUP_MAX_ACK_PENDING` (default `4096`)
- `ROLLUP_PROCESS_TIMEOUT` (default `3s`)
- `ROLLUP_DRAIN_TIMEOUT` (default `8s`)

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
  - web default: same-origin (`http://localhost` in local k3d) and should normally be left unset
  - native debug override: point directly at the public edge or standalone BFF when needed
  - native fallback behavior when unset: retry host variants (`<host>`, Expo host hints, `127.0.0.1`, `localhost`) and both local ports (`:18081`, then public-edge default port)
- `EXPO_PUBLIC_WS_URL`
  - default when unset: derive from API base (`ws(s)://<api-host>/ws`, trimming a trailing `/api` path when present)
  - native fallback behavior when unset: retry host variants (`<host>`, Expo host hints, `127.0.0.1`, `localhost`) and include both BFF-proxied (`/ws`) and standalone gateway (`:8082/ws`) paths
  - native debug override: set explicitly when bypassing BFF `/ws` proxy routing
- `EXPO_PUBLIC_ASSET_BASE_URL`
- `EXPO_PUBLIC_OIDC_ISSUER_URL` (Keycloak issuer URL for Authorization Code + PKCE)
- `EXPO_PUBLIC_OIDC_CLIENT_ID` (public OIDC client ID for Expo app)
- `EXPO_PUBLIC_OIDC_AUDIENCE` (optional audience for token exchange/validation alignment)
- `EXPO_PUBLIC_OIDC_SCOPES` (optional, default `openid profile email offline_access`)
  - optional absolute base URL for product/brand images in native and web,
  - when unset on web, app uses `/public` local paths,
  - large product images can be remote URI-based and are cached via `expo-image` (`memory-disk`),
  - brand/logo assets are bundled local app assets for smooth top-bar and menu rendering.

Runtime behavior:
- if OIDC is configured, the universal app waits for persisted auth-store hydration before issuing REST requests or opening the realtime websocket.
- if auth is configured but no valid access token exists, the telemetry engine remains in `auth_required` and the devices screen shows a sign-in-required state instead of opening anonymous realtime connections.
- websocket lifecycle is owned by `TelemetryEngineProvider`; token refresh/reconnect should not clear active device subscriptions at the screen hook layer.
- device solar history treats `404` from `/api/v1/devices/{id}/history/compare` as empty history (no blocking detail-page error banner).
- fleet summary solar rendering is conservative when all visible devices report inactive solar inputs (`solarChargingOn=false` and per-port watts `0`): the summary PV stat and fleet solar trend are clamped to `0` to avoid stale/inconsistent spike artifacts.
- the public app serves HTML with:
  - `no-cache, no-store, must-revalidate` on HTML responses,
  - immutable cache headers on hashed/static assets,
  - `Link` preload headers for discovered script/style assets,
  - optional `preconnect` / `dns-prefetch` hints for configured cross-origin endpoints,
  - `103 Early Hints` when the runtime supports `writeEarlyHints`.
- local k3d development is real-data and single-origin by default:
  - the public app serves `/` through `https://localhost`
  - the Node BFF is reached through `/api`
  - the realtime gateway is reached through `/ws`
  - browser-facing HTTP/2 is expected once TLS ingress is active
  - browser-facing HTTP/3 is deferred with the current ingress-nginx controller path
