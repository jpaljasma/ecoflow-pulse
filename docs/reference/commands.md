# Reference: Commands

## Go Commands

```bash
go test ./...
make test-race
make test-race-stress
go run ./cmd/ecoflow-smoke
go run ./cmd/ecoflow-server
go run ./cmd/ecoflow-mqtt-sub
go run ./cmd/ecoflow-pv-fingerprint
go run ./cmd/ecoflow-panel-db-import
go run ./cmd/ecoflow-panel-csv-backfill
go run ./cmd/ecoflow-panel-select-train
go run ./cmd/ecoflow-grpc-api
go run ./cmd/ecoflow-dev-seed
go run ./cmd/ecoflow-ingest-worker
go run ./cmd/ecoflow-rollup-worker
go run ./cmd/ecoflow-projection-worker
go run ./cmd/ecoflow-archive-worker
go run ./cmd/ecoflow-replay-cli
go run ./cmd/ecoflow-gap-detector
go run ./cmd/ecoflow-gap-repair-worker
```

## Node/Expo Auth Commands

```bash
# Typecheck + test Node JWKS middleware package.
npm run typecheck --workspace @ecoflow-pulse/node-jwks-auth
npm run test --workspace @ecoflow-pulse/node-jwks-auth

# Pulse platform Node REST BFF.
npm run -w apps/pulse-platform typecheck
npm run -w apps/pulse-platform lint
npm run -w apps/pulse-platform test
npm run platform-bff

# Pulse realtime WebSocket gateway.
npm run -w apps/pulse-realtime-gateway typecheck
npm run -w apps/pulse-realtime-gateway lint
npm run -w apps/pulse-realtime-gateway test
npm run realtime-gateway

# Universal app checks (includes PKCE auth card code paths).
npm run -w apps/universal typecheck
npm run -w apps/universal lint
npm run -w apps/universal test
npm run -w apps/universal e2e:web
```

Run Playwright E2E via repository make target:

```bash
make test-web-e2e
```

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
PULSE_PLATFORM_DEV_SUBJECT='jpaljasma@gmail.com' \
PULSE_PLATFORM_PORT='18081' \
npm run platform-bff
```

Run the realtime WebSocket gateway against the local authz API + live Valkey/NATS data plane:

```bash
# Port-forward a stable Valkey node for local snapshot reads first.
kubectl -n pulse-platform port-forward pod/pulse-platform-valkey-node-0 6380:6379

GRPC_API_ADDR='127.0.0.1:9090' \
NATS_URLS='nats://127.0.0.1:4222' \
VALKEY_ADDRS='127.0.0.1:6380' \
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
VALKEY_ADDRS='127.0.0.1:6380' \
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
VALKEY_ADDRS='127.0.0.1:6379' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-ingest-worker
```

Run projection worker loop (consume ingest envelopes from JetStream and build Valkey live snapshots):

```bash
VALKEY_ADDRS='127.0.0.1:6379' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-projection-worker
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
go run ./cmd/ecoflow-replay-cli -mode device -provider-device-ids R351ZABAPH331057 -from 2026-02-26T08:00:00Z -to 2026-02-26T09:00:00Z -dry-run

# Fleet shard/time replay (publishes TelemetryEnvelope bytes to pulse.telemetry.replay.sNNN).
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
ARCHIVE_OBJECT_ENDPOINT='127.0.0.1:9000' \
ARCHIVE_OBJECT_ACCESS_KEY='minio' \
ARCHIVE_OBJECT_SECRET_KEY='minio123' \
NATS_URLS='nats://127.0.0.1:4222' \
go run ./cmd/ecoflow-replay-cli -mode fleet -shards 7,11 -from 2026-02-26T08:00:00Z -to 2026-02-26T09:00:00Z
```

Run gap detector loop (projection lag detection + targeted replay enqueue):

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse_app:...@127.0.0.1:5432/pulse?sslmode=disable' \
VALKEY_ADDRS='127.0.0.1:6379' \
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
```

## Protobuf / gRPC Generation

```bash
buf generate
buf lint
```

## gRPC Profiling / Benchmarks

