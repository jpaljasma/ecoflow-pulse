# ADR-0019: Split Energy RPCs Into a Dedicated gRPC EnergyService

**Status:** Accepted  
**Date:** 2026-03-11  
**Owners:** Platform / API  
**Supersedes:** ADR-0018 (service-boundary non-goal only)  
**Related:** [ADR-0013-grpc-server-baseline-go-high-throughput.md](./ADR-0013-grpc-server-baseline-go-high-throughput.md), [ADR-0018-rollups-persist-explicit-energy-buckets.md](./ADR-0018-rollups-persist-explicit-energy-buckets.md), [../README.md](../README.md)

---

## Context
Energy APIs now have materially different behavior and scaling pressure than the core telemetry RPCs:
- `GetSnapshot` and `Subscribe` are latency-sensitive live-read paths,
- historical `QueryRollupRange` and `CompareRollupRange` can serve both general telemetry history and heavier energy-specific windows,
- `GetEnergyDashboard` mixes rollup queries, fleet aggregation, archive-backed PV history enrichment, and local-calendar comparison logic.

This creates an operational mismatch inside `TelemetryService`:
- energy requests can have higher latency and CPU cost than live snapshot/realtime RPCs,
- scaling telemetry live reads and scaling energy/history workloads are not the same problem,
- release cadence for energy-specific logic should not require bundling all telemetry RPCs into one service boundary.

The current architecture already standardizes the internal gRPC baseline through ADR-0013. That means splitting service boundaries does not require inventing a second transport stack as long as the same shared `internal/grpcserver` builder, interceptor order, transport tuning, limits, and lifecycle rules are preserved.

ADR-0018 intentionally treated a separate Energy service as a non-goal for the explicit-rollup decision. That was acceptable while the Energy dashboard was new. It is no longer the preferred service boundary once energy/history traffic and tuning needs are diverging from core telemetry snapshot/realtime behavior.

## Decision
- Create a dedicated internal gRPC `EnergyService`.
- Move energy-facing RPCs out of `TelemetryService` and into `EnergyService`.
- The initial `EnergyService` scope includes:
  - `QueryRollupRange`
  - `CompareRollupRange`
  - `GetEnergyDashboard`
- `TelemetryService` remains focused on live telemetry snapshot/streaming responsibilities:
  - `GetSnapshot`
  - `Subscribe`
- `EnergyService` must use the exact same ADR-0013 baseline:
  - shared `internal/grpcserver` server construction,
  - same keepalive params and enforcement policy,
  - same transport tuning and message/header limits,
  - same unary/stream interceptor ordering,
  - same reflection/dev-only and graceful drain behavior.

## Rationale
This split improves operational clarity without changing the platform shape:
- live snapshot/realtime telemetry paths stay isolated from heavier energy/history calls,
- energy/history workloads can be scaled and deployed independently,
- tuning and profiling can target the correct request mix,
- the Node BFF and Expo client contracts stay stable while the internal service boundary becomes cleaner.

Keeping the same ADR-0013 baseline avoids service drift. The split is about deployability and scaling isolation, not about relaxing gRPC performance or safety defaults.

## Consequences
### Positive
- Energy/history APIs can be scaled independently from snapshot/stream APIs.
- Telemetry live-read latency is less likely to be impacted by heavy energy/history requests.
- Energy-specific rollout cadence is cleaner.

### Tradeoffs
- One more registered gRPC service and client surface to maintain.
- Node BFF client wiring becomes slightly more explicit because it now talks to both telemetry and energy RPC surfaces.
- Proto/service generation and test coverage must be updated carefully to avoid client breakage.

## Implementation plan
1. Add `EnergyService` to the protobuf contract and move energy-facing RPCs under it.
2. Keep message types stable where possible so BFF/UI contract churn stays minimal.
3. Implement a dedicated Go server type for `EnergyService`.
4. Deploy `TelemetryService` and `EnergyService` as separate internal gRPC workloads so they can scale and roll independently while preserving the same ADR-0013 baseline.
5. Update Node gRPC clients and REST handlers to call `EnergyService`.
6. Validate local deploy and end-to-end history/dashboard behavior across both internal services.

## Acceptance criteria
- `TelemetryService` no longer owns energy/history RPCs.
- `EnergyService` serves `QueryRollupRange`, `CompareRollupRange`, and `GetEnergyDashboard`.
- Shared ADR-0013 gRPC tuning/interceptor behavior remains unchanged.
- `EnergyService` is independently deployable/scalable from `TelemetryService`.
- Node BFF and universal `/energy` flows continue working without user-visible contract regressions.
- Local deploy validation confirms end-to-end behavior.
