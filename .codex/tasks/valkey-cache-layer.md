# Valkey Cache Layer Phase 1

## Status

PROGRESS

## Checklist

- [x] Create branch and Ralph Loop files.
- [x] Add shared cache substrate tests.
- [x] Implement shared cache substrate.
- [x] Refactor Valkey client setup for cache-client opt-in.
- [x] Migrate backend hot caches.
- [x] Add encrypted provider MQTT session cache pilot.
- [x] Update Helm/config docs.
- [x] Add ADR-0027 and ADR index entry.
- [x] Run targeted tests and record benchmark evidence.
- [ ] Package branch into PR.

## Notes

Phase 1 intentionally preserves the current Valkey replication + Sentinel
topology. Keys are cluster-ready and tag invalidation is versioned so old values
expire naturally without reverse-index scans.

## Validation Evidence

- `go test ./internal/valkeycache ./internal/weatherd/... ./internal/inference ./internal/provideradapter ./cmd/ecoflow-grpc-api ./cmd/ecoflow-inference-worker -count=1`
- `go test ./cmd/ecoflow-ingest-worker ./cmd/ecoflow-scheduler ./internal/ingestlease -count=1`
- `go test ./internal/valkeycache -bench=. -benchmem -run '^$'`
- `make test-race`
- `make lint`
- `helm template pulse-services deploy/charts/pulse-services -f deploy/env/local/values.services.yaml`
- `helm template pulse-services-cloud deploy/charts/pulse-services -f deploy/env/cloud/values.services.yaml`
