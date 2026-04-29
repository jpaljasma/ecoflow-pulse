# AGENTS

## Scope
This file adds universal-app-specific rules on top of the repository root `AGENTS.md`.

## UI System
1. Preserve the shared design-system contract:
   - semantic theme tokens first,
   - shared spacing/radius/control primitives before screen-level overrides,
   - avoid ad-hoc repeated color literals or one-off component variants.
2. Keep Pulse branding canonical:
   - no new explicit EcoFlow branding in user-facing product chrome unless a screen is intentionally describing provider integration,
   - app icons, splash assets, and navigation chrome should align with the Pulse visual system.
3. Accessibility is mandatory:
   - WCAG AA minimum for text and controls,
   - 44px minimum touch targets,
   - visible focus states on web,
   - do not rely on color alone for meaning.

## Data and Routing
1. Use canonical UUID device IDs in routes, params, and outbound links.
2. User-facing day/month/year ranges must use local-calendar semantics, not elapsed-millisecond subtraction.
3. Expensive long-range history views should stay on-demand and cached rather than always-on live refresh.
4. Preserve stale content during refetch when possible; layout jumps and flash-empty transitions are bugs.

## Charts and Telemetry UI
1. Shared chart semantics must stay consistent across screens:
   - positive energy above zero,
   - negative energy below zero,
   - comparison periods rendered distinctly and accessibly.
2. Reuse fetched payloads aggressively for related widgets instead of refetching parallel slices of the same period.
3. Device detail and shared chrome must receive explicit scope from the current page; do not silently re-derive unrelated global scope.

## Auth and Browser Runtime
1. iOS web sign-in must use a same-tab Authorization Code + PKCE redirect instead of relying on popup/opener completion.
2. Store PKCE verifier/state only as short-lived browser state before leaving the app, then exchange the returned code from `/auth/callback`.
3. Keep browser-specific auth workarounds centralized in shared auth helpers, with regression coverage for Safari and Chrome on iOS user agents.
4. Preserve the existing Expo AuthSession popup/native behavior for desktop web and native app flows unless those paths are explicitly being changed and revalidated.

## Validation
1. Run the relevant universal checks for touched areas:
   - `npm run -w apps/universal typecheck`
   - `npm run -w apps/universal lint`
   - targeted `npm run -w apps/universal test -- ...` when logic changes
2. For theme, routing, auth, or browser-runtime changes, keep or add regression coverage before merge.
