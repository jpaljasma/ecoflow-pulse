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

Runtime note:

- request-serving EcoFlow, Pecron, and Anker SOLIX integrations use per-user
  credential material from `provider_credentials` in Postgres,
- provider credential config is non-secret JSON. Pecron stores `region` as
  `us`, `eu`, or `cn`; Anker SOLIX stores `server` as `com` or `eu` plus
  two-letter `country`. Clients receive this config with masked credential
  metadata only.
- `ECOFLOW_ENV` / `ECOFLOW_BASE_URL` supply the shared provider profile/endpoints
  used by the EcoFlow adapter runtime and local env-backed credential tests.
- the local services Helm release now injects those two vars into the Go service
  pods through `deploy/env/local/values.services.yaml`.
- steady-state ingest MQTT sessions derive their broker `ClientID` from the
  provider device ID plus `PULSE_ENV`, so `local` and `cloud` produce distinct
  client IDs for the same EcoFlow device.
- Pecron cloud runtime endpoints are selected by saved provider credential
  `region`. Pecron REST snapshots and MQTT-over-WebSocket `kv` payloads are
  decoded into canonical provider telemetry before entering the shared ingest
  publisher path.
- Anker SOLIX cloud runtime endpoints are selected by saved provider credential
  `server` and `country`. REST is used for login, discovery, and MQTT
  certificate bootstrap; MQTT TLS telemetry is decoded into canonical provider
  telemetry before entering the shared ingest publisher path.

## Logging and Process Safety

Common service/worker logging knobs (`cmd/ecoflow-ingest-worker`, `cmd/ecoflow-projection-worker`, `cmd/ecoflow-archive-worker`, `cmd/ecoflow-gap-detector`, `cmd/ecoflow-gap-repair-worker`, `cmd/ecoflow-grpc-api`, `cmd/ecoflow-replay-cli`, `cmd/ecoflow-dev-seed`):

- `LOG_LEVEL` (default `info`)
- `LOG_ASYNC_DISABLED` (default `false`; when `true`, bypass async queue)
- `LOG_ASYNC_QUEUE_SIZE` (default `8192`; bounded queue capacity)
- `LOG_ASYNC_BYPASS_LEVEL` (default `warn`; warn/error logs bypass queue sync)
- `LOG_METRICS_INTERVAL` (default `30s`; set `0` to disable async queue metrics logs)

Durable ingest consumers (`cmd/ecoflow-projection-worker`, `cmd/ecoflow-archive-worker`, `cmd/ecoflow-rollup-worker`, `cmd/ecoflow-inference-worker`) also expose subscription watchdog state on their metrics server:

- `/readyz` returns unavailable while the required JetStream durable consumer is not attached
- `pulse_worker_consumer_ready`
- `pulse_worker_consumer_subscribe_total{outcome="success|failure"}`
- `pulse_worker_consumer_consecutive_subscribe_failures`

Ingest payload debug knobs (`cmd/ecoflow-ingest-worker`):

- `INGEST_MQTT_LOG_PAYLOAD_DEBUG` (default `false`; sampled MQTT debug logs stay off the hot path and redact topics/raw payload bodies)
- `INGEST_MQTT_LOG_PAYLOAD_SAMPLE_EVERY` (default `100`; sampled debug metadata logging interval)
- `INGEST_QUOTA_FETCH_TIMEOUT` (default `10s`; per-call timeout for `GetDeviceAllQuota()` bootstrap/refresh pulls)
- `INGEST_QUOTA_REFRESH_INTERVAL` (default `30s`; periodic quota refresh cadence while an MQTT session is alive)
- `INGEST_QUOTA_REFRESH_JITTER` (default `0.20`; proportional jitter applied to periodic quota refresh scheduling)
- `INGEST_PECRON_SNAPSHOT_FETCH_TIMEOUT` (default `10s`; per-call timeout for Pecron REST bootstrap/refresh snapshots)
- `INGEST_PECRON_SNAPSHOT_REFRESH_INTERVAL` (default `70s`; Pecron REST snapshot refresh cadence while MQTT is alive; values below `63s` are rejected to avoid Pecron `4026` rate-limit exhaustion)
- `INGEST_PECRON_SNAPSHOT_REFRESH_JITTER` (default `0.20`; proportional jitter applied to Pecron REST snapshot refresh scheduling)
- `INGEST_ANKER_SOLIX_REALTIME_TRIGGER_TIMEOUT` (default `300s`; Anker SOLIX MQTT realtime trigger lifetime)
- `INGEST_ANKER_SOLIX_REALTIME_TRIGGER_REFRESH` (default `270s`; refresh cadence before the trigger expires)
- `INGEST_QUOTA_METRICS_INTERVAL` (default `30s`; jittered aggregate quota refresh metrics log interval, set `0` to disable)
- `INGEST_MQTT_CLIENT_ID_NAMESPACE` (optional; overrides the environment namespace used when deriving steady-state EcoFlow MQTT client IDs; defaults to `PULSE_ENV`)
- `PROVIDER_MQTT_SESSION_CACHE_KEY_PREFIX` (default `pulse:provider-mqtt-session`; shared Valkey cache key prefix for encrypted provider MQTT session/certification material)
- `PROVIDER_MQTT_SESSION_CACHE_IDLE_TTL` (default `30m`; sliding idle TTL for encrypted provider MQTT session material)
- `PROVIDER_MQTT_SESSION_CACHE_MAX_AGE` (default `6h`; conservative hard cap when the provider does not return an explicit expiry)
- `PROVIDER_MQTT_SESSION_CACHE_LOCAL_TTL` (default `5s`; valkey-go client-side-cache TTL for provider-session cache reads)

