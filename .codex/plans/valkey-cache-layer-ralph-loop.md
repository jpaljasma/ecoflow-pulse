# Valkey Cache Layer Ralph Loop

## Goal

Build phase 1 of the shared Go Valkey cache substrate on the existing
Sentinel-backed Valkey topology. Keep current operations stable while making
cache keys, payload envelopes, invalidation, compression, encryption, metrics,
and local client-side caching reusable across backend services.

## Workstreams

- [x] Project manager: branch hygiene and task scaffolding.
- [ ] Backend Go: cache substrate, hot-cache migrations, provider-session pilot.
- [ ] Platform deploy: configuration, Helm values, and documentation.
- [ ] QA: unit, integration, benchmark, race, and lint validation.

## Acceptance Criteria

- New `internal/valkeycache` package provides canonical keying, envelopes,
  threshold compression, optional AES-GCM encryption, versioned tag
  invalidation, sliding TTL helpers, observability, and optional local
  client-side-cache reads.
- Existing Valkey lease/script clients preserve client-side caching disabled by
  default; cache clients opt in explicitly.
- Weather, inference energy-comparison, EnergyService history, and inference
  device-context caches use the shared substrate where Valkey is available.
- Provider MQTT session material uses encrypted cache paths only when a cache
  encryption key is configured; otherwise sensitive caching is bypassed.
- ADR-0027 documents the decision and phase-two scale triggers.
- Relevant Go tests, benchmarks, race checks, and Markdown lint evidence are
  recorded for the pull request.

## Risks

- Provider-session cache must fail closed to avoid plaintext sensitive cache
  writes.
- Client-side caching must stay confined to shared cache read paths.
- Key/tag cardinality in metrics must remain bounded.
- Existing in-memory caches should remain fallback paths when Valkey is absent.
