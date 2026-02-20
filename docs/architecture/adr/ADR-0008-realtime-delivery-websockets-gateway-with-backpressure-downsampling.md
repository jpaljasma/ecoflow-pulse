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

## Consequences
### Positive
- Isolates realtime complexity from the REST BFF and query APIs
- Prevents slow consumers from destabilizing the system
- Provides consistent UX expectations during load

### Tradeoffs
- Another service to operate
- Requires thoughtful client reconnection/subscription logic

### Follow-ups
- Define WS protocol (subscribe/unsubscribe, filters, ack strategy)
- Implement client reconnect/resubscribe and UX states
