# ADR-0012: Control-plane schema — UUIDv7 IDs and UTC app-managed timestamps

**Status:** Accepted  
**Date:** 2026-02-21  
**Owners:** Jaan

## Context
M1 introduces control-plane authorization tables (`users`, `devices`, `user_devices`).
Without explicit conventions, identifier generation and timestamp ownership can drift
between services and database defaults, creating inconsistent behavior across APIs
and migrations.

## Decision
For control-plane schema and future relational tables:

1. All IDs must be `UUID` with `uuidv7()` defaults (PostgreSQL native UUIDv7).
2. `users.keycloak_subject` is required and globally unique.
3. `users.email` remains nullable, indexed, and non-unique.
4. `devices.ecoflow_sn` is required, globally unique, and canonical for EcoFlow device identity.
5. `user_devices` keeps composite primary key `(user_id, device_id)` and role check constraints.
6. `created_at` and `updated_at` are always:
   - stored as `TIMESTAMPTZ`,
   - UTC semantics,
   - application-managed (no DB default timestamp writes or trigger-owned mutation).

## Consequences
### Positive
- Stable, sortable identifiers across services and storage.
- Explicit ownership of write timestamps in service code.
- Clear authorization linkage model for viewer/admin roles.

### Tradeoffs
- Application code must always provide timestamp values.
- UUID-based foreign keys require consistent type handling in all services.

## Follow-ups
- [ ] Apply these rules to all new M1+ control-plane tables.
- [ ] Add service-level validation/tests that enforce UTC timestamp writes.
