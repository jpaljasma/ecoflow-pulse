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

By default, smoke now also opens a shared MQTT probe across all discovered
devices, prints status updates, and waits until live data arrives for each
device or you interrupt it.

For the old API-only quick check:

```bash
go run ./cmd/ecoflow-smoke -mqtt=false
```

If the API-only smoke passes, configuration is valid enough for basic API
access.

## 5. Boot the Local Platform (optional)

```bash
make dev-up
make dev-web-deploy
```

## 6. Enable Expo Keycloak PKCE Login (optional)

To enable the `/login` Google sign-in flow in the universal web app:

```bash
EXPO_PUBLIC_OIDC_ISSUER_URL=https://localhost/realms/pulse
EXPO_PUBLIC_OIDC_CLIENT_ID=pulse-universal-app
EXPO_PUBLIC_OIDC_SCOPES="openid profile email offline_access"
```

These values are read at web-build time. For local k3d, keep them in `.env` and use:

```bash
make dev-deploy
```

Notes:

- Leave `EXPO_PUBLIC_API_URL` and `EXPO_PUBLIC_WS_URL` unset for the normal `https://localhost` local edge path unless you are intentionally overriding the browser targets.
- If a build/deploy pipeline injects those values as empty strings, the web runtime now treats them as unset and falls back to secure localhost defaults.
- Local sign-in + realtime telemetry also depend on the shared ingress routing:
  - `/realms` and `/resources` -> Keycloak
  - `/ws` -> realtime gateway
  - `/` -> public app

## 7. Enable local Keycloak Google broker (optional)

Keep local Google broker credentials in `.env`, not in committed Helm values:

```bash
KEYCLOAK_SOCIAL_GOOGLE_CLIENT_ID=<google-client-id>
KEYCLOAK_SOCIAL_GOOGLE_CLIENT_SECRET=<google-client-secret>
```

Then rerun:

```bash
make platform-up
make platform-wait
make dev-deploy
```
