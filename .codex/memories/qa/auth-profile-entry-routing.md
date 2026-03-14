# QA Memory — Auth Profile Entry Routing

## Must-cover areas

- unauthenticated protected-route redirect
- redirect-back sanitization
- first-sign-in bootstrap path
- unauthorized device read semantics (`401` at the HTTP boundary, `403` for role failures)
- logout teardown
- profile update + location revoke behavior

## Next step

- Completed targeted regressions:
  - `useReturnTo.test.ts`
  - `useRequireAuth.test.ts`
  - `LogoutButton.test.ts`
  - `profile/hooks.test.ts`
  - `telemetry/store.test.ts`
  - public route tests for `/api/v1/me`, devices, and history auth semantics
- Remaining QA focus is local acceptance evidence for the live k3d login/profile/realtime flow.
