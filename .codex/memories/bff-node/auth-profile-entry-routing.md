# BFF Node Memory — Auth Profile Entry Routing

## Current focus

- Add a clean current-user bootstrap/update surface without weakening the existing JWT boundary.

## Target endpoints

- `GET /api/v1/me`
- `PATCH /api/v1/me`

## Next step

- Reuse the existing auth pre-handler, add strict request validation, and keep error mapping aligned with gRPC status codes.

