# ADR-0005: Databases — Postgres + TimescaleDB for Control Plane + Rollups

**Status:** Accepted  
**Date:** 2026-02-20

## Context
We need:
- relational control-plane data (users, devices, entitlements, content)
- time-series rollups for history queries and comparisons
- low operational burden and straightforward migrations

## Decision
Use:
- **Postgres** for control-plane data
- **TimescaleDB** (Postgres extension) for time-series rollups and history

Operate Postgres via **CloudNativePG** in Kubernetes.

Retention policy:
- minute rollups: **90 days**
- hourly rollups: **3 years**
- daily rollups: **3 years**

## Consequences
### Positive
- Unified SQL surface area and tooling
- Good fit for early scale and rapid iteration
- Easier operations vs introducing ClickHouse day 1

### Tradeoffs
- At very large scale, ClickHouse may become attractive for heavy analytics
- Requires thoughtful schema and indexing for comparison queries

### Follow-ups
- Define hypertables + indexes and rollup job cadence
- Decide when/how to introduce ClickHouse (future ADR)