Shared Go Valkey cache knobs:

- `VALKEY_CACHE_CLIENT_SIDE_CACHE_ENABLED` (default `true` for shared cache clients; legacy lease/script clients still keep valkey-go client-side caching disabled)
- `VALKEY_CACHE_SIZE_EACH_CONN` (optional bytes; valkey-go client-side-cache size per connection, default library behavior when empty)
- `VALKEY_CACHE_CLIENT_TRACKING_OPTIONS` (default `OPTIN`; passed to valkey-go client tracking for cache clients)
- `VALKEY_CACHE_ENCRYPTION_KEY_ID` (required with `VALKEY_CACHE_ENCRYPTION_KEYS` before sensitive provider-session caching is enabled)
- `VALKEY_CACHE_ENCRYPTION_KEYS` (secret; comma-separated `key-id:base64-aes-key` entries; provider-session caching is bypassed when unset)

## Server Runtime

- `ECOFLOW_SERVER_ADDR` (default `:8080`)
- `ECOFLOW_SERVER_GOMAXPROCS`
- `ECOFLOW_SERVER_COMPRESSION_MIN_BYTES` (default `1024`)
- `ECOFLOW_SERVER_GZIP_LEVEL` (default `5`)
- `ECOFLOW_SERVER_DEFLATE_LEVEL` (default `5`)
- `ECOFLOW_SERVER_BROTLI_LEVEL` (default `5`, `moderncompress` builds)
- `ECOFLOW_SERVER_ZSTD_LEVEL` (default `3`, `moderncompress` builds)

## Shared Postgres Search-Path Cutover

- `DB_SCHEMA_SEARCH_PATH` (optional)
  - when set, Go Postgres clients in the runtime path connect with
    `search_path=<value>`
  - this is the runtime cutover knob intended for future `pgroll` transitions
  - when unset, services keep the database default schema behavior

## Internal gRPC API Runtime

- `PULSE_ENV` (`local|dev|staging|prod`, default `local`)
- `GRPC_LISTEN_ADDR` (default from grpc server profile; typically `:9090` in local/dev)
- `GRPC_SERVICE_MODE` (`telemetry|energy`, default `telemetry`)
  - `telemetry`: serves `TelemetryService` live snapshot/stream RPCs plus control-plane, weather, solar-forecast, and inference services.
  - `energy`: serves `EnergyService` history/dashboard RPCs on a separately deployable internal gRPC workload.
- `GRPC_AUTH_MODE` (`noop|keycloak`, default `noop`)
  - `noop`: development-only pass-through auth mode.
  - `keycloak`: validates bearer JWTs via Keycloak OIDC/JWKS and injects claims into gRPC context.
- `KEYCLOAK_ISSUER_URL` (required when `GRPC_AUTH_MODE=keycloak`)
- `KEYCLOAK_AUDIENCE` (optional; when set, JWT audience must match)
- `KEYCLOAK_JWKS_URL` (optional override; lets internal gRPC services fetch JWKS from an in-cluster Keycloak URL while still validating the public issuer)
- `GRPC_AUTH_ALLOW_MISSING_JWT` (default `false`; optional only for controlled local bootstrap)
- `GRPC_HISTORY_GZIP_MIN_BYTES` (default `16384`; when a `QueryRollupRange` or `CompareRollupRange` response is at least this serialized size, grpc-api enables gzip for that unary response)
- `CONTROL_PLANE_DB_DSN`
  - when set to a non-empty DSN, `cmd/ecoflow-grpc-api` uses Postgres-backed control-plane storage,
  - when unset (or whitespace), service falls back to in-memory control-plane storage for local bootstrap/testing.
