# Reference: Commands

## Go Commands

```bash
go test ./...
make test-race
make test-race-stress
go run ./cmd/ecoflow-smoke
go run ./cmd/ecoflow-server
go run ./cmd/ecoflow-pv-fingerprint
go run ./cmd/ecoflow-panel-db-import
go run ./cmd/ecoflow-panel-csv-backfill
go run ./cmd/ecoflow-panel-select-train
go run ./cmd/ecoflow-db-migrate-job
go run ./cmd/ecoflow-grpc-api
go run ./cmd/ecoflow-dev-seed
go run ./cmd/ecoflow-ingest-worker
go run ./cmd/ecoflow-inference-worker
go run ./cmd/ecoflow-rollup-worker
go run ./cmd/ecoflow-projection-worker
go run ./cmd/ecoflow-archive-worker
go run ./cmd/ecoflow-replay-cli
go run ./cmd/ecoflow-gap-detector
go run ./cmd/ecoflow-gap-repair-worker
go run ./cmd/ecoflow-loadtest-ingest-bridge
make test-db-migrations-ci
make pgroll-init-local
make pgroll-status-local
make pgroll-start-local PGROLL_PLAN=deploy/db/pgroll/plans/<plan-file>
make pgroll-complete-local
make pgroll-rollback-local
```

Run the forward-only rollout migration runner directly (the same binary used by
the Helm/Argo hook job):

```bash
DB_MIGRATION_ENVIRONMENT='dev' \
DB_MIGRATION_DB_HOST='127.0.0.1' \
DB_MIGRATION_DB_PORT='15432' \
DB_MIGRATION_DB_NAME='pulse' \
DB_MIGRATION_DB_USER='pulse' \
DB_MIGRATION_DB_PASSWORD='pulse-local-dev-password' \
DB_MIGRATIONS_DIR='deploy/db/migrations' \
go run ./cmd/ecoflow-db-migrate-job
```

Prepare the local CNPG database for future `pgroll` transition work:

```bash
make pgroll-init-local
make pgroll-status-local
make pgroll-start-local PGROLL_PLAN=deploy/db/pgroll/plans/<plan-file>
make pgroll-complete-local
make pgroll-rollback-local
```

## Node/Expo Auth Commands

```bash
# Typecheck + test Node JWKS middleware package.
npm run typecheck --workspace @ecoflow-pulse/node-jwks-auth
npm run test --workspace @ecoflow-pulse/node-jwks-auth
npm run build --workspace @ecoflow-pulse/node-jwks-auth

# Pulse platform Node REST BFF.
npm run -w apps/pulse-platform typecheck
npm run -w apps/pulse-platform build
npm run -w apps/pulse-platform lint
npm run -w apps/pulse-platform test
npm run platform-bff

# Pulse realtime WebSocket gateway.
npm run -w apps/pulse-realtime-gateway typecheck
npm run -w apps/pulse-realtime-gateway build
npm run -w apps/pulse-realtime-gateway lint
npm run -w apps/pulse-realtime-gateway test
npm run realtime-gateway

# Universal app checks (includes PKCE auth card code paths).
npm run -w apps/universal typecheck
npm run -w apps/universal lint
npm run -w apps/universal test
npm run -w apps/universal e2e:web
MAESTRO_EXPO_URL='exp://127.0.0.1:8081' npm run -w apps/universal e2e:mobile
```

- The Node workspace `build` commands above emit runtime JS to `dist/` for
  container images; the `typecheck` commands remain no-emit validation paths.

Run E2E suites via repository make targets:

```bash
make test-web-e2e
MAESTRO_EXPO_URL='exp://127.0.0.1:8081' make test-mobile-e2e
make test-load-k6
make test-grpc-load-harness
make test-grpc-soak-10k
```

Run k6 load test coverage for ingest + websocket + query (M5 slice):

```bash
# prerequisites:
# - local stack up (`make dev-up`, `make db-seed-dev-local`)
# - k6 installed
make test-load-k6

# optional tuning overrides:
K6_DURATION=2m \
K6_INGEST_RATE=40 \
K6_QUERY_RATE=2 \
K6_WS_VUS=40 \
make test-load-k6
```

For mobile smoke with Maestro:

```bash
# one-time install
curl -Ls "https://get.maestro.mobile.dev" | bash
java -version

# then run Expo on a simulator/emulator and execute the smoke flow
npm run -w apps/universal ios
MAESTRO_EXPO_URL='exp://127.0.0.1:8081' make test-mobile-e2e
```

Notes:
- `MAESTRO_APP_ID` defaults to `host.exp.Exponent` (Expo Go). Override it for custom dev-build bundle IDs.
- `MAESTRO_EXPO_URL` defaults to `exp://127.0.0.1:8081` if unset.
- `make test-mobile-e2e` auto-starts local mock API (`127.0.0.1:18081`) and mock WS (`127.0.0.1:8082`) when those ports are not already listening.
- `make test-load-k6` opens a temporary NATS port-forward (`svc/pulse-platform-nats` -> `127.0.0.1:14222`) and runs a temporary local ingest bridge (`127.0.0.1:19090`) during the k6 run.
- `make test-load-k6` defaults are tuned to stay below the BFF history rate limiter (`PULSE_PLATFORM_HISTORY_RATE_LIMIT_MAX=120/min`) while still exercising ingest + WS + query paths.

Run the full local cluster-served web stack (default local workflow):

```bash
make platform-up
make platform-wait
make services-up
make services-wait
make db-seed-dev-local
```

Then open:

```bash
open http://localhost/devices
```

The public edge serves all three paths from the same origin:

```bash
curl -sS http://localhost/healthz
curl -sS http://localhost/api/devices | jq '.'
```

Standalone debug mode (optional, not the default local workflow):

Run the Node REST BFF against the local gRPC API:

```bash
GRPC_API_ADDR='127.0.0.1:9090' \
NODE_AUTH_MODE='noop' \
PULSE_PLATFORM_DEV_SUBJECT='dev-user@example.com' \
PULSE_PLATFORM_PORT='18081' \
npm run platform-bff
```

Run the realtime WebSocket gateway against the local authz API + live Valkey/NATS data plane:

```bash
# Port-forward the local Valkey Sentinel service first so clients can resolve the current primary.
kubectl -n pulse-platform port-forward svc/pulse-platform-valkey 26379:26379

GRPC_API_ADDR='127.0.0.1:9090' \
NATS_URLS='nats://127.0.0.1:4222' \
VALKEY_ADDRS='127.0.0.1:26379' \
VALKEY_SENTINEL_MASTER_SET='myprimary' \
PROJECTION_KEY_PREFIX='pulse:projection' \
TELEMETRY_SUBJECT_PREFIX='pulse.telemetry' \
NODE_AUTH_MODE='noop' \
PULSE_REALTIME_GATEWAY_PORT='8082' \
npm run realtime-gateway
```

Run the Expo universal app against the standalone debug stack:

```bash
EXPO_PUBLIC_API_URL='http://127.0.0.1:18081' \
EXPO_PUBLIC_WS_URL='ws://127.0.0.1:8082/ws' \
npm run -w apps/universal web -- --clear
```

Run the Expo universal app with local PKCE auth enabled (standalone debug mode):

