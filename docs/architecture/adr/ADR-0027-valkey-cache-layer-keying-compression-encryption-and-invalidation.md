# ADR-0027: Valkey Cache Layer Keying, Compression, Encryption, and Invalidation

**Status:** Superseded
**Superseded by:** ADR-0028, ADR-0029
**Date:** 2026-05-14
**Related:** ADR-0004

## Context

ADR-0004 selected Valkey with replication and Sentinel as the hot cache
topology. Backend code now has several independent cache shapes: weather hot
forecasts, inference comparison results, EnergyService history helpers,
control-plane device context, and provider MQTT session material.

Phase 1 needs one shared Go cache substrate without changing the existing
Valkey Sentinel topology. The implementation should make keys ready for a
future clustered topology, avoid key scans, and provide a path to larger
keyspaces later. "Billions of keys" is a phased scale target, not a phase-1
deployment requirement.

## Decision

Use a shared internal Go cache package for backend cache users.

- Canonical keys use
  `prefix:namespace:{partition}:xxh3-128:<digest>`.
- Digests use `internal/hashutil.XXH3Hex128` for non-cryptographic cache key
  identity.
- Domain code supplies canonical input parts; custom key generation is limited
  to canonical inputs, not ad hoc string-concatenated final keys.
- Payloads are wrapped in a versioned envelope containing content type, codec,
  encryption key id, original/stored sizes, created timestamp, and bytes.
- Payloads above `4 KiB` are S2-compressed only when compression shrinks the
  stored body.
- Sensitive provider MQTT session payloads use AES-GCM encryption. If no cache
  encryption key is configured, that cache is bypassed and plaintext is not
  written.
- Tag invalidation is versioned. Invalidating a tag increments its version key;
  future cache keys include the new version and old payloads expire by TTL.
  Reverse indexes and key scans are intentionally avoided.
- Cache clients opt into valkey-go client-side caching through shared cache
  read paths with explicit local TTLs. Existing lease/script clients keep
  client-side caching disabled by default.
- Cache clients are persistent process-level Valkey clients and are closed only
  on service shutdown.

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
- REST BFF device-list or feature-flag reads that are expensive but not
  contract-critical.
- Weather/solar forecast read-through fragments keyed by site/device scope.
- Rate-limit counters and idempotency windows when they can tolerate Valkey
  failover semantics.
- Replay or repair job coordination metadata where TTL-bounded state is enough.

## Scale Triggers

Stay on the current Sentinel-backed topology until one or more of these become
true:

- Working-set size or memory fragmentation requires horizontal keyspace
  sharding beyond vertical node sizing.
- Hot partitions cannot be resolved by key design, local TTLs, or workload
  isolation.
- Operational failover or resharding needs exceed the Sentinel model.
- Managed cache cost/reliability becomes better than operating the in-cluster
  Valkey stateful set.

At that point, create a new ADR for Valkey Cluster or managed cache. Do not
silently replace the ADR-0004 topology.

## Consequences

### Positive

- Cache behavior is consistent across Go services.
- Key format is already hash-tagged for cluster migration.
- No-scan tag invalidation keeps invalidation cost bounded.
- Sensitive provider session caching fails closed when encryption is absent.
- Client-side caching is constrained to explicit shared cache reads.

### Tradeoffs

- Old cache entries written before this envelope format cold-miss until TTL
  expiry.
- Versioned tag invalidation relies on bounded payload TTLs for cleanup.
- AES-GCM key management becomes an operational dependency before sensitive
  provider-session caching is enabled.

### Follow-ups

- Add a live Valkey client-side-cache validation target behind an explicit
  integration flag.
- Revisit topology when the scale triggers above are observed.
