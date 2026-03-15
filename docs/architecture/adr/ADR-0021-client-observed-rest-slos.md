# ADR-0021: Client-Observed REST SLIs for User-Facing Reliability

**Status:** Accepted  
**Date:** 2026-03-15  
**Owners:** Platform / Observability / Universal App  
**Related:** [ADR-0001-architecture-universal-client-node-rest-bff-go-grpc-data-plane.md](./ADR-0001-architecture-universal-client-node-rest-bff-go-grpc-data-plane.md), [ADR-0007-authentication-keycloak-oidc-with-social-login-jwt-validated-at-every-boundary.md](./ADR-0007-authentication-keycloak-oidc-with-social-login-jwt-validated-at-every-boundary.md), [ADR-0020-authenticated-entry-routing-profile-and-device-authorization.md](./ADR-0020-authenticated-entry-routing-profile-and-device-authorization.md), [../README.md](../README.md)

---

## Context
Server-side HTTP metrics are necessary, but they are not sufficient to represent what users actually experienced.

For user-facing REST paths in Pulse:
- the universal app may see transport failures, browser-origin problems, TLS/redirect issues, stale-session failures, or client-side retry behavior that never appears as a successful server request,
- server-side request metrics can overstate availability because they only describe traffic that reached the public edge,
- Google SRE guidance emphasizes that user-facing reliability should be measured from the client perspective whenever practical, because that is the closest representation of actual service experience.

Pulse already exposes public-edge metrics and dashboards for auth/profile and server-side request behavior. Those remain useful for diagnostics, but they should not be the only basis for user-facing REST SLOs.

## Decision
- For user-facing REST flows, the primary SLI source is client-observed request telemetry emitted by first-party clients.
- The initial scope is the universal app browser/runtime path; native clients should adopt the same event schema when this path is stable.
- Server-side REST metrics remain required, but are supporting diagnostics rather than the primary SLI source for user-perceived availability and latency.
- Client-observed REST telemetry must be emitted with canonical low-cardinality route labels only; raw URLs, query strings, device serials, and other high-cardinality identifiers are forbidden.
- Client-observed REST SLO dashboards must be request-based and start with:
  - availability,
  - latency,
  - error rate / failure mix.

## SLI model
For user-facing REST endpoints:

- **Availability SLI**
  - good = client-observed requests with successful completion
  - total = all client-observed requests for the same canonical route/method scope
- **Latency SLI**
  - measured from the client-observed duration of successful requests
  - dashboards must include at least `P95` and `P99`
- **Error SLI / supporting error views**
  - explicit client-observed failures, including:
    - HTTP failures,
    - network/transport failures,
    - client-side request/response handling failures

Throughput is supporting context only; it is not itself an objective.

## Rationale
This gives Pulse a more truthful reliability model for user-facing REST paths:
- transport and browser-path failures are visible,
- stale client auth/session behavior is visible,
- dashboard availability reflects the user experience rather than only server reachability,
- server-side metrics still help diagnose where failures occurred after the fact.

## Consequences
### Positive
- User-facing SLOs better match real browser experience.
- Browser-only failures become visible.
- Availability and latency claims are harder to overstate.

### Tradeoffs
- We must maintain a stable client metrics schema and ingestion path.
- Best-effort client metric reporting can still miss some terminal failures, so server-side metrics remain important as a secondary diagnostic view.
- Route classification discipline becomes mandatory to avoid cardinality blowups.

## Guardrails
- Canonical route labels only.
- No raw query strings in metric labels.
- No serial numbers, provider device IDs, emails, or other sensitive identifiers in client metric payloads.
- Client metric submission must be best-effort and must never block or materially delay user traffic.
- Server-side dashboards should show client-observed SLIs as primary and server-side HTTP metrics as supporting context when both exist for the same user-facing flow.

## Implementation plan
1. Define a canonical client REST metric event schema with low-cardinality route labels.
2. Instrument the universal app request layer to emit best-effort client-observed request events.
3. Add a public-edge ingestion endpoint that converts those events into Prometheus metrics.
4. Build Grafana dashboards around request-based availability, latency, and error views.
5. Extend the same schema to native clients after the browser path is validated.

## Acceptance criteria
- User-facing REST dashboards can show client-observed availability, latency, and error views by canonical route.
- Client metrics include transport failures that server-only request metrics cannot observe.
- The architecture docs and repo rules explicitly treat client-observed REST SLIs as the primary source for user-facing reliability.
