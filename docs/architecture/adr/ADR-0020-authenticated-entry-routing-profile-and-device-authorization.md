# ADR-0020: Authenticated Entry, Protected Routing, Profile Preferences, and Device Authorization

**Status:** Accepted  
**Date:** 2026-03-09  
**Owners:** Jaan  
**Related:** ADR-0001, ADR-0007, ADR-0011, ADR-0012, ADR-0013

---

## Context

EcoFlow Pulse already chose Keycloak as the identity provider, social login, Authorization Code + PKCE on clients, JWT validation at both the Node REST and Go gRPC boundaries, and authorization through the `user_devices` mapping. The universal app already has a working Expo Router structure, a devices route, a settings screen that exposes Keycloak PKCE diagnostics, and an auth session store that can hydrate persisted OIDC state.

What is still missing is the product-facing identity experience and the profile/preferences surface around it:

- a public welcome page
- a first-class login page
- redirect-to-login and redirect-back-to-destination behavior
- protected route handling across web/iOS/Android
- a real logout action in app chrome
- server-enforced device authorization semantics for list/detail/realtime paths
- a profile page for display name, readonly email, readonly social avatar/auth method, timezone, and optional location consent for weather/solar forecast
- a clean first-sign-in registration/provisioning flow

This decision must keep daily development local-first on k3d, while still supporting GKE integration validation for real OAuth redirect URIs, TLS, and device/browser behavior.

### Requirements / Goals

- Provide a public landing page and a dedicated login page.
- Keep canonical protected URLs stable (`/devices`, `/device/[deviceId]`).
- Force sign-in before protected pages and redirect the user back to the intended destination after authentication.
- Enforce authorization where the data is accessed, not only in the UI.
- Add a profile page where users can edit display name, inspect readonly email, inspect readonly avatar + sign-in method, choose timezone from an IANA type-ahead selector, and optionally grant one-shot location access for weather/solar forecasting.
- Support first-sign-in provisioning without a separate password-based registration flow.
- Keep the implementation simple enough for a small team and compatible with the existing Expo + Node + Go architecture.

### Non-goals

- Native username/password authentication.
- Multi-IdP rollout beyond Google in this increment.
- Background location tracking.
- Fine-grained sharing UX beyond existing `viewer` / `admin` authorization roles.
- Replacing Keycloak or changing the public/private API topology.

---

## Options considered

### Option A: Keep auth mostly in Settings and gate data fetches only

**Pros**
- Smallest immediate code change.
- Reuses the current Keycloak diagnostics card.

**Cons**
- Poor product UX; users should not have to discover sign-in through Settings.
- No proper redirect-back behavior for deep links.
- Encourages partial protection in the UI instead of explicit protected-route semantics.
- Makes onboarding and logout feel unfinished.

### Option B: Add first-class public pages, protected-route guard, profile page, and server-enforced authorization

**Pros**
- Clean product UX for welcome, sign-in, sign-out, and first-use flows.
- Deep links behave correctly across web/mobile.
- Authorization remains authoritative at Node/Go/WS boundaries.
- Naturally extends the existing Keycloak + JWT + `user_devices` model.
- Creates a durable foundation for future social providers and account settings.

**Cons**
- Requires coordinated changes across universal app, Node BFF, Go API, DB schema, and tests.
- Introduces some routing and bootstrap complexity.

### Option C: Push all auth/session behavior into the BFF and make the client mostly passive

**Pros**
- Web-cookie ergonomics can be simpler in browser-only apps.
- Centralizes more auth logic server-side.

**Cons**
- Poor fit for Expo universal iOS/Android.
- Conflicts with the existing PKCE client-session direction.
- Adds complexity for mobile token handling and WS reconnect flows.

---

## Decision

We will implement a first-class authenticated entry and profile model for EcoFlow Pulse built on top of the existing Keycloak OIDC decision.

### Public and protected routes

We will expose the following public routes:

- `/` → welcome page
- `/login` → sign-in/register page

We will keep the following canonical protected routes:

- `/devices`
- `/device/[deviceId]`
- `/profile`
- `/settings`

Group folder names may change internally in Expo Router, but canonical URLs must remain stable.

### Login / registration model

We will treat registration as first sign-in through Google via Keycloak.

- There will be no separate username/password registration form.
- The login page will present a Pulse-branded **Sign In** action that routes into the Google-backed Keycloak login flow.
- Facebook remains configured infrastructure-side but hidden and disabled by env for this increment.
- On first successful sign-in, backend services will provision or upsert the current user record from trusted identity claims.
- If the user has no assigned devices after sign-in, the app will show an authenticated empty-state instead of failing.
- Login/profile/onboarding acceptance for this milestone must use real Keycloak-issued JWTs end to end; `noop` or dev-subject shortcuts are not acceptable substitutes for user validation.