```bash
EXPO_PUBLIC_API_URL='http://127.0.0.1:18081' \
EXPO_PUBLIC_WS_URL='ws://127.0.0.1:8082/ws' \
EXPO_PUBLIC_OIDC_ISSUER_URL='http://127.0.0.1:8084/realms/pulse' \
EXPO_PUBLIC_OIDC_CLIENT_ID='pulse-universal-app' \
EXPO_PUBLIC_OIDC_SCOPES='openid profile email offline_access' \
npm run -w apps/universal web -- --clear
```

Race detection commands (critical service paths):

```bash
# PR-equivalent race gate
make test-race

# Optional stress pass (defaults to 5 repetitions)
make test-race-stress

# Override stress count
make test-race-stress RACE_STRESS_COUNT=10
```

Run gRPC API with explicit control-plane Postgres store:

```bash
CONTROL_PLANE_DB_DSN='postgres://<user>:<pass>@<host>:5432/pulse?sslmode=disable' go run ./cmd/ecoflow-grpc-api
```

Run gRPC API with live snapshot reads from Valkey (used by `TelemetryService.GetSnapshot`):

```bash
CONTROL_PLANE_DB_DSN='postgres://<user>:<pass>@<host>:5432/pulse?sslmode=disable' \
VALKEY_ADDRS='127.0.0.1:26379' \
VALKEY_SENTINEL_MASTER_SET='myprimary' \
PROJECTION_KEY_PREFIX='pulse:projection' \
go run ./cmd/ecoflow-grpc-api
```

Run the opt-in real EcoFlow adapter integration check against seeded SNs:

```bash
ECOFLOW_ADAPTER_INTEGRATION=1 go test ./internal/provideradapter -run TestEcoFlowAdapterGetMQTTCertificationSeededSNsIntegration -count=1 -v
```

Run distributed ingest worker loop (poll active assignments, claim lease, start one session per provider device):

```bash
CONTROL_PLANE_DB_DSN='postgres://<user>:<pass>@<host>:5432/pulse?sslmode=disable' \
VALKEY_ADDRS='127.0.0.1:26379' \
VALKEY_SENTINEL_MASTER_SET='myprimary' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-ingest-worker
```

Run local load-test ingest bridge (accepts `POST /ingest` JSON and publishes `TelemetryEnvelope` frames to NATS ingest subjects):

```bash
NATS_URLS='nats://127.0.0.1:4222' \
TELEMETRY_SUBJECT_PREFIX='pulse' \
LOADTEST_INGEST_BIND_ADDR='127.0.0.1:19090' \
go run ./cmd/ecoflow-loadtest-ingest-bridge
```

Run projection worker loop (consume ingest envelopes from JetStream and build Valkey live snapshots):

```bash
VALKEY_ADDRS='127.0.0.1:26379' \
VALKEY_SENTINEL_MASTER_SET='myprimary' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-projection-worker
```

Run inference worker loop (consume ingest envelopes and build Valkey device insights):

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
VALKEY_ADDRS='127.0.0.1:26379' \
VALKEY_SENTINEL_MASTER_SET='myprimary' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-inference-worker
```

Run rollup worker loop (consume ingest envelopes from JetStream and upsert minute/hour/day Timescale rollups):

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-rollup-worker
```

Run archive worker loop (consume ingest envelopes from JetStream and write protobuf+zstd objects to MinIO-compatible storage):

```bash
NATS_URLS='nats://127.0.0.1:4222' \
ARCHIVE_OBJECT_ENDPOINT='127.0.0.1:9000' \
ARCHIVE_OBJECT_ACCESS_KEY='minio' \
ARCHIVE_OBJECT_SECRET_KEY='minio123' \
ARCHIVE_OBJECT_BUCKET='pulse-telemetry-raw' \
go run ./cmd/ecoflow-archive-worker
```

Run replay CLI modes (manifest-backed listing + device/fleet replay to NATS replay subjects):

```bash
# List known device/provider ids in the archive manifest window.
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
go run ./cmd/ecoflow-replay-cli -mode list-devices -from 2026-02-26T00:00:00Z -to 2026-02-26T23:59:59Z

# Per-device replay (dry-run decode/filter only; no NATS publish).
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
ARCHIVE_OBJECT_ENDPOINT='127.0.0.1:9000' \
ARCHIVE_OBJECT_ACCESS_KEY='minio' \
ARCHIVE_OBJECT_SECRET_KEY='minio123' \
go run ./cmd/ecoflow-replay-cli -mode device -provider-device-ids DEMOD2M00001057 -from 2026-02-26T08:00:00Z -to 2026-02-26T09:00:00Z -dry-run

# Fleet shard/time replay (publishes TelemetryEnvelope bytes to pulse.telemetry.replay.sNNN).
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
ARCHIVE_OBJECT_ENDPOINT='127.0.0.1:9000' \
ARCHIVE_OBJECT_ACCESS_KEY='minio' \
ARCHIVE_OBJECT_SECRET_KEY='minio123' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-replay-cli -mode fleet -shards 7,11 -from 2026-02-26T08:00:00Z -to 2026-02-26T09:00:00Z

# Device replay backfill directly into ingest subjects (for rollup/projection rebuilds).
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
ARCHIVE_OBJECT_ENDPOINT='127.0.0.1:9000' \
ARCHIVE_OBJECT_ACCESS_KEY='minio' \
ARCHIVE_OBJECT_SECRET_KEY='minio123' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-replay-cli -mode device -provider ecoflow -provider-device-ids DEMOD2M00001057 -from 2026-02-25T00:00:00Z -to 2026-02-26T00:00:00Z -nats-target ingest

# Rebuild rollups directly from local raw MQTT log capture instead of archive objects.
# This is the bounded backfill path for explicit energy bucket columns on
# historical windows and uses canonical quota/MPPT updates plus the same
# interval-based energy integration as the live rollup worker.
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
go run ./cmd/ecoflow-rollup-rebuild \
  -provider ecoflow \
  -provider-device-ids DEMODPU0000294 \
  -from 2026-03-07T05:00:00Z \
  -to 2026-03-07T18:00:00Z \
  -raw-logs logs/mqtt_payload_raw-2026-03-07.log
```

Run gap detector loop (projection lag detection + targeted replay enqueue):

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
VALKEY_ADDRS='127.0.0.1:26379' \
VALKEY_SENTINEL_MASTER_SET='myprimary' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-gap-detector
```

Run gap-repair worker loop (consume queue jobs and replay back to ingest subjects):

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
ARCHIVE_OBJECT_ENDPOINT='127.0.0.1:9000' \
ARCHIVE_OBJECT_ACCESS_KEY='minio' \
ARCHIVE_OBJECT_SECRET_KEY='minio123' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-gap-repair-worker
```

Ingest worker scaling knobs:

```bash
# startup/reconcile worker pool (default: clamp(4*GOMAXPROCS, 8, 64))
export INGEST_START_WORKERS=32

# bounded startup queue (default: INGEST_START_WORKERS*8)
export INGEST_START_QUEUE_SIZE=256
```

Common structured logging knobs (all service/worker binaries):

