# AGENTS

## Scope
This file adds backend/runtime guidance for `internal/` work on top of the repository root `AGENTS.md`.

## Backend Design
1. Optimize for long-lived service correctness first:
   - clean startup,
   - bounded retries,
   - graceful shutdown,
   - no leaked transactions, goroutines, or connections.
2. Default non-cryptographic internal hashing to `XXH3_128`:
   - use it for internal cache keys, internal dedup/checksum tags, and other high-volume pipeline hashing where collision resistance only needs to be pragmatic,
   - keep SHA-2/HMAC style hashing for security boundaries, auth/signing, or places where an external protocol/storage contract explicitly requires it.
3. Shared behavior belongs in shared packages:
   - connection-pool tuning,
   - compatibility shims,
   - metrics-server helpers,
   - retry helpers.
4. Prefer fixes that keep worker behavior safe under multi-replica rolling updates, not just single-process local runs.

## Database and Messaging
1. Idle connections and idle transactions must be bounded and cleaned up on normal shutdown.
2. Retry only safe transient operations, with short backoff and jitter.
3. Preserve write safety: do not add automatic retries to writes unless idempotency and side effects are explicitly handled.
4. Background workers must release leases, subscriptions, and DB resources cleanly during drain.

## Go Quality Gates
1. Keep `golangci-lint run ./...` clean.
2. Run targeted `go test` for touched packages and `make test-race` for concurrency-sensitive shutdown/lease/worker changes.
3. When touching a hot path, add or update regression tests for:
   - cancellation,
   - drain/shutdown,
   - retries/backoff,
   - resource cleanup.

## Observability
1. Hot-path logs should stay structured and sampled appropriately.
2. Rollout and shutdown behavior should be diagnosable from logs and metrics:
   - startup dependency retries,
   - readiness/drain transitions,
   - dropped work / queue depth,
   - lease and connection pressure symptoms.
