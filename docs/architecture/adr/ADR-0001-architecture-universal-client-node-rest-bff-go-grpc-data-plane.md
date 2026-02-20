# ADR-0001: Architecture — Universal Client + Node REST BFF + Go gRPC Data Plane

**Status:** Accepted  
**Date:** 2026-02-20

## Context
EcoFlow Pulse is a real-time monitor of streaming IoT telemetry that must run on Web/iOS/Android and scale from early adoption to large fleets. The team is initially a single operator/developer, so operational simplicity is critical. The system must support:
- Real-time dashboards (low latency)
- Historical queries and comparisons
- Durable replay (MQTT is not queryable historically)
- A clean path to swap components for managed services later

The repo already contains Go ingestion/runtime components and an Expo universal client.

## Decision
Adopt a multi-tier architecture:
- **Client:** Expo universal app (Web/iOS/Android)
- **Public API:** Node **REST BFF** (product flows, content/upsell orchestration)
- **Internal API/Data Layer:** Go services exposing **gRPC**
- **Realtime:** dedicated **WebSockets Gateway**
- **Data plane:** event-driven ingestion + projections + read models

Data flow pattern:
**Ingestion → normalization/derivation → projection/read models → UI rendering**

## Consequences
### Positive
- Clear separation of concerns and independent scaling per layer
- gRPC is efficient for internal service boundaries
- REST BFF keeps the public surface stable and product-friendly
- WebSockets gateway isolates realtime concerns (backpressure, downsampling)

### Tradeoffs
- More moving parts than a monolith
- Requires strong contracts (protobuf) and observability early

### Follow-ups
- Standardize protobuf versioning rules and compatibility policy
- Implement end-to-end authz enforcement in both REST and gRPC boundaries