```bash
export LOG_LEVEL=info
export LOG_ASYNC_DISABLED=false
export LOG_ASYNC_QUEUE_SIZE=8192
export LOG_ASYNC_BYPASS_LEVEL=warn
export LOG_METRICS_INTERVAL=30s
```

Ingest worker MQTT session + telemetry bus knobs:

```bash
# MQTT session defaults (per leased provider device)
export INGEST_MQTT_KEEPALIVE=90s
export INGEST_MQTT_CONNECT_TIMEOUT=10s
export INGEST_MQTT_READ_TIMEOUT=45s
export INGEST_MQTT_WRITE_TIMEOUT=15s
export INGEST_MQTT_RECONNECT_INITIAL_BACKOFF=500ms
export INGEST_MQTT_RECONNECT_MAX_BACKOFF=15s
export INGEST_MQTT_RECONNECT_JITTER=0.25

# EOF reconnect-rate spike alerting (read mqtt message: EOF)
export INGEST_MQTT_RECONNECT_ALERT_WINDOW=5m
export INGEST_MQTT_RECONNECT_ALERT_THRESHOLD=8
export INGEST_MQTT_RECONNECT_ALERT_COOLDOWN=2m

# MQTT auth-reject spike alerting (broker connect rejected)
export INGEST_MQTT_AUTH_ALERT_WINDOW=10m
export INGEST_MQTT_AUTH_ALERT_THRESHOLD=5
export INGEST_MQTT_AUTH_ALERT_COOLDOWN=5m

# Optional sampled payload logging (debug-only, off by default)
export INGEST_MQTT_LOG_PAYLOAD_DEBUG=false
export INGEST_MQTT_LOG_PAYLOAD_SAMPLE_EVERY=100

# Quota bootstrap + refresh (same ingest pipeline, source=quota)
export INGEST_QUOTA_FETCH_TIMEOUT=8s
export INGEST_QUOTA_REFRESH_INTERVAL=30s
export INGEST_QUOTA_REFRESH_JITTER=0.20

# Lease-loss spike alerting (heartbeat renew rejected: missing)
export INGEST_LEASE_MISSING_ALERT_WINDOW=5m
export INGEST_LEASE_MISSING_ALERT_THRESHOLD=4
export INGEST_LEASE_MISSING_ALERT_COOLDOWN=2m

# Bounded async publish queue (per MQTT session)
export INGEST_PUBLISH_QUEUE_SIZE=256
export INGEST_PUBLISH_WORKERS=1
export INGEST_PUBLISH_ENQUEUE_TIMEOUT=2s

# Safety guard: workers > 1 can reorder per-session envelope publish
export INGEST_ALLOW_UNORDERED_PUBLISH=false

# Throughput knob: drop optional map labels before protobuf marshal/publish
export INGEST_DISABLE_ENVELOPE_LABELS=false

# JetStream ingest stream bootstrap (idempotent on worker start)
export INGEST_NATS_JS_BOOTSTRAP_ENABLED=true
export INGEST_NATS_JS_STREAM_NAME='PULSE_TELEMETRY_INGEST'
export INGEST_NATS_JS_REPLICAS=3
export INGEST_NATS_JS_MAX_AGE=72h
export INGEST_NATS_JS_MAX_BYTES=0

# Envelope publish policy (retry/backpressure tuning)
export INGEST_NATS_USE_JETSTREAM=true
export INGEST_NATS_PUBLISH_TIMEOUT=3s
export INGEST_NATS_PUBLISH_MAX_RETRIES=3
export INGEST_NATS_PUBLISH_RETRY_INITIAL_BACKOFF=50ms
export INGEST_NATS_PUBLISH_RETRY_MAX_BACKOFF=500ms
export INGEST_NATS_PUBLISH_RETRY_JITTER=0.20

# NATS connection defaults for envelope publishing
export NATS_URLS='nats://127.0.0.1:4222'
export NATS_NAME='ecoflow-ingest-worker'
export NATS_CONNECT_TIMEOUT=5s
export NATS_RECONNECT_WAIT=2s
export NATS_RECONNECT_JITTER=2s
export NATS_PING_INTERVAL=20s
export NATS_MAX_PINGS_OUT=3
export NATS_MAX_RECONNECTS=-1

# Replay CLI publish target (default replay subjects; set ingest for backfill)
export REPLAY_NATS_TARGET='replay' # replay|ingest

# Telemetry subject routing / deterministic shard mapping
export TELEMETRY_SUBJECT_PREFIX='pulse'
export TELEMETRY_SHARD_COUNT=128

# Projection worker consumer + snapshot store knobs
export PROJECTION_KEY_PREFIX='pulse:projection'
export PROJECTION_INGEST_STREAM_NAME='PULSE_TELEMETRY_INGEST'
export PROJECTION_CONSUMER_DURABLE='projection-live-v1'
export PROJECTION_QUEUE_GROUP='projection-live'
export PROJECTION_ACK_WAIT=30s
export PROJECTION_MAX_ACK_PENDING=4096
export PROJECTION_PROCESS_TIMEOUT=3s
export PROJECTION_DRAIN_TIMEOUT=8s

# Rollup worker consumer + Timescale upsert knobs
export ROLLUP_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable'
export ROLLUP_INGEST_STREAM_NAME='PULSE_TELEMETRY_INGEST'
export ROLLUP_CONSUMER_DURABLE='rollup-timeseries-v1'
export ROLLUP_QUEUE_GROUP='rollup-timeseries'
export ROLLUP_ACK_WAIT=30s
export ROLLUP_MAX_ACK_PENDING=4096
export ROLLUP_PROCESS_TIMEOUT=3s
export ROLLUP_DRAIN_TIMEOUT=8s

# Archive worker consumer + object writer knobs
export ARCHIVE_INGEST_STREAM_NAME='PULSE_TELEMETRY_INGEST'
export ARCHIVE_CONSUMER_DURABLE='archive-raw-v1'
export ARCHIVE_QUEUE_GROUP='archive-raw'
export ARCHIVE_ACK_WAIT=60s
export ARCHIVE_MAX_ACK_PENDING=4096
export ARCHIVE_PROCESS_TIMEOUT=3s
export ARCHIVE_DRAIN_TIMEOUT=8s
export ARCHIVE_FAILURE_ALERT_WINDOW=10m
export ARCHIVE_FAILURE_ALERT_THRESHOLD=6
export ARCHIVE_FAILURE_ALERT_COOLDOWN=5m
export ARCHIVE_FLUSH_INTERVAL=30s
export ARCHIVE_FLUSH_TIMEOUT=12s
export ARCHIVE_MAX_RECORDS_PER_PART=1024
export ARCHIVE_MAX_BYTES_PER_PART=4194304
export ARCHIVE_ZSTD_LEVEL=3
export ARCHIVE_OBJECT_BUCKET='pulse-telemetry-raw'
export ARCHIVE_OBJECT_PREFIX='raw'
export ARCHIVE_WRITER_ID='archive-worker-1'

# MinIO/S3-compatible object store connection
export ARCHIVE_OBJECT_ENDPOINT='127.0.0.1:9000'
export ARCHIVE_OBJECT_ACCESS_KEY='minio'
export ARCHIVE_OBJECT_SECRET_KEY='minio123'
export ARCHIVE_OBJECT_REGION='us-east-1'
export ARCHIVE_OBJECT_SECURE=false
export ARCHIVE_OBJECT_AUTO_CREATE_BUCKET=true

# Optional manifest index persistence (falls back to CONTROL_PLANE_DB_DSN when unset)
export ARCHIVE_MANIFEST_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable'

# Gap detector knobs
export GAP_REPAIR_PROVIDER='ecoflow'                    # optional
export GAP_REPAIR_POLL_INTERVAL=30s
export GAP_REPAIR_POLL_JITTER=0.20
export GAP_REPAIR_LOOKBACK_WINDOW=30m
export GAP_REPAIR_LAG_THRESHOLD=90s
export GAP_REPAIR_LAG_ALERT_WINDOW=15m
export GAP_REPAIR_LAG_ALERT_THRESHOLD=12
export GAP_REPAIR_LAG_ALERT_COOLDOWN=5m
export GAP_REPAIR_WINDOW_PADDING=30s
export GAP_REPAIR_MAX_REPLAY_WINDOW=30m
export GAP_REPAIR_SAFE_DELAY=10s
export GAP_REPAIR_MAX_OBJECTS_PER_JOB=0
export GAP_REPAIR_MAX_JOBS_PER_CYCLE=64
export GAP_REPAIR_EVAL_WORKERS=16
export GAP_REPAIR_DRY_RUN=false
export GAP_REPAIR_NATS_USE_JETSTREAM=true
export GAP_REPAIR_MSG_ID_BUCKET=1m

# Gap-repair queue stream bootstrap
export GAP_REPAIR_NATS_JS_BOOTSTRAP_ENABLED=true
export GAP_REPAIR_NATS_JS_STREAM_NAME='PULSE_TELEMETRY_GAPREPAIR'
export GAP_REPAIR_NATS_JS_REPLICAS=3
export GAP_REPAIR_NATS_JS_MAX_AGE=24h
export GAP_REPAIR_NATS_JS_MAX_BYTES=0

# Gap-repair worker consumer knobs
export GAP_REPAIR_STREAM_NAME='PULSE_TELEMETRY_GAPREPAIR'
export GAP_REPAIR_CONSUMER_DURABLE='gap-repair-v1'
export GAP_REPAIR_QUEUE_GROUP='gap-repair-workers'
export GAP_REPAIR_ACK_WAIT=2m
export GAP_REPAIR_MAX_ACK_PENDING=1024
export GAP_REPAIR_PROCESS_TIMEOUT=2m
export GAP_REPAIR_DRAIN_TIMEOUT=10s
export GAP_REPAIR_DEFAULT_MAX_OBJECTS=0
export GAP_REPAIR_REPLAY_FAILURE_ALERT_WINDOW=10m
export GAP_REPAIR_REPLAY_FAILURE_ALERT_THRESHOLD=6
export GAP_REPAIR_REPLAY_FAILURE_ALERT_COOLDOWN=5m
```

