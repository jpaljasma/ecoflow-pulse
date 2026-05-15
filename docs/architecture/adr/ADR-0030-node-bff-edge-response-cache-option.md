# ADR-0030: Node BFF Edge Response Cache Option

**Status:** Accepted
**Date:** 2026-05-14
**Related:** ADR-0001, ADR-0028, ADR-0029

## Context

ADR-0028 defines the shared Go cache substrate and ADR-0029 defines the Valkey
runtime policy. The public Node REST BFF still has a separate opportunity: it
can avoid repeated gRPC calls during short user-visible bursts, especially for
weather routes that already have durable/shared caching behind the Go service.

The BFF should not become a second source of truth and should not duplicate the
Valkey cache substrate in TypeScript for phase 2. The safer option is a small,
bounded, optional edge response cache for selected public read routes.

## Decision

Add an optional in-process BFF response cache for low-risk public read routes.

- The cache is disabled by default.
- The cache is per public-app process/pod and is not durable.
- Entries have explicit route-level TTLs and a bounded max-entry count.
- Concurrent identical misses are coalesced so one gRPC request serves the
  burst.
- Cached values are cloned on read/write to prevent caller mutation leaks.
- Errors are not cached.
- Metrics are low-cardinality and label only cache namespace and result.
- Route keys may include canonical request/location attributes, but metrics and
  logs must not expose raw keys or user-linked identifiers.
- Route keys must include the low-cardinality data-plane dimension (`local` or
  `cloud`) from `X-Pulse-Data-Plane`, falling back to
  `PULSE_PLATFORM_DATA_PLANE`, so local-edge cloud-data sessions cannot reuse
  local-mode BFF cache entries.
- Relative-day routes must include the resolved local-day bucket in their cache
  key, or bypass this edge cache when the local day cannot be derived.

Weather forecast and yesterday-verification responses are the first BFF cache
pilot because they are read-heavy, already backed by Go/Valkey caching, and can
tolerate short edge TTLs.

## Consequences

### Positive

- Reduces Node-to-Go gRPC fanout during weather refresh bursts.
- Keeps BFF cache behavior easy to disable or tune per environment.
- Avoids adding Valkey credentials or a second distributed cache client to the
  public Node process.

### Tradeoffs

- Cache reuse is per pod; multi-pod deployments still rely on the Go/Valkey
  cache layer for shared reuse.
- Short TTLs can serve slightly stale weather responses from the BFF edge.
- Additional route caches need explicit review for sensitivity, freshness, and
  invalidation behavior.

## Follow-ups

- Consider BFF edge caching for low-risk BFF aggregates only after route-level
  tests and metrics exist.
- Keep sensitive/provider/session material out of the BFF response cache.
