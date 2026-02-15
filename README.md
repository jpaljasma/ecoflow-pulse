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
- `ECOFLOW_MQTT_PRINT_PAYLOAD` (optional bool; default `false`)

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