### Redirect-auth-redirect-back

Protected pages will require a valid session.

- If the user opens a protected page without a valid session, the app will redirect to `/login?returnTo=<relative-internal-path>`.
- After successful sign-in and bootstrap, the app will redirect back to that internal destination.
- `returnTo` values must be sanitized to allow only internal relative paths.
- External or malformed redirect targets must be ignored and replaced with the authenticated default destination (`/devices` when the user has devices, otherwise `/onboarding`).
- `/onboarding` is a first-time authenticated route only; if the user already has at least one device, `/onboarding` must redirect to `/devices`.
- Deploying or restarting public/platform pods must not log the user out if their browser session is still valid; session recovery must survive routine local/platform rollouts.
- Persisted browser sessions must refresh access tokens automatically before expiry; a manual “refresh token” action is not an acceptable primary session-management path.
- This flow participates in the product-level `99.99%` uptime target, so routine deploys must be operationally rock-solid and effectively invisible to signed-in users.
- The same deploy-safety standard applies to supporting background services that keep auth/profile/device data current:
  - worker shutdown must be graceful,
  - in-flight work should drain or hand off safely,
  - routine deploys must not create avoidable processing gaps that later surface as broken profile/device behavior.
- The broader ingest/transform/archive pipeline also participates in the product-level `99.99%` availability target because gaps there directly degrade later device/history/realtime correctness.
- During routine deploy/refetch churn, authenticated views should preserve the last successful profile/homepage/history payload until fresh data is ready; brief empty-state flashes are continuity bugs, not acceptable rollout behavior.
- Rolling deploys must be operationally seamless for signed-in users:
  - active requests and websocket connections should be drained gracefully instead of failing abruptly,
  - readiness/lifecycle handling must keep traffic on healthy pods only,
  - deploy-induced transient bootstrap/auth errors are considered bugs to fix, not acceptable UX during rollout.
- Public-facing Kubernetes workloads participating in auth/bootstrap/realtime flows must be configured for zero-downtime RollingUpdate behavior:
  - run at least two replicas in environments where uninterrupted access is expected,
  - use `strategy.type=RollingUpdate`,
  - use `maxUnavailable: 0`,
  - use non-zero `maxSurge` so replacement pods can become ready before old pods terminate,
  - expose accurate readiness probes and liveness probes,
  - set `terminationGracePeriodSeconds` long enough for in-flight HTTP/WebSocket drain,
  - use a `preStop` hook or equivalent shutdown step so endpoint removal happens before hard termination,
  - protect replica availability with Pod Disruption Budgets so voluntary disruptions cannot drop all ready pods at once.

### Profile page

We will add a first-class `/profile` page with the following fields:

- `displayName` → editable
- `email` → readonly
- `avatarUrl` → readonly provider-managed social avatar
- `authMethod` → readonly session-auth/bootstrap value such as `google` or `facebook`
- `timezone` → selection-only IANA timezone from a type-ahead list, validated on both client and server
- `weatherLocationEnabled` → optional opt-in toggle
- `weatherLocationSource` → `none | auto`
- `weatherLatitude` / `weatherLongitude` → nullable, stored only after explicit consent
- `weatherLocationLabel` → optional human-readable label for the detected location

We will also persist enough trusted social-profile information from the authenticated IdP claims to bootstrap and maintain the user record correctly:

- stable identity key remains `users.keycloak_subject`
- `email`
- `emailVerified`
- social bootstrap values for `displayName`, `givenName`, `familyName`
- `picture` claim mapped to `avatarUrl`
- `locale`

Behavior:

- Timezone defaults to client auto-detection at first sign-in or first profile bootstrap, then becomes user-editable.
- Auto-detection uses platform locale/timezone APIs only; no location permission is needed for timezone detection.
- Weather/solar forecast location is opt-in and revocable.
- Location capture uses foreground, one-shot permission only.
- If consent is revoked, stored weather location fields must be cleared.
- Email is displayed but never edited in-app; identity email remains provider-managed.
- After a user edits `displayName` in Pulse, later IdP claim refreshes must not overwrite it.
- We do not use email as the unique user identifier; stable identity remains the trusted OIDC subject.
- In this architecture, Google profile data is hydrated through brokered Keycloak OIDC claims rather than direct client-side Google profile calls.
- If provider-managed avatar/profile fields are still missing at render time, the profile page may trigger a one-shot authenticated background refresh against Keycloak `userinfo` and persist any newly available provider-managed identity fields without interrupting the current page.

### Authorization model

We will enforce device visibility and access by `user_devices` at every backend read boundary.

Rules:

- Device list endpoints return only devices linked to the authenticated user.
- Device detail/history/insight queries must scope by both authenticated user and device identifier in the same query path.
- Unauthorized direct device reads at the public HTTP boundary should return `401`.
- Mutations that fail role checks should return `403`.
- WebSocket subscriptions must authorize each requested `deviceId` against the authenticated user before subscribing.
- Go remains the authoritative authorization layer even if Node has already validated the JWT.

### Logout behavior

We will provide logout in app chrome and in Settings.

On logout, the client will:

1. stop telemetry subscriptions
2. disconnect realtime transports
3. clear auth state and token storage
4. clear user/device query caches
5. redirect to `/`

Logout uses RP-initiated Keycloak end-session flow in addition to local session teardown so sign-out actually ends the upstream SSO session instead of only clearing local app state.

### Operational constraints

- Daily development remains local-first on k3d.
- Real Google OAuth redirect validation, TLS/ingress behavior, and physical-device integration checks happen in GKE dev only when needed.

---

## Rationale

This option gives Pulse a real product-grade identity experience without fighting the architecture already chosen.

It keeps authentication and authorization aligned with the current design:

- clients handle OIDC Authorization Code + PKCE
- Node validates JWTs at the public API boundary
- Go validates again and enforces authorization close to the data
- WebSockets use the same identity model for realtime subscriptions

It also fits the current app reality better than a “Settings-only” sign-in approach. Users need a predictable welcome/login/logout flow, and deep links to protected device pages must return them to the page they originally wanted.

The profile design stays intentionally small:

- name can be changed locally without becoming a full identity-management subsystem
- email remains provider-owned and readonly
- timezone is user-controlled for history/reporting correctness
- location is optional and privacy-conscious, used only for weather/solar forecast value

This is the right balance between UX quality, privacy, implementation cost, and future growth.

---

## Consequences

### Positive

- Pulse gains a coherent front door and login experience.
- Deep links and protected routes behave correctly.
- Wrong users cannot see wrong devices, including over realtime channels.
- Profile/preferences become explicit and extensible.
- The system is ready for future social providers without redesigning app entry flow.

### Negative / Tradeoffs

- More moving parts must be changed together.
- First-sign-in provisioning and profile bootstrap add backend work.
- Token refresh, logout, and reconnect behavior must be tested more thoroughly.

### Risks & mitigations

- Routine deploy and restart churn can look like auth bugs if readiness/drain behavior is weak.
  Mitigation:
  - public auth/bootstrap/realtime workloads use zero-downtime RollingUpdate settings,
  - signed-in sessions refresh automatically before token expiry,
  - authenticated views keep prior successful data during refetch,
  - websocket shutdown coverage now verifies sessions/subscriptions close cleanly during app shutdown,
  - local cold-start acceptance includes full platform recovery without manual pod intervention.

- **Risk:** open redirect bugs through `returnTo`.  
  **Mitigation:** allow only internal relative paths and fall back to `/devices`.

- **Risk:** UI protection exists but backend authorization is incomplete.  
  **Mitigation:** require authorization in Go query boundaries and WS subscription checks before merging.

- **Risk:** location feature feels creepy or over-collects.  
  **Mitigation:** explicit opt-in, one-shot foreground permission, clear explanation, and full revoke/clear behavior.

- **Risk:** timezone defaults become inconsistent across devices.  
  **Mitigation:** persist a server-side IANA timezone preference after first bootstrap and allow manual override.

- **Risk:** social-profile hydration incorrectly treats mutable fields such as email as identity keys.  
  **Mitigation:** keep `keycloak_subject` as the application identity key, persist social claims as profile attributes only, and track user edits separately from provider-owned bootstrap values.

---

## Implementation plan

1. Add control-plane support for current-user profile/preferences and first-sign-in provisioning.
2. Add Node BFF endpoints for current-user bootstrap, read, and update.
3. Add universal-app welcome page, login page, auth callback handling, protected-route guard, and logout action.
4. Add `/profile` page and settings navigation.
5. Enforce device authorization consistently in Go read paths and WebSocket subscriptions.
6. Add automated tests for redirect-back, unauthorized access, and profile updates.
7. Validate locally on k3d first, then validate real Google redirect/TLS behavior on GKE dev.

### Rollout / Migration

- Add DB migration(s) first.
- Keep existing Settings auth card during rollout for diagnostics.
- Ship welcome/login/profile UX behind normal app navigation once current-user bootstrap endpoint is available.
- Validate Google IdP end-to-end on GKE dev before treating the feature as complete.

### Observability

- metrics:
  - login success/failure count
  - token refresh success/failure count
  - protected-route redirect count
  - unauthorized device read count
  - websocket auth rejection count
- implementation requirement:
  - ship auth/profile observability in the same milestone, not as post-merge cleanup
  - add Grafana dashboard panels that make login, redirect, provisioning, unauthorized access, profile update, and websocket auth behavior inspectable in local/dev