- `VALKEY_ADDRS` (comma/whitespace-delimited; enables the Valkey-backed inference reader when set)
- `VALKEY_SENTINEL_MASTER_SET` (optional; when set, Go Valkey clients resolve the current primary through Sentinel instead of treating `VALKEY_ADDRS` as direct node addresses)
- `VALKEY_SENTINEL_USERNAME` (optional)
- `VALKEY_SENTINEL_PASSWORD` (optional)
- `VALKEY_USERNAME` (optional)
- `VALKEY_PASSWORD` (optional)
- `INFERENCE_KEY_PREFIX` (default `pulse:inference`; Valkey device-insight read-model key prefix)
- `INFERENCE_ENERGY_COMPARISON_CACHE_LOCAL_TTL` (default `30s`; valkey-go local TTL for energy-comparison cache reads)
- `ENERGY_CACHE_KEY_PREFIX` (default `pulse:energy`; Valkey cache key prefix for EnergyService calendar/PV history caches)
- `ENERGY_CALENDAR_CACHE_LOCAL_TTL` (default `5s`; valkey-go local TTL for EnergyService calendar cache reads)
- `ENERGY_PV_PORT_HISTORY_CACHE_LOCAL_TTL` (default `5s`; valkey-go local TTL for EnergyService PV-port history cache reads)
- `SOLAR_FORECAST_VERIFICATION_INTERVAL` (default `15m`; when `> 0`, the solar verifier processes matured issued hourly rows on each pass)
- `SOLAR_FORECAST_VERIFICATION_BATCH_LIMIT` (default `1536`; max number of pending solar hourly records the verification loop will process per pass)
- `WEATHER_HOT_CACHE_TTL` (default `4h`; the canonical hot-cache and snapshot freshness window for weather reads)
- `WEATHER_REFRESH_INTERVAL` (default `30m` for standalone grpc-api; Helm keeps request-serving `go-grpc-api` traffic-only with `0s`)
- `WEATHER_RECENT_ACTIVE_WINDOW` (default `168h`; active-location lookback for background weather refresh)

Deployment note:

- local/dev Helm values keep request-serving `go-grpc-api` replicas traffic-only by setting `SOLAR_FORECAST_VERIFICATION_INTERVAL=0s` there, and run the background loop on separate `go-solar-verification` workers that cooperatively claim pending runs from Postgres.
- local/dev Helm values also keep request-serving weather refresh traffic-only by setting `WEATHER_REFRESH_INTERVAL=0s` on `go-grpc-api`, with background weather refresh and prune work moved to the dedicated `go-scheduler` worker.

## Background Scheduler Runtime

The dedicated `cmd/ecoflow-scheduler` worker owns reusable recurring jobs for forecast refresh and hot-data pruning.

- `SCHEDULER_METRICS_LISTEN_ADDR` (default empty; optional metrics server address)
- `SCHEDULER_POLL_INTERVAL` (default `1m`; scheduler claim loop cadence)
- `SCHEDULER_CLAIM_LIMIT` (default `8`; max recurring jobs claimed per poll)
- `SCHEDULER_WEATHER_REFRESH_SCAN_INTERVAL` (default `5m`; how often the scheduler scans for due weather refresh candidates)
- `SCHEDULER_WEATHER_PRUNE_INTERVAL` (default `24h`; weather hot-data prune cadence)
- `SCHEDULER_SOLAR_REFRESH_INTERVAL` (default `24h`; active solar forecast refresh cadence)
- `SCHEDULER_SOLAR_REFRESH_ACTIVE_WINDOW` (default `720h`; how far back recent solar forecast runs keep a site/device eligible for scheduled refresh)
- `SCHEDULER_SOLAR_REFRESH_BATCH_LIMIT` (default `64`; max active solar forecast inputs refreshed per scheduler run)
- Helm exposes the same defaults through `runtime.workers.scheduler.solarRefreshInterval`, `runtime.workers.scheduler.solarRefreshActiveWindow`, and `runtime.workers.scheduler.solarRefreshBatchLimit`; cloud values keep the scheduler worker enabled so panel-capacity learning refreshes without manual retraining. Sustained high PV-max evidence can lift a stale learned single-device capacity envelope promptly, while aggregate site series and isolated spikes remain bounded.
- `SCHEDULER_SOLAR_PRUNE_INTERVAL` (default `24h`; solar hot-data prune cadence)
- `WEATHER_VERIFICATION_RETENTION` (default `720h`; weather yesterday-verification retention)
- `WEATHER_REFRESH_CANDIDATE_RETENTION` (default `336h`; stale weather refresh-candidate retention)
- weather snapshot pruning keeps roughly the most recent `48h` of per-location bundles for verification lookback, then compacts older rows down to the latest surviving snapshot per location before inactive-location cleanup
- `SOLAR_FORECAST_RUN_RETENTION` (default `336h`; raw solar forecast run retention)
- `SOLAR_FORECAST_DAILY_VERIFICATION_RETENTION` (default `1440h`; solar daily verification retention)
- `SOLAR_FORECAST_PRUNE_RUN_BATCH_LIMIT` (default `250`; max runs pruned per batch)
- `SOLAR_FORECAST_PRUNE_DAILY_BATCH_LIMIT` (default `1000`; max daily verification rows pruned per batch)

## Shared Archive Object Store Runtime

These settings are shared across the archive worker, replay CLI, rollup rebuild,
gap repair, history backfill, and gRPC history/archive readers:

- `ARCHIVE_OBJECT_PROVIDER` (`minio|gcs`, default `minio`)
- `ARCHIVE_OBJECT_ENDPOINT`
  - required for `minio`,
  - ignored for steady-state native GCS access.
- `ARCHIVE_OBJECT_ACCESS_KEY`
  - required for `minio`,
  - normally unset for native GCS access through Workload Identity.
