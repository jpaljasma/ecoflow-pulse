# Frontend Universal Memory - Pulse Pi 5 Appliance

## Current focus

- Stay ready for first-user setup, local auth, and appliance health/product
  states after backend/platform decisions are concrete.

## Files to inspect first

- `apps/universal`
- `apps/universal/src/features/auth`
- `apps/universal/src/features/profile`
- `apps/universal/src/shared/config`

## Decisions made

- Appliance UX should stay simple and avoid exposing cluster internals.
- Local Keycloak username/password must be a complete day-one path.
- Social login is a convenience, not a dependency.
- 2026-06-05: The Pi appliance public web image is built by the manual image
  workflow with `pulse.home.arpa` API/WS/OIDC defaults and the local connection
  profile; do not change those build args without revalidating browser auth.

## Open risks

- First-run UX can become too operational if it mirrors installer details.
- Local hostname/TLS assumptions must not break existing local browser auth
  behavior.

## Next step

- Wait for Phase 1/2 setup contracts, then add only user-visible flows needed
  for appliance onboarding and diagnostics.
