# Pulse Platform (Node REST BFF)

Public-facing Node REST BFF that validates JWTs and forwards history queries to the internal Go gRPC API.

Authenticated history endpoints are rate-limited per client IP to bound abuse and satisfy CodeQL security requirements.

The BFF can optionally use a bounded in-process response cache for selected
public read routes. This is an edge cache only; durable/shared cache semantics
remain in the Go/Valkey cache layer.

## Routes

- `GET /healthz`
- `GET /api/v1/weather/forecast`
- `GET /api/v1/weather/yesterday`
- `GET /api/v1/devices/:deviceId/history`
- `GET /api/v1/devices/:deviceId/history/compare`
- `GET /api/v1/devices/:deviceId/history/solar`
- `GET /api/v1/history/solar/fleet`