- `ARCHIVE_OBJECT_SECRET_KEY`
  - required for `minio`,
  - normally unset for native GCS access through Workload Identity.
- `ARCHIVE_OBJECT_BUCKET` (required logical bucket name, default `pulse-telemetry-raw`)
- `ARCHIVE_OBJECT_PREFIX` (default `raw`)
- `ARCHIVE_OBJECT_REGION`
  - used for S3/MinIO signing,
  - optional metadata for GCS flows.
- `ARCHIVE_OBJECT_GCS_PROJECT_ID` (optional; used for cloud logging/bucket bootstrap context)
- `ARCHIVE_OBJECT_SECURE` (default `false` for local MinIO; typically `true` in cloud)
- `ARCHIVE_OBJECT_AUTO_CREATE_BUCKET`
  - useful in local MinIO bootstrap,
  - should normally stay `false` in cloud/GCS once the bucket is provisioned.
- `INGEST_NATS_JS_REPLICAS`
  - JetStream replica factor for the ingest stream bootstrap,
  - set this to `1` when a bootstrap profile intentionally runs NATS without clustering.
- `INGEST_NATS_JS_MAX_AGE`
  - JetStream ingest retention horizon (default `12h`; hosted cloud uses `6h`),
  - keep this bounded so recreated durable consumers cannot accumulate a
    multi-day backlog before live telemetry catches up.
- `GAP_REPAIR_NATS_JS_REPLICAS`
  - JetStream replica factor for the gap-repair stream bootstrap,
  - set this to `1` when a bootstrap profile intentionally runs NATS without clustering.
- `ARCHIVE_MANIFEST_DB_DSN`
  - archive-manifest Postgres DSN for archive/rebuild/reconcile flows,
  - archive worker falls back to `CONTROL_PLANE_DB_DSN` when unset.

## Pulse MQTT Emulator Runtime

- `PULSE_MQTT_EMULATOR_ENABLED` (default `true`; set `false` in cloud when the emulator service is intentionally absent)
- `PULSE_MQTT_EMULATOR_BASE_URL`
- `PULSE_MQTT_EMULATOR_BROKER_HOST`
- `PULSE_MQTT_EMULATOR_BROKER_PORT`
- `PULSE_MQTT_EMULATOR_TLS_SERVER_NAME`

Cloud note:

- the hosted `cloud` deploy overlay switches this runtime to
  `ARCHIVE_OBJECT_PROVIDER=gcs` and relies on GKE Workload Identity instead of
  HMAC keys for steady-state bucket access.

## Pulse Platform Node REST BFF (`apps/pulse-platform`)

