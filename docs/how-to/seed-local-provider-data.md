# How-To: Seed Local Provider Data

Use this when you need explicit local seed data for M1 provider integration
tables (`provider_credentials`, `provider_devices`) in k3d/CNPG.

This is explicit-only (no auto-seed on startup).

## Prerequisites

- local platform is up (`make dev-up`)
- migrations are applied (`make db-migrate-up-local`)
- `.env` (or shell env) contains:
  - `ECOFLOW_DEV_ACCESS_KEY`
  - `ECOFLOW_DEV_SECRET_KEY`

## Seed with defaults

```bash
make db-seed-dev-local
```

Defaults:

- user subject/email: `jpaljasma@gmail.com`
- serials:
  - `R351ZABAPH331057`
  - `Y711ZABA9H2P0294`

## Override seed inputs

```bash
make db-seed-dev-local \
  DB_SEED_USER_SUBJECT=me@example.com \
  DB_SEED_USER_EMAIL=me@example.com \
  DB_SEED_SERIALS=R351ZABAPH331057,Y711ZABA9H2P0294
```

## What gets upserted

- `users`
- `devices`
- `user_devices` (`role=admin`)
- `provider_credentials` (write-only secret columns; masked access key output)
- `provider_devices` (`is_active=true`, `ingest_desired_state=active`)

The command is idempotent and safe to rerun.
