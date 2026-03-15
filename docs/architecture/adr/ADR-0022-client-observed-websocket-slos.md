# ADR-0022: Client-Observed WebSocket SLIs for Realtime Reliability

**Status:** Accepted  
**Date:** 2026-03-15  
**Owners:** Platform / Realtime / Universal App  
**Related:** [ADR-0008-realtime-delivery-websockets-gateway-with-backpressure-downsampling.md](./ADR-0008-realtime-delivery-websockets-gateway-with-backpressure-downsampling.md), [ADR-0021-client-observed-rest-slos.md](./ADR-0021-client-observed-rest-slos.md), [../README.md](../README.md)

---

## Context
Pulse realtime behavior is user-facing. A healthy websocket system is not defined only by whether the gateway process is up or whether server-side websocket handshakes succeed.

From the client perspective, realtime reliability includes:
- how often authenticated connections become usable,
- how quickly reconnect and resubscribe complete after interruption,
- whether auth/session issues block realtime,
- whether live data goes stale even when the socket still exists.

Server-side websocket metrics remain necessary, but they do not fully capture browser-visible failures such as:
- stale live views after reconnect churn,
- auth-required states caused by client session drift,
- browser disconnect/reconnect loops,
- silent freshness gaps where the client is connected but no usable live data arrives.

## Decision
- For user-facing realtime reliability, the primary SLI source is client-observed websocket telemetry emitted by first-party clients.
- The initial scope is the universal app browser/runtime path; native clients should adopt the same low-cardinality event model once validated.
- Server-side websocket metrics remain required for debugging and capacity analysis, but they are supporting diagnostics rather than the primary source for user-perceived realtime SLOs.
- Client websocket telemetry must use canonical low-cardinality labels only. No device serials, provider IDs, user IDs, or raw URLs may appear in metric labels.

## Initial SLI set
The first client-observed websocket SLI set must include:

- **Availability / success**
  - connection attempts vs successful usable connections
  - reconnect attempts vs successful reconnects
- **Latency**
  - initial connect latency
  - reconnect latency
- **Errors**
  - auth-required / auth-failed outcomes
  - unexpected disconnect reasons
- **Freshness context**
  - stale-data transitions
  - stale duration until recovery

## Rationale
This keeps websocket reliability aligned with actual user experience:
- a connected socket that never becomes useful is not healthy,
- a reconnecting client with stale data is not fully available,
- auth-related realtime failures should be visible from the client perspective, not only inferred from gateway logs.

## Consequences
### Positive
- Realtime reliability is measured from the user’s point of view.
- Silent live-data failures become visible.
- Reconnect and freshness regressions are easier to catch early.

### Tradeoffs
- We must maintain a stable client websocket event schema.
- Freshness metrics require disciplined low-cardinality reporting to avoid noise.
- Server-side and client-side websocket views must be read together during incidents.

## Guardrails
- Canonical labels only for phase, outcome, disconnect reason, and freshness state.
- No per-device or per-user labels in Prometheus metrics.
- Best-effort reporting only; client metrics must never block the live transport.
- Dashboards should present client-observed websocket SLIs as primary and server-side gateway metrics as supporting context.

## Implementation plan
1. Instrument the universal telemetry engine for connect, reconnect, auth-required, disconnect, and freshness transitions.
2. Add a public-edge ingestion endpoint that converts those events into Prometheus metrics.
3. Build Grafana panels for websocket availability, latency, error mix, and freshness context.
4. Extend the event schema to native clients after browser validation.

## Acceptance criteria
- Grafana can show client-observed websocket connect/reconnect success and latency.
- Grafana can show client-observed disconnect/auth failure mix.
- Grafana can show client-observed freshness degradation context.
- Architecture docs and repo rules explicitly recognize client-observed websocket SLIs as the primary source for user-facing realtime reliability.
