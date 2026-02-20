# ADR-0004: Cache — Valkey (Redis-Compatible) with Replication + Sentinel

**Status:** Accepted  
**Date:** 2026-02-20

## Context
A hot store is needed for:
- last-known device snapshots
- fast connect for realtime UI
- rate limiting buckets and ephemeral session state

Redis licensing changes created uncertainty. We prefer open-source-first with compatibility and a swap path to hosted later.

## Decision
Use **Valkey** (Redis-compatible) deployed as:
- **replication + Sentinel** (cheap HA feel)
- 1 primary + 2 replicas (3 pods total) in dev/local for realistic failover
- use Kubernetes Services for stable endpoints

Avoid Valkey/Redis Cluster mode in v1 (sharding not required at expected scale).

## Consequences
### Positive
- Open-source posture and drop-in client compatibility
- Realistic failover behavior in small environments
- Minimal complexity vs cluster/sharding

### Tradeoffs
- Sentinel adds some operational components (but manageable via Helm)
- Future very-large-scale may require cluster mode or a managed cache

### Follow-ups
- Define keyspace conventions and TTL rules
- Add metrics for replication health and role changes
