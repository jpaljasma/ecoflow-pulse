# Backend Go Memory — Auth Profile Entry Routing

## Current focus

- Extend the control plane from device ownership only into current-user bootstrap/profile preferences and enforce authz semantics consistently at Go boundaries.
- Hydrate enough trusted social-profile data from brokered OIDC claims to keep the user record correct without treating mutable fields as identity.

## Files to inspect first

- `proto/pulse/controlplane/v1/control_plane.proto`
- `cmd/ecoflow-grpc-api/controlplane_service.go`
- `cmd/ecoflow-grpc-api/main.go`
- `internal/controlplane/store.go`
- `internal/controlplane/store_postgres.go`
- `deploy/db/migrations/000001_m1_control_plane_schema.up.sql`

## Next step

- Add profile fields and current-user RPCs, including trusted social-profile hydration (`email_verified`, `given_name`, `family_name`, `avatar_url`, `locale`), then harden unauthorized read semantics where detail/history/realtime paths still rely on looser behavior. Public HTTP auth failures should land as `401`; authenticated role failures stay `403`.
