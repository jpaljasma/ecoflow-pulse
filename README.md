# EcoFlow API Playground (Go 1.26)

Production-focused EcoFlow HTTP client foundation with:

- request signing (HMAC-SHA256 canonical parameters),
- resilient retry with exponential backoff + full jitter,
- tuned HTTP transport (compression, keep-alives, HTTP/2 attempt),
- environment-scoped credentials (`dev` / `staging` / `prod`),
- observability + metrics hooks by default (OTEL-friendly interfaces).

## Layout

- `pkg/ecoflow`: reusable client package.
- `pkg/ecoflowserver`: high-throughput HTTP server foundation with compression middleware.
- `cmd/ecoflow-server`: runnable API server entrypoint.
- `cmd/ecoflow-smoke`: minimal executable for manual connectivity checks.
- `cmd/ecoflow-mqtt-sub`: MQTT quota subscriber for device-to-app telemetry.

This structure keeps feature work parallel-friendly: endpoint-specific services can be added in `pkg/ecoflow/<domain>_service.go` without touching core transport/signing code.

## MQTT Telemetry Dashboard

`cmd/ecoflow-mqtt-sub` provides a live terminal dashboard for EcoFlow MQTT device-to-app telemetry (`/open/{certificateAccount}/{sn}/quota`).

Supported models (actively tested):

- DELTA 2 Max (D2M)
- DELTA Pro Ultra (DPU)

Telemetry capabilities:

- startup bootstrap from `GetDeviceAllQuota()` for initial state mapping,
- live summary table (SOC, AC input, solar generated, output, net, state, updated time),
- per-device status flags (capability-dependent): AC, DC/USB, USB, 12V DC, EV charging, UPS passthrough, solar passthrough, grounded estimate,
- fan/cooling status indicator (`Fan On`) when device telemetry provides `fanState`/`fanLevel`,
- PV channel telemetry (low/high): watts, volts, amps, and state (`active` / `locked` / `idle`),
- channel breakdown: AC in, PV in (low/high + total), AC out, DC out, XT150 in/out,
- battery pack telemetry (up to 5 packs): SOC, temp, power, remain, max cell delta, serial, energy, SOH, voltage, target SOC, limits, delta SOC, capacity, board temperature,
- heuristic + ML-based ETA estimate rows with confidence scoring,
- minute-by-minute history (default 10-minute window): solar generated, AC input, AC output, DC output, battery charge, total input/output, net,
- minute-by-minute history values are energy buckets (Wh per minute), not instantaneous power (W),
- persistent minute telemetry history at `logs/telemetry_history.jsonl` (append-only) to preload ML/minute history on startup,
- durable logging to `logs/mqtt.log` (cleared each run), including raw payloads and parsed `energy_summary`,
- resilient ingestion: reconnect with jitter backoff, idle-time reconnect, drop-oldest ring-buffer behavior, graceful quit (`q` or `Ctrl+C`).
- MQTT connection diagnostics in dashboard (queue/degraded/last message metadata + connection uptime).
- refactored ETA/ML estimation engine into dedicated code paths for easier tuning and future model iteration.

Recent improvements:

- ML confidence warmup tuned to ramp faster on stable streams,
- MPPT/PV voltage normalization hardened for low-light raw payload scaling edge cases,
- startup + persisted minute-history preload used to warm ETA/ML context after restart.
- bootstrap logging now explicitly reports `GetDeviceAllQuota` start/success/failure in `logs/mqtt.log`.
- MQTT write timeout became configurable and idle-forced reconnects are disabled by default (`ECOFLOW_MQTT_IDLE_RECONNECT_AFTER=0s`).

## Environment Variables

Shared fallback:

- `.env` is auto-loaded by `ConfigFromEnvironment()` when present.
- `ECOFLOW_ACCESS_KEY`
- `ECOFLOW_SECRET_KEY`
- `ECOFLOW_BASE_URL` (optional, default `https://api.ecoflow.com`)
- `ECOFLOW_ENV` (`dev|staging|prod`, default `dev`)
- `ECOFLOW_DEBUG` (optional bool; defaults `true` in `dev`, `false` in `staging/prod`)
- `ECOFLOW_ADVANCED_DEBUG_TELEMETRY` (optional bool; default `false`)
- `ECOFLOW_DEBUG_LOG_HEADERS` (optional bool; default `false`; logs full request/response headers in debug mode)
- `ECOFLOW_REQUEST_COMPRESSION` (optional bool; default `true`)
- `ECOFLOW_RESPONSE_COMPRESSION` (optional bool; default `true`)
- `ECOFLOW_REQUEST_COMPRESSION_ALGORITHM` (optional; default `gzip`)
- `ECOFLOW_REQUEST_COMPRESSION_MIN_BYTES` (optional int; default `0`)
- `ECOFLOW_ACCEPT_ENCODINGS` (optional CSV; default `gzip,deflate`)
- `ECOFLOW_REQUEST_TIMEOUT` (optional, e.g. `20s`)
- `ECOFLOW_DOTENV_PATH` (optional, default `.env`)
- `ECOFLOW_MQTT_SN` (optional; explicit target SN)
- `ECOFLOW_MQTT_DEVICE_MATCH` (optional; default `delta pro ultra`)
- `ECOFLOW_MQTT_TOPIC` (optional; override `/open/{certificateAccount}/{sn}/quota`)
- `ECOFLOW_MQTT_KEEPALIVE` (optional duration; default `60s`)
- `ECOFLOW_MQTT_CONNECT_TIMEOUT` (optional duration; default `10s`)
- `ECOFLOW_MQTT_READ_TIMEOUT` (optional duration; default `30s`)
- `ECOFLOW_MQTT_WRITE_TIMEOUT` (optional duration; default `15s`)
- `ECOFLOW_MQTT_IDLE_RECONNECT_AFTER` (optional duration; default `0s` disabled; set positive duration to enable idle-triggered reconnects)
- `ECOFLOW_MQTT_QUEUE_CAPACITY` (optional int; default `64`; ingress queue with drop-oldest behavior when full)
- `ECOFLOW_MQTT_PRINT_PAYLOAD` (optional bool; default `false`)
- `ECOFLOW_MQTT_HISTORY_PATH` (optional; default `logs/telemetry_history.jsonl`)
- `ECOFLOW_MQTT_HISTORY_LOAD_WINDOW_MINUTES` (optional int; default `180`; on startup load only recent minute history for this many minutes)
- `ECOFLOW_MQTT_AUTH_REJECT_THRESHOLD` (optional int; default `3`; repeated MQTT code=5 threshold before fallback polling)
- `ECOFLOW_MQTT_FALLBACK_POLL_INTERVAL` (optional duration; default `15s`; REST `GetDeviceAllQuota` poll interval while degraded)
- `ECOFLOW_MQTT_FALLBACK_POLL_TIMEOUT` (optional duration; default `12s`; timeout per REST fallback poll)

Environment-specific (preferred):

- `ECOFLOW_DEV_ACCESS_KEY`, `ECOFLOW_DEV_SECRET_KEY`
- `ECOFLOW_STAGING_ACCESS_KEY`, `ECOFLOW_STAGING_SECRET_KEY`
- `ECOFLOW_PROD_ACCESS_KEY`, `ECOFLOW_PROD_SECRET_KEY`

## Quick Start

```bash
go test ./...
go run ./cmd/ecoflow-smoke
go run ./cmd/ecoflow-server
```

Or use Make:

```bash
make lint
make test
make bench
make build
make smoke
make mqtt
```

By default, `make` commands compile with `-tags=moderncompress` so client/server
`br` and `zstd` codecs are enabled.
The Make defaults also include `-mod=mod`, so first run will auto-resolve and
write required module checksums to `go.sum`.
If you need to disable this for a run:

```bash
GOFLAGS= make test
```

## OpenTelemetry (Optional)

`pkg/ecoflow/otel_adapter.go` is behind the `otel` build tag.

```bash
go get go.opentelemetry.io/otel go.opentelemetry.io/otel/metric go.opentelemetry.io/otel/trace
go test -tags otel ./...
```

## Notes

- The signing implementation follows public EcoFlow client behavior (canonical sorted key-value list + HMAC SHA-256).
- If EcoFlow changes canonicalization rules for nested JSON bodies, only `pkg/ecoflow/signer.go` needs updates.
- `ObservabilityOptions` exposes tracer/meter interfaces so you can plug OpenTelemetry SDK exporters in connected environments.
- `pkg/logger` provides a high-performance structured JSON logger, used by default in the EcoFlow client.
- Outgoing EcoFlow HTTP requests are debug-logged (with redacted key details) when debug logging is enabled.
- Latency/CPU/memory/bytes/compression profiling fields are only included when `ECOFLOW_ADVANCED_DEBUG_TELEMETRY=true`.
- Full HTTP request and response headers are included only when `ECOFLOW_DEBUG_LOG_HEADERS=true`.
- EcoFlow client request-body compression is enabled by default (`gzip`, threshold `0`) and skips compression automatically when payload does not shrink.
- EcoFlow client response compression negotiation/decoding is enabled by default (`gzip,deflate`).
- If you configure unsupported client encodings (for example `br`/`zstd` without `moderncompress` build tag), client initialization fails fast with a clear error.
- `go.mod` includes modern compression dependencies (`andybalholm/brotli`, `klauspost/compress`) for `moderncompress` builds.
- Server compression supports `gzip` and `deflate` by default for both request decoding and response encoding.
- To enable Brotli (`br`) and Zstandard (`zstd`), build with `-tags moderncompress` and add:
  - `github.com/andybalholm/brotli`
  - `github.com/klauspost/compress/zstd`
- HTTP/2 is supported automatically when serving TLS with Go's standard `net/http` server.

## Server Runtime Tuning

- `ECOFLOW_SERVER_ADDR` (default `:8080`)
- `ECOFLOW_SERVER_GOMAXPROCS` (optional; defaults to Go runtime behavior)
- `ECOFLOW_SERVER_COMPRESSION_MIN_BYTES` (default `1024`)
- `ECOFLOW_SERVER_GZIP_LEVEL` (default `5`)
- `ECOFLOW_SERVER_DEFLATE_LEVEL` (default `5`)
- `ECOFLOW_SERVER_BROTLI_LEVEL` (default `5`, only with `moderncompress`)
- `ECOFLOW_SERVER_ZSTD_LEVEL` (default `3`, only with `moderncompress`)
