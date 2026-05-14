# ADR-0029: Valkey Cache Runtime, Client Topology, and Reliability

**Status:** Accepted
**Date:** 2026-05-14
**Related:** ADR-0004, ADR-0014, ADR-0028
**Supersedes:** ADR-0027

## Context

ADR-0004 selected Valkey with replication and Sentinel as the hot cache
topology. ADR-0014 uses Valkey for distributed ingest leases. ADR-0028 defines
the shared cache substrate behavior that backend services should use.

This ADR captures only the Valkey-specific runtime and client policy for phase
1. It does not replace the Sentinel-backed topology selected in ADR-0004 and it
does not introduce Valkey Cluster for the current rollout.

## Decision

Use the existing Sentinel-backed Valkey clusters for phase-1 cache traffic.
Cache clients, lease clients, and script clients share common client bootstrap
helpers, but they must keep their runtime modes explicit.

- Process-level Valkey clients are persistent and are closed only during
  service shutdown.
- Client construction uses the shared Valkey client helper so Sentinel,
  authentication, retry, backoff, jitter, reconnect, and client-side-cache
  options stay centralized.
- Cache clients opt into valkey-go client-side caching only through the shared
  cache read path, and only with explicit per-namespace local TTLs.
- Lease/script clients keep client-side caching disabled. Ingest lease clients
  remain `DisableCache=true` because locks, fencing, and Lua scripts must not
  couple to client-tracking state.
- Valkey operations that are safe to retry use bounded retries with backoff and
  jitter. Writes are retried only when the operation is idempotent or the call
  path explicitly handles side effects.
- Client behavior must reliably reconnect after disconnects, Sentinel primary
  changes, and transient network failures.
- Lock/write paths must target a writable primary through Sentinel, not random
  replica fan-out endpoints.
- Cache key design comes from ADR-0028; Valkey clients must not introduce
  wildcard scans or reverse-index invalidation.
- Local/dev and hosted environments keep AOF and PVC-backed Valkey data nodes
  when the data is part of the default availability baseline.

## Configuration Ownership

Keep cache-specific and Valkey-specific configuration separate:

- Cache knobs: namespace local TTLs, compression threshold, encryption key id,
  encryption keys, sensitive-cache enablement, and tag/version TTL policy.
- Valkey knobs: Sentinel addresses, master set name, credentials, database,
  TLS, retry/backoff/jitter settings, client-side-cache enablement, and local
  cache TTL bounds.
- Deployment values should expose these groups separately so a future backend
  migration can keep cache behavior stable while changing the Valkey runtime.
- Secrets must stay in Kubernetes Secrets or an approved secret source; PRs and
  docs must not include provider session material, Valkey credentials, or cache
  encryption keys.

## Scale Triggers

Stay on the current Sentinel-backed topology until one or more of these become
true:

- Working-set size or memory fragmentation requires horizontal keyspace
  sharding beyond vertical node sizing.
- Hot partitions cannot be resolved by key design, explicit local TTLs,
  request coalescing, or workload isolation.
- Operational failover or resharding needs exceed the Sentinel model.
- Managed cache cost/reliability becomes better than operating the in-cluster
  Valkey StatefulSet.

At that point, create a new ADR for Valkey Cluster, managed cache, or another
runtime. Do not silently replace the ADR-0004 topology.

## Consequences

### Positive

- Valkey runtime concerns are centralized instead of copied into each cache
  user.
- Persistent clients avoid per-request connection churn.
- Client-side caching stays constrained to explicit read paths with bounded
  local TTLs.
- Retry/reconnect behavior can be validated once and reused across services.

### Tradeoffs

- Sentinel remains simpler than cluster mode, but does not provide horizontal
  keyspace sharding.
- Client-side caching adds invalidation behavior that must be tested with live
  Valkey behind an explicit integration flag.
- Shared client helpers need careful defaults so lock/script paths and cache
  read paths do not accidentally inherit each other's behavior.

## Validation Expectations

- Unit tests cover retry/backoff/jitter configuration, reconnect-safe helper
  setup, and explicit client-side-cache opt-in/opt-out modes.
- Integration tests cover Valkey `GET`, `SET`, `PEXPIRE`, and `INCR` paths with
  miniredis where supported.
- Live Valkey client-side-cache validation stays behind an explicit integration
  flag.
- Race tests cover cache/session concurrency paths and lease/script clients
  when their shared client setup changes.

## Follow-ups

- Add operational dashboards for cache client reconnects, retry attempts,
  client-side-cache hits/invalidations, and Valkey role/failover events.
- Revisit topology only when the scale triggers above are observed.
