# Config 05 — Authentication, authorization, and request authority

This config ensures REST and gRPC can independently verify requests are legitimate.

---

## Identity provider
- **Keycloak** (OIDC)
- Social: Google + Facebook
- Client flow: Authorization Code + PKCE

Realm bootstrap conventions:
- Realm is imported via `keycloakConfigCli` in Helm, not manually in UI.
- Realm import config source:
  - ConfigMap: `pulse-platform-keycloak-realm-import`
- Social provider credentials source:
  - Secret: `pulse-platform-keycloak-social-providers`
- Local validation command:
  - `make auth-keycloak-verify-local`

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

Schema conventions:
- IDs are `UUID` with PostgreSQL-native `uuidv7()` defaults.
- `devices.ecoflow_sn` is global and unique.
- timestamps (`created_at`, `updated_at`) are `TIMESTAMPTZ` and application-managed (UTC semantics, no DB-managed timestamp defaults).

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

Social provider secret keys (contract):
- `KEYCLOAK_SOCIAL_GOOGLE_CLIENT_ID`
- `KEYCLOAK_SOCIAL_GOOGLE_CLIENT_SECRET`
- `KEYCLOAK_SOCIAL_FACEBOOK_CLIENT_ID`
- `KEYCLOAK_SOCIAL_FACEBOOK_CLIENT_SECRET`

---

## Early security checklist
- TLS at ingress
- JWT validation at REST/gRPC/WS boundaries
- Least-privilege service accounts
- Audit logs for device sharing and admin actions
- Retention policies enforced technically (not docs-only)
