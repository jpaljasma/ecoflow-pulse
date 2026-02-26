# ADR-0013: Go gRPC Server Baseline — High Throughput, Resilient, HTTP/2 + Streaming

**Status:** Accepted  
**Date:** 2026-02-21  
**Owners:** Jaan  
**Related:** ADR-0001 (overall architecture), ADR-0007 (auth), ADR-0008 (realtime WS), ADR-0011 (dev cost policy)

---

## Context

EcoFlow Pulse is adopting a split architecture:
- Node REST BFF (public)
- Go API/Data layer over gRPC (internal)

We need a **standard gRPC server baseline** for Go that:
- supports high throughput and modern HTTP/2 features
- is resilient (graceful shutdown, timeouts, backpressure)
- supports unary + streaming RPCs
- supports compression (selectively)
- includes observability (metrics/tracing/logging)
- is safe-by-default (authn/authz hooks, message size limits, keepalive abuse protection)

This ADR standardizes configuration and patterns so new services don't invent their own servers.

---

## Decision

### 1) Standard server framework
All Go services exposing gRPC will use:
- `google.golang.org/grpc` (grpc-go)
- a shared `internal/grpcserver` package to build servers consistently

### 2) Required server options (baseline)
Every server uses a consistent set of `grpc.ServerOption`s:
- Interceptor chains:
  - `grpc.ChainUnaryInterceptor(...)`
  - `grpc.ChainStreamInterceptor(...)`
- Keepalive:
  - `grpc.KeepaliveParams(...)`
  - `grpc.KeepaliveEnforcementPolicy(...)`
- Transport tuning (start conservative, tune via load tests):
  - `grpc.ReadBufferSize(...)`
  - `grpc.WriteBufferSize(...)`
  - `grpc.MaxConcurrentStreams(...)`
  - `grpc.InitialConnWindowSize(...)`
  - `grpc.InitialWindowSize(...)`
- Safety limits:
  - `grpc.MaxRecvMsgSize(...)`
  - `grpc.MaxSendMsgSize(...)`
  - `grpc.MaxHeaderListSize(...)`

All gRPC services are **internal-only** (cluster/private network scope), never internet-exposed directly.

### 3) Interceptor requirements (baseline)
**Unary + Stream interceptors MUST include:**
- Request ID / correlation ID propagation
- Structured logging (method, status, latency, request ID)
- Recovery/panic safety (no process crash)
- Authn/authz enforcement (JWT claims in metadata, per-method policy)
- Metrics + tracing hooks (OpenTelemetry)
- Optional follow-up: per-method concurrency limits and rate limiting (load shedding)

### 4) Compression policy
- Register gzip (or other) compression in both client/server binaries.
- Do **not** enable compression globally by default.
- Enable compression per-method / per-call based on payload size thresholds or bandwidth needs.

### 5) Streaming policy
- Streaming RPCs are first-class for real-time telemetry feeds.
- The server must enforce:
  - per-stream backpressure
  - bounded buffering
  - slow-consumer behavior (drop/merge/sample where appropriate)

### 6) Operations & lifecycle
- Provide gRPC health checks.
- Reflection enabled only in local/dev.
- Graceful shutdown on SIGTERM with drain period.

### 7) Throughput and contention policy (mandatory)
- Per-request hot paths must avoid unnecessary locking.
  - Use lock-free atomics for simple counters/ID generation where ordering is sufficient.
  - Introduce mutexes on hot paths only when profiling shows no safer low-contention alternative.
- Stream/update fanout must use bounded channel patterns.
  - No unbounded goroutine spawning under burst traffic.
  - Slow-consumer handling must be explicit (drop/merge/sample policy).
- Minimize allocations in frequent RPC handlers.
  - Reuse immutable defaults/templates when possible.
  - Prefer fixed-size logging attributes over variadic append-heavy logging in interceptors.
- All throughput-sensitive changes require benchmark re-validation before merge.

---

## Recommended starter defaults (tune later)

> These are starting values. Tune with load testing. Prefer conservative defaults over aggressive “tuning by vibes”.

- Keepalive:
  - `Time`: 60s
  - `Timeout`: 20s
  - `MaxConnectionIdle`: 5m
  - `MaxConnectionAge`: 30m (plus jitter)
  - enforcement `MinTime`: 10s
  - `PermitWithoutStream`: false

- Transport:
  - `ReadBufferSize`: 64KB
  - `WriteBufferSize`: 64KB
  - `MaxConcurrentStreams`: 10_000
  - `InitialConnWindowSize`: 4MB
  - `InitialWindowSize`: 1MB

- Message sizes:
  - `MaxRecvMsgSize`: 4MB
  - `MaxSendMsgSize`: 16MB
  - `MaxHeaderListSize`: 16KB (keep modest to avoid abuse)

---

## Implementation sketch (Codex-ready)

### `internal/grpcserver/server.go` (skeleton)

