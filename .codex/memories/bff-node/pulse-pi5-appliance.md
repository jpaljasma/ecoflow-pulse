# BFF Node Memory - Pulse Pi 5 Appliance

## Current focus

- Stay ready for appliance setup/auth API adjustments if Phase 1 or product
  review needs them.

## Files to inspect first

- `apps/pulse-platform`
- `docs/how-to/configure-keycloak-social-providers-local.md`
- `docs/reference/configuration.md`

## Decisions made

- Keycloak remains the local auth authority.
- Social providers are optional per appliance install; local username/password
  must be sufficient.
- Public URLs should assume LAN-only access unless a split-horizon real domain
  is explicitly configured.

## Open risks

- Social OAuth redirect rules can make `home.arpa` unsuitable for some
  providers, so setup flows must not require social login.
- BFF route config can drift if local hostname/TLS assumptions are duplicated.

## Next step

- Wait for Phase 1 setup requirements, then add only the minimum BFF changes
  needed for first-user and local-domain behavior.
