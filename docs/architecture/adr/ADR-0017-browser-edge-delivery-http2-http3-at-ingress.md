# ADR-0017: Browser Edge Delivery — HTTP/2 by Default, HTTP/3 When QUIC Is Exposed at Ingress

**Status:** Accepted  
**Date:** 2026-03-08  
**Owners:** Jaan  
**Related:** ADR-0002, ADR-0011

---

## Context

EcoFlow Pulse serves a browser-facing public edge through the Node BFF/public
app and websocket gateway path, but the transport and caching behavior should be
owned by the ingress/public edge instead of the Node runtimes themselves.

The platform already standardizes on:
- `ingress-nginx`
- `cert-manager`
- portable Kubernetes ingress patterns

We need an explicit browser-edge policy for:
- browser-facing HTTP/2
- browser-facing HTTP/3/QUIC where the ingress runtime actually supports it
- HTML/static asset cache policy
- preload / `103 Early Hints`
- optional `preconnect` / `dns-prefetch`
- local trusted TLS behavior

### Requirements / Goals
- Keep browser-facing transport policy at the ingress/public edge.
- Make HTTP/2 the default browser-facing transport under TLS.
- Support HTTP/3 when the ingress runtime exposes QUIC and UDP 443.
- Avoid obsolete HTTP/2 server push.
- Keep local TLS/browser validation reproducible.

### Non-goals
- Move browser-edge transport logic into the Node app processes.
- Guarantee every local client tool can speak HTTP/3.

---

## Options considered
### Option A: Keep browser delivery mostly at the Node app layer
**Pros**
- Fewer ingress changes.

**Cons**
- Transport/caching behavior drifts from real edge behavior.
- Harder to keep parity with cloud ingress.

### Option B: Own browser delivery at ingress/public edge (chosen)
**Pros**
- Matches real deployment shape.
- Keeps HTTP version negotiation, redirects, compression, and cache policy in
  the right layer.
- Keeps the Node app focused on app semantics and response hints.

**Cons**
- Requires ingress/controller-specific configuration.
- HTTP/3 enablement must include UDP 443 exposure and QUIC listener wiring.

---

## Decision

- Browser-facing TLS terminates at ingress/public edge.
- Browser-facing HTTP/2 is the default transport once TLS ingress is enabled.
- Browser-facing HTTP/3 is supported only when all of the following are true:
  - the ingress runtime is built with HTTP/3/QUIC support,
  - UDP 443 is exposed at the edge,
  - the ingress server block listens with QUIC,
  - `Alt-Svc` advertises HTTP/3 availability.
- We will not use HTTP/2 server push.
- We will use preload / `103 Early Hints` and optional `preconnect` only where
  they materially help first render.
- Local trusted TLS should be CA-based, not a bare self-signed leaf cert.

---

## Rationale

This keeps ownership aligned with the actual deployment boundary:
- ingress owns protocol negotiation, redirects, and edge compression,
- the app can still emit useful `Link` hints and cache policy,
- local development can exercise the same edge behavior without pretending the
  Node server itself is an HTTP/3 edge,
- CA-based local TLS is the practical way to make browsers and `curl` trust the
  local endpoint.

---

## Consequences
### Positive
- Browser edge behavior is explicit and portable.
- HTTP/2 and HTTP/3 support are defined by ingress capability, not wishful app
  behavior.
- Local browser validation is closer to real ingress behavior.

### Negative / Tradeoffs
- HTTP/3 requires more than a single ingress annotation.
- Verification depends on client support (`curl --http3`, browser devtools).

### Risks & mitigations
- **Risk:** ingress validated resources fail during one-pass local bringup.  
  **Mitigation:** wait for ingress-nginx and cert-manager webhook/controller
  readiness before the second Helm reconcile.
- **Risk:** local TLS trust fails if a self-signed leaf cert is trusted
  directly.  
  **Mitigation:** mint a local CA issuer and trust the CA certificate instead.

---

## Implementation plan
1. Configure ingress/public-edge TLS, redirects, compression, preload, and cache policy.
2. Enable QUIC/HTTP/3 only when UDP 443 and QUIC listeners are both present.
3. Keep local CA/trust automation documented and reproducible.
4. Record verification steps for HTTP/2/HTTP/3 separately.

### Rollout / Migration
- Local: enable ingress + TLS + optional HTTP/3 path on `https://localhost`.
- Dev: keep the ingress path configurable; require explicit issuer/domain wiring
  before claiming full HTTPS/HTTP/3 validation.

### Observability
- logs:
  - ingress-nginx config reload and ingress sync events
- verification:
  - `curl -I https://...` for HTTP/2/default headers
  - browser devtools or `curl --http3` for HTTP/3 confirmation

### Security / Compliance
- Enforce HTTP→HTTPS redirect when TLS is enabled.
- Use trusted local CA flow for localhost rather than bypass flags such as
  `curl -k`.

---

## Acceptance criteria
- Browser-facing HTTP/2 is available under TLS ingress.
- HTTP/3 is advertised only when QUIC listener + UDP 443 exposure are both
  configured.
- HTML/static cache policy and preload hints are owned by the browser edge path.
- Local trusted TLS works without disabling certificate verification.

---

## Follow-ups
- [x] Add an HTTP/3-capable client verification step to local automation when a
  suitable tool is available in the repo/toolchain.
  - [x] Added `make edge-verify-http3-local` with explicit preflight for `curl -V` `Features: HTTP3`, local HTTP/3 service presence, `Alt-Svc` advertising, and `curl --http3-only` verification.
  - [x] Validation evidence (2026-03-10): `make edge-verify-http3-local` failed fast with `curl is installed, but the linked libcurl lacks HTTP/3 support; install an HTTP/3-capable curl before running this check.`
- [ ] Decide whether dev/GKE should enable HTTP/3 by default or keep it opt-in
  per environment.