## Protobuf / gRPC Generation

```bash
buf generate
buf lint
```

## gRPC Profiling / Benchmarks

```bash
# Stable local make wrappers
make test-grpc-load-harness
make test-grpc-soak-10k

# Baseline grpc benchmarks (unary path)
go test ./cmd/ecoflow-grpc-api -run '^$' -bench 'BenchmarkTelemetry(GetSnapshotParallel|GetSnapshot)$' -benchmem -benchtime=5s

# Workload-calibrated benchmarks (derived from mqtt payload logs)
go test ./cmd/ecoflow-grpc-api -run '^$' -bench 'BenchmarkTelemetry(GetSnapshotObservedFleetMix|SubscribeObservedBurst)$' -benchmem -benchtime=3s
go test ./cmd/ecoflow-grpc-api -run '^$' -bench 'BenchmarkTelemetry(GetSnapshotObservedFleetMix10k|SubscribeObservedStartupSpike10k)$' -benchmem -benchtime=2s

# Compare GC settings
GOGC=200 go test ./cmd/ecoflow-grpc-api -run '^$' -bench 'BenchmarkTelemetry(GetSnapshotObservedFleetMix|SubscribeObservedBurst)$' -benchmem -benchtime=3s
GOGC=400 go test ./cmd/ecoflow-grpc-api -run '^$' -bench 'BenchmarkTelemetry(GetSnapshotObservedFleetMix|SubscribeObservedBurst)$' -benchmem -benchtime=3s
GOGC=200 GOMEMLIMIT=128MiB go test ./cmd/ecoflow-grpc-api -run '^$' -bench 'BenchmarkTelemetry(GetSnapshotObservedFleetMix|SubscribeObservedBurst)$' -benchmem -benchtime=3s

# Opt-in 10k-device p99 + heap guard soak test
ECOFLOW_GRPC_10K_SOAK=1 GOGC=200 GOMEMLIMIT=128MiB go test ./cmd/ecoflow-grpc-api -run TestTelemetryServerP99LatencyAndHeapStable10k -count=1 -v
# Optional threshold overrides (milliseconds / MiB):
# ECOFLOW_GRPC_P99_STEADY_MAX_MS=50 ECOFLOW_GRPC_P99_BURST_MAX_MS=250 ECOFLOW_GRPC_MAX_HEAP_DELTA_MB=64

# race detection for streaming/snapshot concurrency paths
go test -race ./cmd/ecoflow-grpc-api -count=1
go test -race ./internal/grpcserver -count=1

# CPU/heap profiles
go test ./cmd/ecoflow-grpc-api -run '^$' -bench BenchmarkTelemetryGetSnapshotParallel -benchtime=10s -cpuprofile /tmp/ecoflow-grpc-api.cpu.out
go tool pprof -top /tmp/ecoflow-grpc-api.cpu.out
go test ./cmd/ecoflow-grpc-api -run TestTelemetryServerHeapStableUnderSnapshotLoad -count=1 -memprofile /tmp/ecoflow-grpc-api.mem.out
go tool pprof -top /tmp/ecoflow-grpc-api.mem.out
```

## Ingest Worker Profiling / Benchmarks

```bash
# worker assignment loop throughput (5k/10k synthetic assignments)
go test ./internal/ingestworker -run '^$' -bench BenchmarkLoopReconcileStartBatch -benchmem -count=1

# race detection for lease/session lifecycle and concurrency edges
go test -race ./internal/ingestworker -count=1

# focused shutdown/queue race checks for async publish and loop drain behavior
go test ./internal/ingestworker -run 'TestAsyncEnvelopePublisher|TestProviderSessionRunner|TestLoopNoGoroutineLeakOnShutdown|TestLoopCancelDuringBurstStart' -count=1

# focused cpu/heap profile (10k, high-concurrency startup path)
go test ./internal/ingestworker -run '^$' -bench BenchmarkLoopReconcileStartBatch/10k_workers48_delay50us -benchtime=3x -cpuprofile /tmp/ingestworker.cpu.out -memprofile /tmp/ingestworker.mem.out
go tool pprof -top /tmp/ingestworker.cpu.out
go tool pprof -top -alloc_space /tmp/ingestworker.mem.out
```

## Worker Hot-Path Benchmarks

