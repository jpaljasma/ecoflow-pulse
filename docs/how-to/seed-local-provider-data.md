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

- user subject/email: `dev-user@example.com`
- serials:
  - `DEMOD2M00001057`
  - `DEMODPU0000294`

## Override seed inputs

```bash
make db-seed-dev-local \
  DB_SEED_USER_SUBJECT=me@example.com \
  DB_SEED_USER_EMAIL=me@example.com \
  DB_SEED_SERIALS=DEMOD2M00001057,DEMODPU0000294
```

## What gets upserted

- `users`
- `devices`
- `user_devices` (`role=admin`)
- `provider_credentials` (write-only secret columns; masked access key output)
- `provider_devices` (`is_active=true`, `ingest_desired_state=active`)

The command is idempotent and safe to rerun.

## Verify EcoFlow certification path for seeded devices (optional)

After seeding, you can verify the real EcoFlow provider adapter can resolve
MQTT certification for seeded serial numbers:

```bash
ECOFLOW_ADAPTER_INTEGRATION=1 go test ./internal/provideradapter -run TestEcoFlowAdapterGetMQTTCertificationSeededSNsIntegration -count=1 -v
```

Requirements:

- `ECOFLOW_DEV_ACCESS_KEY`
- `ECOFLOW_DEV_SECRET_KEY`
- optional `ECOFLOW_DEV_SEED_SNS` (defaults are used when omitted)