```bash
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

# focused cpu/heap profile (10k, high-concurrency startup path)
go test ./internal/ingestworker -run '^$' -bench BenchmarkLoopReconcileStartBatch/10k_workers48_delay50us -benchtime=3x -cpuprofile /tmp/ingestworker.cpu.out -memprofile /tmp/ingestworker.mem.out
go tool pprof -top /tmp/ingestworker.cpu.out
go tool pprof -top -alloc_space /tmp/ingestworker.mem.out
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
make build
make smoke
make mqtt
make ingest-worker
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
  - `markdownlint` over tracked `*.md` files using `.markdownlint.json`
  - if `buf` is missing, it fails with install hint:
    `https://buf.build/docs/installation/`
  - if `markdownlint` is missing, it fails with install hint:
    `brew install markdownlint-cli`
- `make mqtt` exits cleanly on `q`/`Ctrl+C` and does not return non-zero on
  intentional stop.
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
  MinIO credentials must be configured with MinIO-chart top-level keys
  (`minio.rootUser` / `minio.rootPassword`), not `minio.auth.*`, to keep
  `secret/pulse-platform-minio` in sync with services runtime credentials.
  It includes retry/backoff for transient CNPG webhook race conditions during initial install.
  It runs a CRD-safe reconcile flow for CloudNativePG:
  1) initial Helm install/upgrade,
  2) wait for CNPG operator deployment readiness,
  3) second Helm pass to apply CRD-backed resources (for example `Cluster`).
  Connection + bootstrap contract:
  - bootstrap app credentials are configured in `cloudnativepgCluster.bootstrap.*` and rendered to secret `pulse-platform-core-app`,
  - service-facing contract is exposed via configmap `pulse-platform-core-contract`,
  - DSN-style connection secret is exposed via `pulse-platform-core-connection`,
  - local Keycloak is configured to use CNPG (`keycloak.postgresql.enabled=false`, `keycloak.externalDatabase.*` -> `pulse-platform-core-rw`).
  Validation examples:
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
  Recovery examples after Docker restart:
  - `docker restart k3d-pulse-local-agent-0`
  - `kubectl get nodes -o wide`
  - `kubectl get pods -n pulse-platform`
- `make services-up` updates Helm deps and installs/upgrades `pulse-services` using `deploy/env/local/values.services.yaml`.
- `make services-image-build-local` builds local telemetry worker image
  `$(SERVICES_IMAGE_REPO):$(SERVICES_IMAGE_TAG)` from
  `deploy/docker/pulse-services.Dockerfile`.
- `make services-image-import-local` imports that local worker image into
  k3d cluster `$(K3D_CLUSTER_NAME)`.
- `make services-image-local-up` runs build + import for local k3d in one step.
- `make services-up` updates Helm deps and installs/upgrades `pulse-services`
  using `deploy/env/local/values.services.yaml`. By default it auto-builds and
  imports the local worker image before Helm apply; set
  `SERVICES_AUTO_BUILD_IMAGE=0` to skip.
- `make platform-wait` blocks until critical platform dependencies are ready:
  - CNPG operator deployment,
  - CNPG cluster `pulse-platform-core` `Ready` condition,
  - `nats`, `valkey-node`, and `keycloak` statefulsets,
  - `minio` deployment,
  - optional `ingress-nginx` controller deployment,
  - optional `cert-manager` controller/webhook/cainjector deployments,
  - optional External Secrets deployments (`external-secrets`, webhook, cert-controller),
  - optional observability-lite deployments (`kube-promet-operator`, `grafana`, `opentelemetry-collector`).
- `make services-wait` blocks until `pulse-services` pods are `Ready` (if services workloads exist).
- `make dev-up` runs `k3d-up`, `platform-up`, `platform-wait`, `services-up`, then `services-wait`.
  This enforces startup order and returns only when dependencies are actually ready.
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
    - `DB_SEED_USER_SUBJECT=jpaljasma@gmail.com`,
    - `DB_SEED_USER_EMAIL=jpaljasma@gmail.com`,
    - `DB_SEED_SERIALS=R351ZABAPH331057,Y711ZABA9H2P0294`.
  - after credential rotation, recycle ingest sessions so workers immediately
    reconnect with fresh provider credentials:
    - `kubectl -n pulse-services rollout restart deploy/pulse-services-go-ingest`
    - `kubectl -n pulse-services rollout status deploy/pulse-services-go-ingest --timeout=120s`
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