```bash
# projection snapshot apply hot path
go test ./internal/projectionworker -run '^$' -bench BenchmarkValkeySnapshotStoreApplyEnvelope -benchmem -count=1

# inference read-model apply hot path
go test ./internal/inference -run '^$' -bench BenchmarkValkeyStoreApplyEnvelope -benchmem -count=1

# rollup worker delivery apply path
go test ./internal/rollupworker -run '^$' -bench BenchmarkProcessDeliverySuccess -benchmem -count=1

# gap-repair worker request handling path
go test ./internal/gaprepair -run '^$' -bench BenchmarkWorkerHandleDeliverySuccess -benchmem -count=1
```

## Command Regression Tests

```bash
# all Go programs under cmd/ have regression coverage
go test ./cmd/...

# focused bootstrap/config regression suites for worker-style entrypoints
go test ./cmd/ecoflow-archive-worker ./cmd/ecoflow-db-migrate-job ./cmd/ecoflow-gap-detector ./cmd/ecoflow-gap-repair-worker ./cmd/ecoflow-inference-worker ./cmd/ecoflow-ingest-worker ./cmd/ecoflow-projection-worker ./cmd/ecoflow-rollup-worker -count=1
```

## Helper Scripts

```bash
./scripts/regenerate_solar_panel_db.sh
./scripts/train_panel_select_model.sh
```

Optional link map override for panel DB regeneration:

```bash
SOLAR_PANEL_LINK_MAP=data/solar_panels/panel_purchase_links_v13.json ./scripts/regenerate_solar_panel_db.sh
```

## Make Targets

```bash
make lint
make test
make bench
make bench-ingestlease-integration
make test-archive-integration
make test-web-e2e
make test-mobile-e2e
make test-load-k6
make build
make smoke
make ingest-worker
make inference-worker
make projection-worker
make archive-worker
make replay-cli
make gap-detector
make gap-repair-worker
make services-image-build-local
make services-image-import-local
make services-image-local-up
make k3d-up
make platform-up
make platform-wait
make services-up
make services-wait
make dev-up
make dev-down
make db-migrate-up-local
make db-migrate-down-local
make db-migrate-verify-local
make db-migrate-cycle-local
make db-migrate-e2e-local
make db-seed-dev-local
make dr-backup-local
make dr-restore-local
make dr-drill-local
make auth-keycloak-verify-local
make gke-context
make gke-dev-guardrails
make gke-park
make gke-wake
make scale-down
make scale-up
make argocd-bootstrap-dev
make argocd-apps-dev
make argocd-wait-apps
make argocd-dev-up
make web-stop
make web
```

For required local tooling (for example `helm`, `k3d`, `kubectl`), see:
`docs/reference/local-dev-prerequisites.md`.

Notes:

- default `GOFLAGS` in `Makefile` include `-tags=moderncompress -mod=mod`,
- `make lint` now runs:
  - `go fmt ./...`
  - `golangci-lint run ./...` (or `go vet ./...` fallback)
  - `buf lint`
  - `actionlint`
  - `markdownlint` over tracked `*.md` files using `.markdownlint.json`
  - if `buf` is missing, it fails with install hint:
    `https://buf.build/docs/installation/`
  - if `actionlint` is missing, it fails with install hint:
    `brew install actionlint`
  - if `markdownlint` is missing, it fails with install hint:
    `brew install markdownlint-cli`
- `make bench-ingestlease-integration` runs Valkey lease manager integration
  leak/throughput checks against a live Valkey service via temporary
  `kubectl port-forward`.
- `make test-archive-integration` runs `internal/archiveworker` integration
  tests (real MinIO object writes + failure injection):
  - opens temporary `kubectl port-forward` to
    `svc/pulse-platform-minio:9000`,
  - resolves MinIO credentials from
    `secret/pulse-platform-minio` (`rootUser` / `rootPassword`) unless
    overridden via `ARCHIVE_OBJECT_ACCESS_KEY` / `ARCHIVE_OBJECT_SECRET_KEY`,
  - sets `ARCHIVE_STORE_INTEGRATION=1` and runs
    `go test ./internal/archiveworker -tags integration`.
- `make test-pipeline-integration` runs the end-to-end telemetry pipeline
  integration suite with Testcontainers:
  - starts isolated Postgres/Timescale, NATS, Valkey, and MinIO containers,
  - applies `deploy/db/migrations/*.up.sql`,
  - runs a full data-path assertion (`NATS ingest -> projection snapshot ->
    rollup row -> archive manifest/object`),
  - requires a running Docker daemon,
  - command:
    `PIPELINE_INTEGRATION=1 go test ./internal/pipelineintegration -tags integration -count=1 -v`.
- `make test-proto-contract` runs Node↔Go protobuf contract tests for realtime
  envelope compatibility:
  - generates canonical envelope fixture bytes from Go generated protobuf types,
  - verifies Node gateway decoder can parse Go-generated wire bytes correctly,
  - verifies Node protobufjs-generated wire bytes decode correctly in Go,
  - requires both `go` and `npm` toolchains available.
- `make test-db-migrations-ci` runs the migration validation gate with
  Testcontainers:
  - starts an isolated PostgreSQL 18 + TimescaleDB container,
  - applies all `deploy/db/migrations/*.up.sql`,
  - verifies expected schema tables, `uuidv7()` defaults, UTC timestamp
    ownership, hypertables, and retention policies,
  - applies all down migrations in reverse order, then reapplies all up
    migrations,
  - runs end-to-end ownership, uniqueness, and role guard checks,
  - runs a `pgroll` smoke `init -> start -> rollback` cycle when `pgroll` is
    installed,
  - validates repo `deploy/db/pgroll/plans/*.{json,yaml,yml}` plans
    sequentially (`start -> complete`) when they exist,
  - requires a running Docker daemon.
  This is the CI-aligned local gate that should stay green before enabling
  environment rollout automation for schema changes.
- `make web` restarts Expo web by first stopping any process listening on
  `WEB_PORT` (default `8081`), then running:
  `npm run -w apps/universal web -- --port $(WEB_PORT) --clear`.
- `make replay-cli` runs the replay command with optional passthrough args:
  `make replay-cli ARGS='-mode list-devices -from 2026-02-26T00:00:00Z -to 2026-02-26T23:59:59Z'`.
- `buf generate` regenerates protobuf/gRPC Go stubs into `gen/` from
  `proto/` using `buf.yaml` + `buf.gen.yaml`.
- `make k3d-up` creates or reuses local k3d cluster from `deploy/tilt/k3d-config.yaml`.
  Requires `k3d`, `kubectl`, and a running Docker daemon.
  It also switches `kubectl` context to `k3d-pulse-local`.
