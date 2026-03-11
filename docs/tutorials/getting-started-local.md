# Tutorial: Get Started Locally

This tutorial walks through a first local run of EcoFlow Pulse using the
supported local platform and universal web app flow.

## Outcome

By the end you will:

- run unit tests,
- run a smoke API check,
- boot the local k3d stack,
- open the web app against the local platform.

## Prerequisites

- Go installed (matching project `go.mod` toolchain/runtime support).
- Node.js + npm installed.
- Docker Desktop running.
- `k3d`, `helm`, and `kubectl` installed.
- EcoFlow API credentials.

## 1. Configure Environment

Create `.env` in the repository root:

```bash
cat > .env <<'EOF'
ECOFLOW_ACCESS_KEY=your_access_key
ECOFLOW_SECRET_KEY=your_secret_key
ECOFLOW_ENV=prod
PULSE_PLATFORM_DEV_SUBJECT=your_user_subject
EOF
```

## 2. Validate Build and Tests

```bash
go test ./...
```

Expected result: all tests pass.

## 3. Run API Smoke Check

```bash
go run ./cmd/ecoflow-smoke
```

Expected result: device list and API connectivity details print successfully.

## 4. Boot the Local Platform

```bash
make dev-up
```

Expected result: local k3d platform and services come up successfully.

## 5. Open the Universal Web App

```bash
make dev-web-deploy
```

Then open `https://localhost` and verify your devices load through the local
platform. In local noop-auth mode, `PULSE_PLATFORM_DEV_SUBJECT` should match
the `users.keycloak_subject` value seeded in the control plane.
