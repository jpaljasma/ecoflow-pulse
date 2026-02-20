# Config 05 — Authentication, authorization, and request authority

This config ensures REST and gRPC can independently verify requests are legitimate.

---

## Identity provider
- **Keycloak** (OIDC)
- Social: Google + Facebook
- Client flow: Authorization Code + PKCE

---

## Request authority model
- Client → Node REST: JWT access token
- Node validates JWT via JWKS
- Node → Go gRPC: forward same user JWT in metadata
- Go validates JWT again and enforces authz

---

## Authorization schema
Tables:
- `users`
- `devices`
- `user_devices(user_id, device_id, role)`

Roles:
- `viewer`
- `admin`

Enforce authorization inside Go query boundaries.

---

## WebSockets security
- Authenticate WS connection with JWT at handshake (or first message)
- Authorize per-device subscriptions
- Clients refresh tokens via Keycloak and reconnect WS if needed

---

## Service-to-service security
Recommended:
- **Linkerd mTLS** for in-cluster encryption + identity
Optional early: enable after M1 to keep M0 simple.

---

## Secrets
- Staging/Prod: External Secrets Operator + GCP Secret Manager
- Local/Dev: SOPS (age) or dev-only secrets

---

## Early security checklist
- TLS at ingress
- JWT validation at REST/gRPC/WS boundaries
- Least-privilege service accounts
- Audit logs for device sharing and admin actions
- Retention policies enforced technically (not docs-only)
