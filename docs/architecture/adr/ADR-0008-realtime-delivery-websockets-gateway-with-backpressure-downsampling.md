# ADR-0008: Realtime Delivery — WebSockets Gateway with Backpressure & Downsampling

**Status:** Accepted  
**Date:** 2026-02-20

## Context
Realtime dashboards need low-latency updates (target ~250ms) but must remain stable under:
- slow clients
- mobile background/foreground transitions
- network jitter
- bursts in telemetry

## Decision
Create a dedicated **WebSockets Gateway** that:
- authenticates connections with JWT
- authorizes per-device subscriptions (owner/guest RBAC)
- sends snapshot on connect (from Valkey), then streams deltas (from NATS)
- enforces backpressure with a degradation ladder:
  250ms → 500ms → 1s → key-metrics-only → paused

The same gateway also owns the global-admin realtime log stream. Browsers use
the existing `/ws` edge path and never connect to NATS or JetStream directly.
Admin clients send `logs_subscribe` / `logs_unsubscribe`; the gateway verifies
the global `admin` role from the websocket JWT, replays a bounded recent
JetStream tail, then forwards live redacted `log_entry` messages plus
`logs_status` and `logs_replay_done` lifecycle events. In noop auth mode this
log stream remains disabled unless an explicit local-only development flag is
set.

## Consequences
### Positive
- Isolates realtime complexity from the REST BFF and query APIs
- Prevents slow consumers from destabilizing the system
- Provides consistent UX expectations during load
- Keeps operational log drill-down on the same authenticated realtime edge,
  with redaction enforced before entries reach the admin UI

### Tradeoffs
- Another service to operate
- Requires thoughtful client reconnection/subscription logic
- Admin log filtering needs BFF/gRPC lookup support so sensitive serial/email
  selections resolve to internal IDs before websocket subscription

### Follow-ups
- Define WS protocol (subscribe/unsubscribe, filters, ack strategy)
- Implement client reconnect/resubscribe and UX states