- `make platform-up` updates Helm deps and installs/upgrades `pulse-platform` using `deploy/env/local/values.platform.yaml`.
  Local Helm/Kubernetes operations are pinned to `k3d-pulse-local` (`--kube-context` / `--context`) so local targets cannot accidentally apply to GKE.
  Local Helm upgrades use server-side apply with `--force-conflicts` so
  k3d redeploys can reclaim fields that were temporarily modified by manual
  commands such as `kubectl set image`.
  Local default public-runtime replica counts are:
  - `pulse-platform-public-app`: `2`
  - `pulse-platform-realtime-gateway`: `2`
  MinIO credentials must be configured with MinIO-chart top-level keys
  (`minio.rootUser` / `minio.rootPassword`), not `minio.auth.*`, to keep
  `secret/pulse-platform-minio` in sync with services runtime credentials.
  It includes retry/backoff for transient CNPG webhook race conditions during initial install.
  It runs a CRD-safe reconcile flow for CloudNativePG:
  1) initial Helm install/upgrade,
  2) wait for CNPG operator deployment readiness,
  3) when local TLS ingress is enabled, wait for cert-manager
     controller/webhook/cainjector readiness,
  4) second Helm pass to apply CRD-backed resources (for example `Cluster`,
     `ClusterIssuer`).
  On a fresh local bootstrap when `pulse-platform-keycloak` does not exist yet,
  the first Helm pass now defers Keycloak entirely, waits for:
  - CNPG cluster `pulse-platform-core` `Ready`,
  - secret `pulse-platform-core-app`,
  - `pulse-platform-core-rw` service endpoints,
  then applies the full Keycloak-enabled release on the second pass. This
  reduces first-boot Keycloak restart churn while the external CNPG database and
  bootstrap credentials settle.
  Connection + bootstrap contract:
  - bootstrap app credentials are configured in `cloudnativepgCluster.bootstrap.*` and rendered to secret `pulse-platform-core-app`,
  - service-facing contract is exposed via configmap `pulse-platform-core-contract`,
  - DSN-style connection secret is exposed via `pulse-platform-core-connection`,
  - local Keycloak is configured to use CNPG (`keycloak.postgresql.enabled=false`, `keycloak.externalDatabase.*` -> `pulse-platform-core-rw`).
  Validation examples:
  - `kubectl get ingress -n pulse-platform`
  - `kubectl describe ingress pulse-platform-public-edge -n pulse-platform`
  - `kubectl get sts -n pulse-platform`
  - `kubectl get pods -n pulse-platform`
  - `kubectl get deploy -n pulse-platform`
  - `kubectl get nodes -o wide`
  - `kubectl get clusters.postgresql.cnpg.io -n pulse-platform`
  - `kubectl get configmap pulse-platform-core-contract -n pulse-platform -o yaml`
  - `kubectl get secret pulse-platform-core-connection -n pulse-platform -o jsonpath='{.data.url}' | base64 -d; echo`
  - `kubectl get cluster pulse-platform-core -n pulse-platform -o yaml | rg "imageName|shared_preload_libraries|postInitApplicationSQL"`
  - `CNPG_POD=$(kubectl get pod -n pulse-platform -l cnpg.io/cluster=pulse-platform-core -o jsonpath='{.items[0].metadata.name}')`
  - `kubectl exec -n pulse-platform "${CNPG_POD}" -- psql -U postgres -d pulse -tAc "SHOW shared_preload_libraries;"`
  - `kubectl exec -n pulse-platform "${CNPG_POD}" -- psql -U postgres -d pulse -tAc "SELECT extname FROM pg_extension WHERE extname='timescaledb';"`
  - `kubectl get pod pulse-platform-valkey-node-0 -n pulse-platform -o jsonpath='{.spec.containers[*].name}'`
  Browser edge examples:
  - `curl -kI https://localhost`
  - `curl -kI https://localhost/_expo/static/js/web/$(ls apps/universal/dist/_expo/static/js/web | head -n 1)`
  - `kubectl get svc pulse-platform-public-edge-http3 -n pulse-platform`
  - verify HTTP/3 in a browser network inspector, or with a curl build that
    supports `--http3`
- `make edge-verify-http3-local` verifies the localhost browser edge using an
  HTTP/3-capable `curl` build:
  - checks the current `curl -V` `Features:` line for `HTTP3`
  - verifies `service/pulse-platform-public-edge-http3` exists in
    `pulse-platform`
  - verifies `Alt-Svc: h3=...` is advertised on `https://localhost`
  - verifies `curl --http3-only` negotiates HTTP version `3`
  - optional override: `HTTP3_VERIFY_URL=https://localhost make edge-verify-http3-local`
- `make local-trust-platform-tls` (macOS only) exports the current
  `pulse-platform-local-ca` certificate authority from k3d and adds it to the
  login keychain trust store so `curl https://localhost` and the browser can
  trust the local TLS endpoint without `-k`.
- `make local-trust-platform-tls-system` (macOS only) installs the same local
  CA into the macOS System keychain using `sudo`. Use this when Chrome still
  rejects `https://localhost` after the login-keychain trust step.
- `make platform-up` now runs that trust step automatically on macOS when:
  - `LOCAL_PLATFORM_AUTO_TRUST_TLS=1` (default), and
  - `secret/pulse-platform-local-tls` exists after the platform reconcile.
  Recovery examples after Docker restart:
  - `docker restart k3d-pulse-local-agent-0`
  - `kubectl get nodes -o wide`
  - `kubectl get pods -n pulse-platform`
- `make services-up` updates Helm deps and installs/upgrades `pulse-services` using `deploy/env/local/values.services.yaml`.
  Like `platform-up`, local Helm upgrades use server-side apply with
  `--force-conflicts` to recover cleanly from manual field-manager drift during
  local iteration.
  Because `pulse-services` has no external chart dependencies, local runs skip
  `helm dependency build`.
  Local default API replica count is:
  - `pulse-services-go-grpc-api`: `3`
  - `pulse-services-go-energy-api`: `3`
- `make services-image-build-local` builds local telemetry worker image
  `$(SERVICES_IMAGE_REPO):$(SERVICES_IMAGE_TAG)` from
  `deploy/docker/pulse-services.Dockerfile`.
  Repeated local builds reuse Docker BuildKit Go module and Go build caches so
  unchanged worker dependencies and object files do not recompile from scratch.
- `make services-image-import-local` imports that local worker image into
  k3d cluster `$(K3D_CLUSTER_NAME)`.
- `make services-image-local-up` runs build + import for local k3d in one step.
- `make public-images-build-local` builds the local public Node images
  `$(PLATFORM_APP_IMAGE_REPO):$(PLATFORM_APP_IMAGE_TAG)` and
  `$(REALTIME_GATEWAY_IMAGE_REPO):$(REALTIME_GATEWAY_IMAGE_TAG)` from
  `deploy/docker/pulse-platform.Dockerfile` and
  `deploy/docker/pulse-realtime-gateway.Dockerfile`.
  The two image builds run in parallel. Repeated local builds reuse Docker
  BuildKit NPM, Expo, and Metro cache mounts, so `npm ci` and Expo web export
  reruns can use previously downloaded packages and bundler state.
- `make public-images-import-local` imports those local public images into k3d
  cluster `$(K3D_CLUSTER_NAME)`.
  It imports both images in one `k3d image import` call to avoid repeated tools
  container startup during local redeploys.
- `make public-images-local-up` runs build + import for both public images in
  one step.
- `make dev-web-deploy` rebuilds/imports the local public app + realtime
  gateway images into k3d, re-applies the `pulse-platform` Helm release only
  when needed (`DEV_DEPLOY_HELM=auto|always|never`), waits for platform
  readiness, then rollout-restarts:
  - `pulse-platform-public-app`
  - `pulse-platform-realtime-gateway`
  Use this when you want to push just the local web/public edge changes into
  the k3d stack without touching `pulse-services`.