```go
package grpcserver

import (
  "context"
  "net"
  "time"

  "google.golang.org/grpc"
  "google.golang.org/grpc/keepalive"
  "google.golang.org/grpc/reflection"
  _ "google.golang.org/grpc/encoding/gzip"
)

type Config struct {
  ListenAddr string
  Env        string // local|dev|staging|prod

  MaxRecvMsgSize int
  MaxSendMsgSize int

  ReadBufferSize  int
  WriteBufferSize int

  MaxConcurrentStreams uint32
  InitConnWindowSize   int32
  InitStreamWindowSize int32

  KA       keepalive.ServerParameters
  KAEnforce keepalive.EnforcementPolicy
}

func New(cfg Config, unary []grpc.UnaryServerInterceptor, stream []grpc.StreamServerInterceptor) (*grpc.Server, net.Listener, error) {
  lis, err := net.Listen("tcp", cfg.ListenAddr)
  if err != nil {
    return nil, nil, err
  }

  s := grpc.NewServer(
    grpc.ChainUnaryInterceptor(unary...),
    grpc.ChainStreamInterceptor(stream...),

    grpc.KeepaliveParams(cfg.KA),
    grpc.KeepaliveEnforcementPolicy(cfg.KAEnforce),

    grpc.ReadBufferSize(cfg.ReadBufferSize),
    grpc.WriteBufferSize(cfg.WriteBufferSize),

    grpc.MaxConcurrentStreams(cfg.MaxConcurrentStreams),
    grpc.InitialConnWindowSize(cfg.InitConnWindowSize),
    grpc.InitialWindowSize(cfg.InitStreamWindowSize),

    grpc.MaxRecvMsgSize(cfg.MaxRecvMsgSize),
    grpc.MaxSendMsgSize(cfg.MaxSendMsgSize),
    // optionally: grpc.MaxHeaderListSize(...)
  )

  // reflection only in local/dev
  if cfg.Env == "local" || cfg.Env == "dev" {
    reflection.Register(s)
  }

  return s, lis, nil
}

func DefaultConfig(env string) Config {
  return Config{
    ListenAddr: ":9090",
    Env: env,

    MaxRecvMsgSize: 4 << 20,
    MaxSendMsgSize: 16 << 20,

    ReadBufferSize:  64 << 10,
    WriteBufferSize: 64 << 10,

    MaxConcurrentStreams: 10_000,
    InitConnWindowSize:   4 << 20,
    InitStreamWindowSize: 1 << 20,

    KA: keepalive.ServerParameters{
      Time:                  60 * time.Second,
      Timeout:               20 * time.Second,
      MaxConnectionIdle:     5 * time.Minute,
      MaxConnectionAge:      30 * time.Minute,
      MaxConnectionAgeGrace: 30 * time.Second,
    },
    KAEnforce: keepalive.EnforcementPolicy{
      MinTime:             10 * time.Second,
      PermitWithoutStream: false,
    },
  }
}

func Serve(ctx context.Context, s *grpc.Server, lis net.Listener, grace time.Duration) error {
  errCh := make(chan error, 1)
  go func() { errCh <- s.Serve(lis) }()

  select {
  case <-ctx.Done():
    stopped := make(chan struct{})
    go func() {
      s.GracefulStop()
      close(stopped)
    }()
    select {
    case <-stopped:
      return nil
    case <-time.After(grace):
      s.Stop()
      return nil
    }
  case err := <-errCh:
    return err
  }
}
```

---

## Consequences

### Positive
- Every gRPC service starts with a sane, production-leaning baseline
- Consistent instrumentation and security enforcement
- Streaming and compression are supported without bespoke server glue

### Negative / Tradeoffs
- Defaults will need tuning under real load
- Requires a shared internal package and discipline

---

## Acceptance criteria
- New gRPC services can be created with no bespoke server glue
- Server passes basic load tests (unary + streaming) without resource blowups
- Keepalive does not cause ping floods or connection churn
- Observability exists for latency, errors, and saturation
- 10k synthetic fleet soak remains available as an opt-in regression gate with:
  - steady p99 latency threshold,
  - burst p99 latency threshold,
  - heap delta ceiling.

---

## Follow-ups
- [x] Implement `internal/grpcserver` in repo
- [x] Add standard middleware package (`internal/grpcmw`)
- [x] Add telemetry bootstrap service (`TelemetryService`) + health registration
- [x] Add workload-calibrated benchmark harness (observed fleet mix + startup burst)
- [x] Add 10k synthetic fleet soak gate with p99/heap thresholds (opt-in)
- [x] Add contention/allocation profiling workflow (`pprof`, mutex/block profiles, gc tuning)
- [ ] Add load test harness for unary + streaming (k6 or ghz)
- [ ] Decide per-method compression rules (thresholds)
- [x] Replace `NoopAuthorizer` with Keycloak JWKS JWT validation + RBAC enforcement at Go boundary
