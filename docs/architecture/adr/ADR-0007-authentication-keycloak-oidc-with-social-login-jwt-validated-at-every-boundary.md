# ADR-0007: Authentication — Keycloak OIDC with Social Login; JWT Validated at Every Boundary

**Status:** Accepted  
**Date:** 2026-02-20

## Context
We need social login and strong request authority:
- clients authenticate with Google/Facebook
- REST and gRPC must both verify authenticity (no trust-by-proxy)
- authorization must ensure users only see their devices; sharing later

## Decision
Use **Keycloak** as the OIDC provider with Google/Facebook identity providers.
- Client: Authorization Code + PKCE
- Node REST BFF validates JWT via JWKS
- Node forwards the same user JWT to Go gRPC in metadata
- Go services validate JWT again and enforce authz via `user_devices` mapping (viewer/admin)

## Consequences
### Positive
- Centralized identity with standard protocols
- Defense-in-depth validation across boundaries
- Authorization enforced where the data is (Go layer)

### Tradeoffs
- Operating Keycloak adds a stateful component
- Requires careful token/session handling in clients and WS

### Follow-ups
- Define token lifetimes and refresh strategy
- Add audit logging for device sharing/admin actions