- WebSocket note for local multi-replica testing:
  - Kubernetes balances websocket traffic when the connection is established,
    not on every message.
  - To validate multi-pod realtime behavior locally, use multiple concurrent
    clients or repeated reconnects rather than expecting one live socket to hop
    between gateway pods.
- `make services-up` updates Helm deps and installs/upgrades `pulse-services`
  using `deploy/env/local/values.services.yaml`. By default it auto-builds and
  imports the local worker image before Helm apply; set
  `SERVICES_AUTO_BUILD_IMAGE=0` to skip.
- `make platform-up` reuses vendored chart packages under
  `deploy/charts/pulse-platform/charts` and only runs
  `helm dependency build --skip-refresh` when `Chart.yaml` / `Chart.lock`
  changed locally or vendored chart tarballs are missing.
- `make platform-wait` blocks until critical platform dependencies are ready:
  - CNPG operator deployment,
  - CNPG cluster `pulse-platform-core` `Ready` condition,
  - `nats`, `valkey-node`, and `keycloak` statefulsets,
  - service endpoints for `pulse-platform-core-rw`, `pulse-platform-nats`,
    `pulse-platform-valkey`, `pulse-platform-minio`, and `pulse-platform-keycloak-headless` when present,
  - optional Keycloak bootstrap job `pulse-platform-keycloak-keycloak-config-cli` reaching `Complete`,
  - `minio` deployment,
  - optional `ingress-nginx` controller deployment,
  - optional `cert-manager` controller/webhook/cainjector deployments,
  - optional External Secrets deployments (`external-secrets`, webhook, cert-controller),
  - local-by-default observability-lite deployments (`kube-promet-operator`, `grafana`, `opentelemetry-collector`).
- `make services-wait` blocks until `pulse-services` pods are `Ready` (if services workloads exist).
- local observability access examples after `make platform-up` / `make platform-wait`:
  - `kubectl -n pulse-platform port-forward svc/pulse-platform-kube-promet-prometheus 9090:9090`
  - `kubectl -n pulse-platform port-forward svc/pulse-platform-grafana 3000:80`
  - worker metrics endpoints exposed in local `pulse-services`:
    - `pulse-services-go-ingest-metrics`
    - `pulse-services-go-inference-metrics`
    - `pulse-services-go-projection-metrics`
    - `pulse-services-go-rollup-metrics`
    - `pulse-services-go-archive-metrics`
  - bundled Grafana dashboards:
    - `Pulse Pipeline Overview`
    - `Pulse Ingest Health`
    - `Pulse Storage & History Pipeline`
    - `Pulse gRPC SLOs`
    - `Pulse Platform Infra`
- `make dev-up` runs `k3d-up`, `platform-up`, `platform-wait`, `services-up`, then `services-wait`.
  This enforces startup order and returns only when dependencies are actually ready.
- `make dev-deploy` now reuses `make dev-web-deploy` for the public/web rollout
  path, then updates only the selected `pulse-services` deployments.
- `make services-up` upgrades the `pulse-services` Helm release and, when
  `SERVICES_AUTO_BUILD_IMAGE=1` (the default local path), also restarts the
  `pulse-services` deployments so the freshly imported `ecoflow-pulse/services:local`
  image is picked up even though the tag stays constant.
  Before applying the release it now also waits for the platform dependency
  endpoints consumed by the services layer: CNPG rw, NATS, Valkey, and MinIO.
- `make dev-deploy` is the incremental local redeploy path for code changes:
  rebuild/import local public + services images, then restart
  `pulse-platform-public-app`, `pulse-platform-realtime-gateway`, and
  `pulse-services-go-inference`, `pulse-services-go-grpc-api`,
  `pulse-services-go-energy-api`, and `pulse-services-go-rollup`, then wait for
  those rollouts to finish.
  By default (`DEV_DEPLOY_HELM=auto`) it skips Helm re-apply when local
  platform/services chart and local values files are unchanged, the releases
  already exist, and the expected restart-target deployments are already
  present. If an expected deployment such as `pulse-services-go-inference` or
  `pulse-services-go-grpc-api` / `pulse-services-go-energy-api` is missing,
  `auto` now forces the relevant Helm re-apply before restart.
  When Helm apply is needed, local chart dependency preparation stays local:
  vendored platform dependencies are reused and any rebuild uses
  `helm dependency build --skip-refresh` instead of refreshing remote repos.
  Use `DEV_DEPLOY_HELM=always make dev-deploy` to force full Helm re-apply, or
  `DEV_DEPLOY_HELM=never make dev-deploy` to skip it explicitly.
- `make dev-regen-data` rebuilds the last 48 hours of archived telemetry for all
  devices into rollup tables on local k3d using a direct archive-to-rollup
  rebuild path.
  It does not delete the requested range first; rebuilt rows are written back in
  bounded transactional chunks so charts do not go empty during regeneration.
  The rebuild now repopulates explicit energy bucket columns alongside the
  existing power/SOC/temp fields, so it is the supported local/dev backfill path
  for ADR-0018 historical coverage after migration.
  The rebuild command now also logs:
  - archive footprint for the matched manifest window (`objects`, `total_bytes`,
    `total_records`, `provider_devices`, `window_ts_*`),
  - decoded quota-frame count inside the matched archive set,
  - pre/post minute-bucket diffs per provider device
    (`pre_total_buckets`, `post_total_buckets`, `bucket_delta`,
    `pre_current_wh`, `post_current_wh`, `current_wh_delta`).
  It port-forwards CNPG and MinIO automatically, then prints proof from
  `telemetry_rollup_minute` in this format:
  `provider_device_id|touched_buckets|total_buckets|latest_bucket_utc|current_window_derived_solar_generated_wh|previous_window_derived_solar_generated_wh|delta_pct`.
  The solar fields derive minute-bucket solar Wh from `pv_avg_w` when the raw
  stored `solar_generated_wh` column is null, and compare the rebuilt window to
  the immediately preceding window of the same duration.
  For migrated schemas, the rebuilt rows also populate:
  `ac_input_energy_wh`, `ac_output_energy_wh`, `dc_output_energy_wh`,
  `load_energy_wh`, `battery_charge_energy_wh`, and
  `battery_discharge_energy_wh`.
  `touched_buckets` counts minute buckets whose `updated_at` moved during this
  rebuild run.
  Optional overrides:
  - `REGEN_FROM='2026-03-05T00:00:00Z'`
  - `REGEN_TO='2026-03-06T00:00:00Z'`
  - `REGEN_MAX_OBJECTS=500`
- `make dev-down` uninstalls `pulse-services` and `pulse-platform`; preserves cluster by default.
  Set `DELETE_CLUSTER=1` to also delete the local k3d cluster.
