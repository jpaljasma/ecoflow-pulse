# ADR-0028: Shared Cache Substrate Keying, Compression, Encryption, and Invalidation

**Status:** Accepted
**Date:** 2026-05-14
**Related:** ADR-0004, ADR-0029
**Supersedes:** ADR-0027

## Context

ADR-0027 bundled two decisions together: the portable cache behavior services
should use, and the Valkey-specific runtime/client behavior that backs phase 1.
Future work will be faster and safer if agents can reason about those layers
separately.

Backend code has several high-value cache users: weather hot forecasts,
inference comparison results, EnergyService history helpers, control-plane
device context, and provider MQTT session material. Phase 1 keeps Valkey as the
backing store, but cache semantics should stay consistent and mostly portable
if a later ADR moves a subset of cache traffic to Valkey Cluster, managed
cache, or another backend.

"Billions of keys" remains a phased path, not the current deployment target.
The substrate must still avoid decisions that would make future keyspace
sharding, replay-safe invalidation, or operational observability hard.

## Decision

Use one shared Go cache substrate for backend cache behavior. New process-wide
or cross-replica caches should use this substrate unless an ADR or local design
note documents why they need a different contract.

- Canonical keys use `prefix:namespace:{partition}:xxh3-128:<digest>`.
- Digests use `internal/hashutil.XXH3Hex128` for non-cryptographic cache key
  identity.
- Domain code owns canonical input parts. Shared cache code owns the final key
  shape, partition hash tag, envelope, compression, encryption, invalidation,
  and metrics.
- Custom key generation hooks may generate canonical inputs, but should not
  concatenate final cache keys by hand.
- Payloads are wrapped in a versioned envelope containing content type, codec,
  encryption key id, original/stored sizes, created timestamp, and bytes.
- Payloads above `4 KiB` are S2-compressed only when compression shrinks the
  stored body.
- Sensitive provider MQTT/session/certification payloads use AES-GCM
  encryption. If no cache encryption key is configured, sensitive caches fail
  closed by bypassing the cache rather than storing plaintext.
- Versioned tag invalidation is the default invalidation model. Invalidating a
  tag increments its version key; future cache keys include the new version and
  old payloads expire by TTL.
- Reverse indexes, wildcard deletes, and key scans are not part of cache
  invalidation.
- Read-through cache helpers must use `singleflight` for loaders that can
  stampede under concurrent misses.
- Process-local memoization may front a shared Valkey cache for hot repeated
  reads when the memo is TTL-bound, stores cloned values, and remains
  subordinate to the cross-replica cache contract.
- Session/sliding-TTL helpers must enforce both idle extension and a hard cap
  when provider expiry is known or a conservative max age is configured.
- Cache metrics must be low-cardinality and must never label by device ID,
  serial number, user ID, raw cache key, raw tag, or location.

## Phase-1 Cache Users

- Weather hot forecast cache.
- Inference energy-comparison cache.
- EnergyService historical calendar cache.
- EnergyService PV-port history cache.
- Inference worker control-plane device-context cache.
- Encrypted provider MQTT session/certification cache pilot for Pecron, Anker
  SOLIX, and EcoFlow-compatible MQTT material.

## Beneficial Future Uses

- Control-plane authorization and profile bootstrap lookups with short TTLs.
- REST BFF device-list, entitlement, or feature-flag reads that are expensive
  but not contract-critical.
- Weather and solar forecast fragments keyed by site/device scope.
- Provider capability and certification metadata with explicit max age.
- Rate-limit counters and idempotency windows when failover semantics are
  acceptable for the business flow.
- Replay or repair job coordination metadata where TTL-bounded state is enough.

## Consequences

### Positive

- Cache behavior stays consistent across Go services.
- Key format is already hash-tagged for future cluster migration.
- No-scan tag invalidation keeps invalidation cost bounded.
- Sensitive provider session caching fails closed when encryption is absent.
- Business cache choices can be discussed separately from Valkey operations.

### Tradeoffs

- Old cache entries written before this envelope format cold-miss until TTL
  expiry.
- Versioned tag invalidation relies on bounded payload TTLs for cleanup.
- AES-GCM key management becomes an operational dependency before sensitive
  provider-session caching is enabled.
- Shared cache helpers add a small abstraction cost, so hot-path changes need
  benchmarks when behavior changes materially.

## Validation Expectations

- Unit tests cover key stability, canonicalization, hash-tag partitioning,
  envelope compatibility, compression thresholds, encryption key-id handling,
  wrong-key failure, no-plaintext sensitive payload storage, tag invalidation,
  and sliding-TTL hard caps.
- End-to-end tests cover business cache users across independent service/cache
  instances, especially weather, EnergyService history, inference comparison,
  and encrypted provider-session reuse.
- Benchmarks cover key generation, envelope encode/decode, compressed decode,
  encrypted decode, cache hit/miss paths, and any new concurrency optimization.

## Follow-ups

- Revisit cache user priorities as Pulse adds new paid/business flows.
- Add a new ADR before changing the cache invalidation model away from
  versioned tags.
