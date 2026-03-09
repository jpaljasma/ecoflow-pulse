# How to Configure Keycloak Social Providers (Local)

This guide configures the M1 auth realm bootstrap for local k3d.

## Goal

Enable Keycloak realm `pulse` and wire Google/Facebook provider credentials using chart-managed resources.

## Preconditions

- local platform is installed (`make dev-up` or at least `make platform-up`)
- Keycloak is enabled in local platform values (`deploy/env/local/values.platform.yaml`)

## 1) Set provider credentials in local values

Edit `deploy/env/local/values.platform.yaml`:

```yaml
keycloakRealm:
  enabled: true
  realmName: pulse
  google:
    enabled: true
    clientId: "<google-client-id>"
    clientSecret: "<google-client-secret>"
  facebook:
    enabled: true
    clientId: "<facebook-client-id>"
    clientSecret: "<facebook-client-secret>"
```

Notes:
- Keep credentials in local-only values. Do not commit real secrets.
- Realm/provider import uses:
  - ConfigMap `pulse-platform-keycloak-realm-import`
  - Secret `pulse-platform-keycloak-social-providers`

## 2) Apply platform changes

```bash
make platform-up
make platform-wait
```

Local sequencing note:
- `make platform-up` now defers the very first Keycloak install pass until the
  external CNPG database service and bootstrap secret are ready, then reconciles
  Keycloak on the second pass.
- `make platform-wait` now also waits for the Keycloak config-cli bootstrap job
  to reach `Complete` when that job exists.

## 3) Verify realm + providers

```bash
make auth-keycloak-verify-local
```

Expected output includes:

```text
keycloak realm verification passed: realm=pulse, providers=google,facebook
```

## 4) Troubleshooting

Check config-cli job/pod:

```bash
kubectl --context k3d-pulse-local -n pulse-platform get jobs,pods | rg keycloak-config-cli
kubectl --context k3d-pulse-local -n pulse-platform logs job/pulse-platform-keycloak-keycloak-config-cli
```

If image pulls fail locally, ensure legacy Bitnami repositories are used in local values.

## Dev/Staging/Prod note

For non-local environments, manage social provider credentials via External Secrets + cloud secret manager and do not store cleartext credentials in committed values files.
