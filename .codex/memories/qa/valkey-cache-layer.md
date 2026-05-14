# Valkey Cache Layer QA Memory

- Unit coverage must include key stability, hash-tag partitioning, tag-version
  invalidation, compression threshold behavior, AES-GCM failures, key-id
  handling, and sliding TTL hard-cap behavior.
- Integration coverage should use miniredis where protocol support is enough.
- A live Valkey/client-side-cache check must stay behind an explicit flag.
- Regression targets:
  `go test ./internal/valkeycache ./internal/weatherd/... ./internal/inference ./internal/provideradapter ./cmd/ecoflow-grpc-api ./cmd/ecoflow-inference-worker -count=1`
  and `make test-race` for concurrency-sensitive cache/session changes.