- `PULSE_PLATFORM_HOST` (default `0.0.0.0`)
- `PULSE_PLATFORM_PORT` (default `18081`; standalone/debug-only port when running the BFF outside the cluster)
- `GRPC_API_ADDR` (default `127.0.0.1:9090`; internal Go gRPC API target)
- `ENERGY_GRPC_API_ADDR` (default empty -> falls back to `GRPC_API_ADDR`; dedicated internal Go gRPC energy/history target)
- `GRPC_API_DEADLINE_MS` (default `10000`)
- `PULSE_PLATFORM_DEV_SUBJECT` (optional in local noop mode; recommended for local UI work so the BFF can resolve the current user's devices without request headers)
- `PULSE_PLATFORM_PUBLIC_PRECONNECT_ORIGINS` (optional comma/whitespace-delimited browser-facing origins for `Link: rel=preconnect` / `dns-prefetch` headers when API/WS are cross-origin)
- `PULSE_PLATFORM_CORS_ALLOWED_ORIGINS` (optional comma/whitespace-delimited exact origins to allow for browser CORS requests; when unset, the public app keeps the existing permissive origin reflection behavior. Local/dev defaults include `http://localhost:8081` and `https://localhost:8081` for Expo web-dev access to `/api/*`.)
- hosted cloud deployments should set `PULSE_PLATFORM_CORS_ALLOWED_ORIGINS` to
  the first-party web origin plus the explicit localhost loopback origins used
  by Expo web-dev (`http://localhost:8081`, `https://localhost:8081`,
  `http://127.0.0.1:8081`, `https://127.0.0.1:8081`).
- `GET /metrics` is exposed on the same public-app HTTP service for Prometheus/OTEL scrape collection. In Helm, enable `runtime.publicApp.metrics.serviceMonitor.enabled=true` to have observability-lite discover the endpoint automatically.
- Expo web-dev note: when the universal app is served from loopback `:8081` and no explicit `EXPO_PUBLIC_API_URL` is set, the browser client now prefers `https://localhost` / `wss://localhost` automatically so local ingress redirects do not surface as browser CORS failures.
- Empty-string `EXPO_PUBLIC_*` values are treated as unset in the universal web runtime. This matters for Docker/Helm-driven web builds, where missing args can appear as `""`; the browser should still fall back to the secure localhost defaults instead of generating broken API/WS URLs.
- `PULSE_PLATFORM_HISTORY_RATE_LIMIT_MAX` (default `120`; per-IP budget for authenticated history endpoints)
- `PULSE_PLATFORM_HISTORY_RATE_LIMIT_WINDOW_MS` (default `60000`; rate-limit window for authenticated history endpoints)
- `PULSE_PLATFORM_BFF_CACHE_ENABLED` (default `false`; enables the optional Node BFF in-process response cache for selected public read routes)
- `PULSE_PLATFORM_BFF_CACHE_MAX_ENTRIES` (default `1000`; maximum in-process BFF response-cache entries per public-app pod)
- `PULSE_PLATFORM_WEATHER_FORECAST_CACHE_TTL_MS` (default `30000`; BFF edge TTL for `GET /api/v1/weather/forecast` when the BFF cache is enabled)
- `PULSE_PLATFORM_WEATHER_YESTERDAY_CACHE_TTL_MS` (default `300000`; BFF edge TTL for `GET /api/v1/weather/yesterday` when the BFF cache is enabled)
- `NODE_AUTH_MODE` (`noop|keycloak`, default `noop`)
  - `noop`: local/dev mode, bearer token optional, forwarded only if present.
  - `keycloak`: validate bearer JWT using the shared Node JWKS package before forwarding to gRPC.
- `KEYCLOAK_ISSUER_URL` (required when `NODE_AUTH_MODE=keycloak`)
- `KEYCLOAK_AUDIENCE` (required when `NODE_AUTH_MODE=keycloak`)
- `KEYCLOAK_JWKS_URL` (optional override)
- `KEYCLOAK_USERINFO_URL` (optional override; lets the public app fetch Keycloak `userinfo` through an in-cluster URL for background social-profile/avatar refresh)
- `KEYCLOAK_ALLOW_MISSING_JWT` (default `false`; only for controlled local bootstrap)

## Pulse Realtime WebSocket Gateway (`apps/pulse-realtime-gateway`)

- `PULSE_REALTIME_GATEWAY_HOST` (default `0.0.0.0`)
- `PULSE_REALTIME_GATEWAY_PORT` (default `8082`; standalone/debug-only port when running the gateway outside the cluster)
- `GRPC_API_ADDR` (default `127.0.0.1:9090`; internal Go gRPC API target for device authz)
- `GRPC_API_DEADLINE_MS` (default `10000`)
- `GRPC_RECONNECT_BASE_MS` (default `250`)
- `GRPC_RECONNECT_MAX_MS` (default `2000`)
- `NATS_URLS` (comma/whitespace-delimited; default `nats://127.0.0.1:4222`)
- `VALKEY_ADDRS` (comma/whitespace-delimited; default `127.0.0.1:6379`; when `VALKEY_SENTINEL_MASTER_SET` is set, point this at Sentinel endpoints such as `127.0.0.1:26379`)
- `VALKEY_SENTINEL_MASTER_SET` (optional; when set, the gateway discovers the writable Valkey primary via Sentinel)
- `VALKEY_SENTINEL_USERNAME` (optional)
- `VALKEY_SENTINEL_PASSWORD` (optional)
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
- `GET /metrics` is exposed on the same realtime-gateway HTTP service for Prometheus/OTEL scrape collection. In Helm, enable `runtime.realtimeGateway.metrics.serviceMonitor.enabled=true` to have observability-lite discover the endpoint automatically.

Runtime behavior:
- the gateway authorizes device access through the internal Go gRPC API,
- serves the initial snapshot from Valkey projection state,
- then streams live deltas from NATS with staged backpressure degradation.

## Local/dev Valkey durability baseline

For `pulse-platform` local/dev, Valkey is configured in replication + Sentinel mode with
PVC-backed persistence enabled on the Valkey data nodes. The current baseline is:

- AOF enabled (`appendonly yes`)
- primary persistence enabled
- replica persistence enabled
- Sentinel enabled with `quorum=2`
- Sentinel graceful primary shutdown wait enabled
- Sentinel automated cluster recovery enabled for cold-restart edge cases
- PVC retention set to `Retain` on scale/delete for Valkey data PVCs

This means cold restarts are expected to preserve Valkey-backed cache/snapshot state as long as
the underlying PVCs remain intact. Valkey is still not the system of record for authoritative
business data; Postgres / Timescale / archive storage remain authoritative.

## Local/dev MinIO raw archive durability baseline

For `pulse-platform` local/dev, MinIO is the authoritative raw replay archive.
The current local baseline is:

- standalone MinIO with `persistence.enabled=true`
- PVC-backed object data volume
- PVC annotated with `helm.sh/resource-policy=keep` in local values so routine
  Helm lifecycle does not casually discard raw archive history
- local default bucket bootstrap for `pulse-telemetry-raw`

This means replay, rollup rebuild, and gap repair may rely on the local MinIO
archive surviving restarts. Ephemeral local MinIO is not an acceptable steady
state when historical trust matters.

Operational rule:

- before trusting a local historical rebuild after archive/storage churn, run a
  direct archive-vs-manifest audit (`make dev-archive-audit`) for the target
  window
- if the audit reports stale manifest rows pointing at missing MinIO objects,
  run `make dev-archive-reconcile` for the same window before retrying the
  audit
- if manifest rows and direct MinIO listing disagree, treat that as archive
  integrity drift and fail closed instead of trusting manifest-only counts

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

## Rollout Migration Job (`cmd/ecoflow-db-migrate-job`)

- `CONTROL_PLANE_DB_DSN` (optional override; if unset, the runner builds a DSN from the `DB_MIGRATION_DB_*` fields below)
- `DB_MIGRATION_DB_HOST` (required when `CONTROL_PLANE_DB_DSN` is unset)
- `DB_MIGRATION_DB_PORT` (default `5432`)
- `DB_MIGRATION_DB_NAME` (default `pulse`)
- `DB_MIGRATION_DB_USER` (required when `CONTROL_PLANE_DB_DSN` is unset)
- `DB_MIGRATION_DB_PASSWORD` (required when `CONTROL_PLANE_DB_DSN` is unset)
- `DB_MIGRATION_DB_SSLMODE` (default `disable`)
- `DB_MIGRATIONS_DIR` (default `/app/deploy/db/migrations`)
- `DB_MIGRATION_ENVIRONMENT` (`local|dev|staging|prod`, default `dev`)
- `DB_MIGRATION_REQUIRE_BACKUP` (default `false`; should be `true` in staging/prod rollout policy)
- `DB_MIGRATION_BACKUP_REF` (required when `DB_MIGRATION_REQUIRE_BACKUP=true`)
- `DB_MIGRATION_FORWARD_ONLY` (default `true`; the rollout path intentionally rejects non-forward-only configuration)

## Inference Worker (`cmd/ecoflow-inference-worker`)

- `CONTROL_PLANE_DB_DSN` (required; used to resolve provider-device model/capability context)
- `NATS_URLS`
- `VALKEY_ADDRS`
- `VALKEY_SENTINEL_MASTER_SET` (optional)
- `VALKEY_SENTINEL_USERNAME` (optional)
- `VALKEY_SENTINEL_PASSWORD` (optional)
- `VALKEY_USERNAME` (optional)
- `VALKEY_PASSWORD` (optional)
- `INFERENCE_KEY_PREFIX` (default `pulse:inference`)
- `INFERENCE_CONTEXT_CACHE_TTL` (default `1m`; control-plane device-context cache TTL)
- `INFERENCE_CONTEXT_CACHE_LOCAL_TTL` (default `10s`; valkey-go local TTL for inference control-plane device-context cache reads)
- `INFERENCE_ENERGY_COMPARISON_CACHE_LOCAL_TTL` (default `30s`; valkey-go local TTL for energy-comparison cache reads when the inference store is used as a cache)
- `TELEMETRY_SUBJECT_PREFIX`
- `TELEMETRY_SHARD_COUNT`
- `INFERENCE_INGEST_STREAM_NAME` (default `PULSE_TELEMETRY_INGEST`)
- `INFERENCE_CONSUMER_DURABLE` (default `inference-device-insights-v1`)
- `INFERENCE_QUEUE_GROUP` (default `inference-device-insights`)
- `INFERENCE_ACK_WAIT` (default `30s`)
- `INFERENCE_MAX_ACK_PENDING` (default `4096`)
- `INFERENCE_PROCESS_TIMEOUT` (default `3s`)
- `INFERENCE_DRAIN_TIMEOUT` (default `8s`)

## Explicit Dev Seed (`cmd/ecoflow-dev-seed`)

- `ECOFLOW_DEV_ACCESS_KEY` (required)
- `ECOFLOW_DEV_SECRET_KEY` (required)
- `CONTROL_PLANE_DB_DSN` (required)
- `ECOFLOW_DEV_USER_SUBJECT` (default `dev-user@example.com`)
- `ECOFLOW_DEV_USER_EMAIL` (default = `ECOFLOW_DEV_USER_SUBJECT`)
- `ECOFLOW_DEV_PROVIDER` (default `ecoflow`)
- `ECOFLOW_DEV_SEED_SNS` (comma/whitespace-delimited serials, default `DEMOD2M00001057,DEMODPU0000294`)

## User Subject Reconcile (`cmd/ecoflow-user-subject-reconcile`)

- `CONTROL_PLANE_DB_DSN` (required unless `-dsn` is passed)
- `-dsn` (optional CLI flag override for the control-plane Postgres DSN)
- `-email` (required verified email to match in the restored `users` table)
- `-user-subject` (required new Keycloak/OpenID subject to assign)

Behavior:

- if the target subject already exists for the same verified email, the command
  returns success without changing ownership,
- if the target subject already belongs to a different email, it fails closed
  with a subject-conflict error,
- otherwise it remaps the verified-email-matching user row to the new subject.

## Universal App (Expo)

- `EXPO_PUBLIC_API_URL`
  - web default: same-origin (`http://localhost` in local k3d) and should normally be left unset
  - blank-string values are treated as unset
  - native debug override: point directly at the public edge or standalone BFF when needed
  - native fallback behavior when unset: retry host variants (`<host>`, Expo host hints, `127.0.0.1`, `localhost`) and both local ports (`:18081`, then public-edge default port)
- `EXPO_PUBLIC_WS_URL`
  - default when unset: derive from API base (`ws(s)://<api-host>/ws`, trimming a trailing `/api` path when present)
  - blank-string values are treated as unset
  - native fallback behavior when unset: retry host variants (`<host>`, Expo host hints, `127.0.0.1`, `localhost`) and include both BFF-proxied (`/ws`) and standalone gateway (`:8082/ws`) paths
  - native debug override: set explicitly when bypassing BFF `/ws` proxy routing
- `EXPO_PUBLIC_ASSET_BASE_URL`
  - optional absolute base URL for product/brand images in native and web,
  - when unset on web, app uses `/public` local paths,
  - large product images can be remote URI-based and are cached via `expo-image` (`memory-disk`),
  - brand/logo assets are bundled local app assets for smooth top-bar and menu rendering.
- `EXPO_PUBLIC_OIDC_ISSUER_URL` (Keycloak issuer URL for Authorization Code + PKCE)
- `EXPO_PUBLIC_OIDC_CLIENT_ID` (public OIDC client ID for Expo app)
- `EXPO_PUBLIC_OIDC_AUDIENCE` (optional audience for token exchange/validation alignment)
- `EXPO_PUBLIC_OIDC_SCOPES` (optional, default `openid profile email offline_access`)
- `EXPO_PUBLIC_CLOUD_API_URL` (cloud BFF base URL used by the `cloud` connection profile)
- `EXPO_PUBLIC_CLOUD_WS_URL` (cloud websocket URL; when unset, derived from `EXPO_PUBLIC_CLOUD_API_URL`)
- `EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL` (cloud Keycloak issuer URL for the `cloud` connection profile)
- `EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID` (cloud public OIDC client ID)
- `EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE` (optional cloud audience override)
- `EXPO_PUBLIC_CLOUD_OIDC_SCOPES` (optional cloud scopes override; defaults to `openid profile email offline_access`)
- `EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE` (`local|cloud`; defaults to `local` unless the requested profile is not configured)
- `EXPO_PUBLIC_LOCAL_DATA_PLANE` (`local|cloud`; defaults to `local`)

Runtime behavior:
- the universal app now persists a single app-wide connection profile
  (`local` or `cloud`) and resolves API base URL, websocket URL, OIDC issuer,
  and OIDC client together from that selection.
- when `EXPO_PUBLIC_LOCAL_DATA_PLANE=cloud`, the `cloud` profile uses the
  local HTTPS edge, local OIDC issuer, and cloud-backed database/realtime data
  plane. In product UI this is Cloud, not Local, because the selected source of
  user data is cloud.
- users can switch that data source directly from the shared app menu or from
  `Settings -> Data source`; the product-facing choices are `Local` and
  `Cloud`.
- if OIDC is configured, the universal app waits for persisted auth-store hydration before issuing REST requests or opening the realtime websocket.
- switching the connection profile clears auth/query state by profile key,
  recreates the realtime engine with the selected websocket endpoint, and
  forces re-login when the issuer or client changes between profiles.
- if auth is configured but no valid access token exists, the telemetry engine remains in `auth_required` and the devices screen shows a sign-in-required state instead of opening anonymous realtime connections.
- authenticated REST requests that receive `401` should first try to recover with a fresher in-memory token or a one-time refresh-token exchange before falling back to `/login`; if recovery fails, redirect to login with a clear re-authentication message for long-idle sessions.
- profile and homepage queries should preserve the last successful payload during routine refetch so deploy rollouts do not flash empty-state content; when a profile is missing `avatarUrl`, the profile page may trigger a one-shot authenticated `/api/v1/me/identity-refresh` in the background to backfill provider-managed social data.
- websocket lifecycle is owned by `TelemetryEngineProvider`; token refresh/reconnect should not clear active device subscriptions at the screen hook layer.
- on web, websocket reconnect must retry the current browser-origin endpoint directly; browser sessions should not rotate through native-dev fallback hosts such as `127.0.0.1` or `localhost` after deploy-induced disconnects.
- local browser auth/realtime flows depend on the shared ingress routing all three public paths correctly:
  - `/realms` and `/resources` must reach Keycloak,
  - `/ws` must reach the realtime gateway,
  - `/` must keep serving the public app.
- universal-app theming contract:
  - the persisted user preference stores the palette family (`original` or `new`), not a fixed light/dark override,
  - light vs dark mode still follows system appearance automatically,
  - the root Tamagui provider must stay on base `light` / `dark` themes, with palette-specific themes (`original-*`, `new-*`) applied as nested theme layers,
  - on web, dark-mode resolution must follow `window.matchMedia('(prefers-color-scheme: dark)')` and update when the browser appearance changes,
  - when the resolved web theme changes, the app must also update `html`, `body`, and the app root background/color-scheme so browser chrome and page background stay aligned with the active theme,
  - reusable UI colors should be defined once in the theme catalog semantic palette and consumed through shared theme helpers instead of repeated inline `hex`/`rgba` literals in feature components.
- device solar history treats `404` from `/api/v1/devices/{id}/history/compare` as empty history (no blocking detail-page error banner).
- device and fleet solar-history queries reuse one response payload for today's
  total, yesterday's total, delta, and both chart series; they refresh on the
  normal polling interval during the day and roll to a new query key shortly
  after local midnight so `today`/`yesterday` swap cleanly without requiring a
  manual reload.
- solar history fetches the full previous local day for `Yesterday total`, but
  percentage deltas compare `Today so far` against yesterday through the latest
  current-day solar bucket so the running comparison is not diluted by the rest
  of yesterday's production.
- the universal client computes and sends explicit `compareFrom` / `compareTo`
  local-day bounds for solar history; DST-sensitive "previous period"
  subtraction on the server is not sufficient for the day after a clock shift.
- the solar-generated chart renders weather-provided sunrise/sunset when
  available, rounded to surrounding half-hour marks, in 10-minute step buckets;
  without weather sun times it falls back to the local `06:00` -> `20:00`
  window. It supports bucket inspection with hover on web and tap on native via
  a shared crosshair/tooltip overlay.
- the `Energy Impact` widget on `/devices` and `/device/{id}` uses real
  measured solar generation from `todayWh` only; it does not annualize or
  extrapolate. The current default factor is `NYUP` (`egrid2023_rev2`). See
  [`solar-avoided-emissions.md`](solar-avoided-emissions.md).
- `/energy` also embeds the same `Energy Impact` card, but that copy is driven
  by the page's global device/window controls instead of a separate local
  period switch, so the impact wording always follows the selected Energy
  dashboard scope and date range.
- `/energy` `Today` uses local midnight to now and compares against yesterday
  from local midnight to the same local clock time; `Last 24h` remains a
  rolling 24-hour window compared with the preceding 24 hours. This keeps
  Energy balance and Power profile buckets aligned while preserving
  local-calendar semantics for the default dashboard.
- `/energy-calendar` is a primary navigation route after `Energy`; it loads one
  Sunday-start visible month for the selected fleet/device scope and prefetches
  adjacent months. Its selected-month totals include only in-month days, while
  muted leading/trailing days can still show their real daily totals.
- Energy and Energy Calendar resolve user-facing local-day timezone from the
  logged-in profile. App-generated Energy URLs and BFF requests should not carry
  `tz`/`timezone` query parameters.
- Energy Calendar treats the current profile-local month as live data: current
  day tiles use fresh current-day rollups, refetch on the live path, and refresh
  after the next profile-local midnight; closed historical months can stay
  cached aggressively. The universal client caches historical month payloads
  and warms adjacent non-live months, while the Go Energy service caches
  already-computed non-current month responses behind a bounded in-memory cache.
- `/energy?preset=today&date=YYYY-MM-DD` selects an explicit local calendar
  day. Historical selected dates query the full local midnight-to-midnight day;
  the current local day still queries local midnight to now, and comparison
  uses the prior local-day window.
- the `/energy` page keeps its `Solar against load` controls collapsed by
  default behind an explicit expand/collapse affordance so the summary content
  stays visible first on smaller screens.
- local energy price preferences live on `Profile` alongside timezone and
  weather-location preferences; the Energy dashboard reads that same locally
  persisted setting for estimated value and AC input cost.
- the same widget also includes a conservative mature-tree equivalent based on
  a `0.045 kg CO2e/kWh` PV lifecycle benchmark and `21.8 kg CO2/year` mature
  tree benchmark. See [`tree-equivalent.md`](tree-equivalent.md).
- the same widget also includes an EV driving-energy equivalent using a
  stored premium-EV median consumption baseline derived from the bundled
  U.S.+Europe EV dataset. See [`ev-us-europe-database-report.md`](ev-us-europe-database-report.md).
- `Energy Impact` period behavior:
  - `Today so far` stays on the normal live/history refresh path,
  - `Past 12 months` is lazy-loaded only after the user clicks it,
  - once loaded, the 12-month value is cached client-side and is not treated as
    a realtime-updating metric.
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
  - detail routes are UUID-only (`/device/<uuid>`); the UI must not pass serial numbers in route or query parameters
  - browser-facing HTTP/2 is expected once TLS ingress is active
  - local browser-facing HTTP/3 is opt-in via ingress QUIC listener + UDP 443
    service exposure when `runtime.publicApp.ingress.http3.enabled=true`
  - local observability-lite is enabled by default, so Prometheus, Grafana,
    and the OpenTelemetry collector should come up with the standard
    `make platform-up` path
  - dev/GKE keeps HTTP/3 opt-in by default; leave
    `runtime.publicApp.ingress.http3.enabled=false` unless the environment is
    explicitly validating a real TLS/domain edge with UDP 443 exposed
  - practical verification still depends on a browser or `curl` build with
    HTTP/3 support