- `make db-migrate-up-local` applies all SQL up migrations from `deploy/db/migrations` to the local CNPG primary (`k3d-pulse-local`, namespace `pulse-platform`, service `pulse-platform-core-rw`).
- `make db-migrate-down-local` applies all SQL down migrations in reverse order from `deploy/db/migrations` to the same local CNPG primary.
- `make db-migrate-verify-local` runs schema verification checks for M1/M2/M3 tables and constraints:
  - `users`, `devices`, `user_devices`, `provider_credentials`, `provider_devices`, `archive_object_manifest` existence
  - rollup table existence: `telemetry_rollup_minute`, `telemetry_rollup_hour`, `telemetry_rollup_day`
  - Timescale hypertable registration for the 3 rollup tables
  - Timescale retention policies for the 3 rollup tables:
    - minute: `90 days`
    - hour: `3 years`
    - day: `3 years`
  - `users.id` default expression (`uuidv7()`)
  - no DB default expression on `users.created_at`/`users.updated_at`
  - control-plane and rollup check constraints (`keycloak_subject`, `ecoflow_sn`, `role`, rollup PKs)
- `make db-migrate-cycle-local` runs the full local validation cycle:
  - `up` -> `verify` -> `down` -> `up` -> `verify`
- `make db-migrate-e2e-local` runs data-path checks on top of applied schema:
  - inserts `users`/`devices`/`user_devices` linkage,
  - verifies ownership join shape (`keycloak_subject -> user_devices -> ecoflow_sn`),
  - verifies uniqueness guards for `keycloak_subject` and `ecoflow_sn`,
  - verifies role guard rejects invalid roles.
- `make db-seed-dev-local` performs explicit local provider/dev-device seeding:
  - loads `.env` first when present (for `ECOFLOW_DEV_ACCESS_KEY` / `ECOFLOW_DEV_SECRET_KEY`),
  - uses temporary `kubectl port-forward` to local CNPG primary service,
  - runs `go run ./cmd/ecoflow-dev-seed` with:
    - `CONTROL_PLANE_DB_DSN` pointed at local forwarded Postgres,
    - `ECOFLOW_DEV_USER_SUBJECT`, `ECOFLOW_DEV_USER_EMAIL`,
    - `ECOFLOW_DEV_SEED_SNS`.
  - defaults:
    - `DB_SEED_USER_SUBJECT=dev-user@example.com`,
    - `DB_SEED_USER_EMAIL=dev-user@example.com`,
    - `DB_SEED_SERIALS=DEMOD2M00001057,DEMODPU0000294`.
  - after credential rotation, recycle ingest sessions so workers immediately
    reconnect with fresh provider credentials:
    - `kubectl -n pulse-services rollout restart deploy/pulse-services-go-ingest`
    - `kubectl -n pulse-services rollout status deploy/pulse-services-go-ingest --timeout=120s`
- `make dr-backup-local` captures a local DR backup bundle in
  `.tmp/dr-backups/<name>/` (default `<name>=latest`):
  - dumps Postgres control-plane + archive-manifest table data from local CNPG
    primary to
    `postgres.data.sql`,
  - opens temporary local MinIO port-forward
    (`svc/pulse-platform-minio -> 127.0.0.1:19000`),
  - mirrors MinIO raw archive bucket (`pulse-telemetry-raw`) to local files
    using Dockerized `minio/mc`,
  - writes `report.env` with baseline DB/object counts for restore validation.
  - override backup name with `DR_BACKUP_NAME=<name>`.
  - override local MinIO forward port with
    `DR_MINIO_LOCAL_PORT=<port>` if `19000` is in use.
- `make dr-restore-local` restores Postgres + MinIO from an existing local DR
  bundle (`DR_BACKUP_NAME=<name>`, default `latest`):
  - truncates managed tables, then restores Postgres data via `psql`,
  - clears and rehydrates MinIO bucket content from backup files via the same
    temporary local MinIO port-forward + Dockerized `minio/mc`.
- `make dr-drill-local` runs the full local DR-lite drill:
  - backup (`dr-backup-local`),
  - simulated data loss (truncate key Postgres tables + remove MinIO bucket objects),
  - restore (`dr-restore-local`),
  - schema verification (`make db-migrate-verify-local`),
  - floor-based validation against `report.env` (`actual >= expected`) so
    concurrent live ingest growth is noted but non-fatal.
- `make auth-keycloak-verify-local` validates Keycloak realm bootstrap on local k3d:
  - authenticates with `kcadm` against running Keycloak pod,
  - verifies realm `$(KEYCLOAK_REALM_NAME)` exists (default `pulse`),
  - verifies social providers `google` and `facebook` are present in that realm.
- `make gke-context` fetches kube credentials for GKE dev.
  Required variables:
  - `GKE_PROJECT_ID` (required)
  Optional variables:
  - `GKE_CLUSTER_NAME` (default `pulse-dev`)
  - `GKE_CLUSTER_ZONE` (default `us-east1-b`)
  - `GKE_BASELINE_NODEPOOL` (default `baseline-pool`; use `default-pool` if cluster was created via raw `gcloud container clusters create`)
- `make gke-dev-guardrails` creates `pulse-dev` namespace if needed and applies:
  - `deploy/env/dev/guardrails/pulse-dev-resourcequota.yaml`
  - `deploy/env/dev/guardrails/pulse-dev-limitrange.yaml`
- `make gke-park` scales stateless app deployments down and reduces node-pool minimums for cost-min idle mode.
  Defaults:
  - `GKE_STATELESS_DEPLOYMENTS="node-bff ws-gateway query-api projection ingest"`
  - baseline pool min/max: `1/4`
  - spot pool min/max: `0/4`
- `make gke-wake` restores baseline autoscaling settings, reapplies guardrails, and scales stateless app deployments up.
  Defaults:
  - stateless replicas: `2`
  - baseline pool min/max: `2/4`
  - spot pool min/max: `0/4`
- `make scale-down` is a convenience alias for `make gke-park` (same required variables and behavior).
- `make scale-up` is a convenience alias for `make gke-wake` (same required variables and behavior).
- `make argocd-bootstrap-dev` installs/upgrades Argo CD in `argocd` namespace on GKE dev and waits for:
  - `crd/applications.argoproj.io` Established
  - `deploy/argocd-server` Ready
  - `deploy/argocd-repo-server` Ready
  - `sts/argocd-application-controller` Ready
  Defaults:
  - chart: `argo/argo-cd`
  - version: `9.4.3`
  - values: `deploy/env/dev/values.argocd.yaml`
- `make argocd-apps-dev` applies direct app manifests in `deploy/argocd/apps/` (`pulse-platform`, `pulse-services`).
- `make argocd-wait-apps` waits until each Argo Application reaches:
  - `status.sync.status=Synced`
  - `status.health.status=Healthy`
- `make argocd-dev-up` is the full bootstrap sequence:
  - Argo CD install/upgrade
  - direct app apply
  - app sync/health wait loop
- pre-merge GKE validation pattern (when Argo apps track `main` but you need to validate branch changes):
  - patch app `spec.source.targetRevision=<branch>` with `argocd.argoproj.io/refresh=hard`,
  - run `make argocd-wait-apps`,
  - verify platform deployments/CRDs,
  - restore `spec.source.targetRevision=main` and wait again.

For complete fresh-project bootstrap (project creation, billing, APIs, cluster create, Argo bootstrap), see:
`docs/how-to/setup-gke-dev-project.md`.
