# Pulse Platform (Node REST BFF)

Public-facing Node REST BFF that validates JWTs and forwards history queries to the internal Go gRPC API.

Authenticated history endpoints are rate-limited per client IP to bound abuse and satisfy CodeQL security requirements.

## Routes

- `GET /healthz`
- `GET /api/v1/devices/:deviceId/history`
- `GET /api/v1/devices/:deviceId/history/compare`
