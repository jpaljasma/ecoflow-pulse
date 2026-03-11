# How-to: Configure Environment and Credentials

Use this guide to configure Ecoflow-Pulse for local development or operations.

## 1. Create `.env`

```bash
cat > .env <<'EOF'
ECOFLOW_ACCESS_KEY=your_access_key
ECOFLOW_SECRET_KEY=your_secret_key
ECOFLOW_ENV=prod
ECOFLOW_BASE_URL=https://api.ecoflow.com
EOF
```

## 2. Optional Local Noop-Auth User Mapping

When running the local platform without full auth wired, set the user subject
that the Node BFF should use to resolve owned devices:

```bash
PULSE_PLATFORM_DEV_SUBJECT=<your-user-subject>
```

Keep your real device serials and provider credentials in `.env` or shell
environment only; do not commit them into tracked config or docs.

## 3. Optional Seed Inputs

For explicit local provider seeding:

- `ECOFLOW_DEV_ACCESS_KEY`
- `ECOFLOW_DEV_SECRET_KEY`
- `ECOFLOW_DEV_USER_SUBJECT`
- `ECOFLOW_DEV_USER_EMAIL`
- `ECOFLOW_DEV_SEED_SNS`

## 4. Verify API Connectivity

```bash
go run ./cmd/ecoflow-smoke
```

If smoke passes, configuration is valid enough for API access.

## 5. Boot the Local Platform (optional)

```bash
make dev-up
make dev-web-deploy
```

## 6. Enable Expo Keycloak PKCE Login (optional)

To enable the Settings -> Authentication (PKCE) card in the universal app:

```bash
EXPO_PUBLIC_OIDC_ISSUER_URL=https://<keycloak-host>/realms/pulse
EXPO_PUBLIC_OIDC_CLIENT_ID=pulse-expo
EXPO_PUBLIC_OIDC_AUDIENCE=pulse-api
EXPO_PUBLIC_OIDC_SCOPES="openid profile email offline_access"
```

Then run:

```bash
npm run -w apps/universal web
```
