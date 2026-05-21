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
   - app icons, splash assets, and navigation chrome should align with the Pulse visual system,
   - shared product marks in chrome should render the canonical `apps/universal/assets/icon.png` through `PulseMark`,
   - keep Pulse marks on the `Horizon Cut P` direction; do not reintroduce heart-pulse symbols, houses, panels, plugs, bolts, or OEM marks as product chrome.
3. Accessibility is mandatory:
   - WCAG AA minimum for text and controls,
   - 44px minimum touch targets,
   - visible focus states on web,
   - do not rely on color alone for meaning.
4. Dashboard design language is documented in
   `docs/explanation/ui-visual-system.md`; read that file before changing home,
   fleet overview, Energy overview, or shared dashboard composition.
5. For iterative Pulse UI polish, use the project skill at
   `.codex/skills/pulse-universal-ui-workflow/SKILL.md` so tests, visual QA,
   docs, and PR packaging follow the same workflow.
6. For multi-page web/tablet UI consistency, use the shared design skills in
   the right phase of the work; if they are not available, install them using
   the root `AGENTS.md` commands before starting broad UI changes:
   - use `design-systems` before changing shared tokens, component defaults,
     variants, navigation chrome, or reusable interaction patterns,
   - use `flex-grid-flow` before changing page grids, responsive tablet/web
     layout, split panels, card rows, or fluid spacing/typography,
   - use `web-design-guidelines` as a final review pass for accessibility,
     spacing, affordance clarity, and cross-page consistency before PR.
7. Keep Tamagui, Expo, and the existing Pulse primitives as the implementation
   system; Bootstrap, Material, shadcn, or other framework vocabulary should
   only inform consistency and should not replace app-owned primitives unless a
   task explicitly adopts that framework.

## Home And Dashboard Layout
1. Treat overview pages as operating consoles:
   - use the first viewport for useful live state and navigation,
   - avoid sparse middle areas in wide hero panels,
   - keep decorative treatment secondary to scan speed.
2. Device shortcut tiles on the Devices home should stay image-forward and DRY:
   - reuse the shared device-visual resolver for thumbnails and fallbacks,
   - show thumbnail, name, the shared SOC progress bar, and a compact charging-state icon,
   - sort active charging/discharging devices first with stable ordering,
   - cap the preview to two rows on desktop/tablet and one row on phones,
   - keep shortcut tiles visually comparable to neighboring metric tiles,
   - place the `All Devices` action below the shortcut grid.
3. Keep primary and secondary hierarchy clear:
   - full-width solar generation history before secondary widgets,
   - pair complementary widgets in a matched-height 50/50 row on desktop/tablet,
   - stack paired widgets on phones,
   - remove redundant telemetry cards when existing tiles already carry the
     signal.
4. Header chrome must be responsive:
   - hide optional weather/status controls on narrow widths before they overlap
     title, breadcrumb, or product identity,
   - preserve route controls and current-page context as the priority content,
   - route the weather/solar header widget into Energy's Solar panel with the
     current device scope when one is explicit, otherwise fleet scope.
5. Device detail pages should prioritize hardware state before secondary
   telemetry:
   - show battery packs and solar inputs before downstream insight widgets,
   - pair Energy Impact and Device Solar Forecast as matched-height 50/50 cards
     on tablet/desktop,
   - hide redundant live telemetry and diagnostics panels from the default page.

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
4. Shared chart primitives under `src/shared/ui` have additional nested guidance; read `src/shared/ui/AGENTS.md` before changing chart components, chart model helpers, hover/tap hit testing, axis labels, or bucket normalization.

## Auth and Browser Runtime
1. Browser web sign-in must use a same-tab Authorization Code + PKCE redirect when the current tab can be navigated; do not rely on popup/opener completion for web browsers because local, iOS, and embedded browser surfaces can block `window.open`.
2. Store PKCE verifier/state only as short-lived browser state before leaving the app, then exchange the returned code from `/auth/callback`.
3. Keep browser-specific auth workarounds centralized in shared auth helpers, with regression coverage for desktop/local web plus Safari and Chrome on iOS user agents.
4. Preserve the existing Expo AuthSession popup/native behavior for native app flows unless those paths are explicitly being changed and revalidated.
5. In local workflows, local browser/API/OIDC edge with a cloud data plane is `Local Edge` in product UI; connection-profile persistence must include the active build default so switching between true Local and Local Edge does not reuse stale issuer/API state.

## Validation
1. Run the relevant universal checks for touched areas:
   - `npm run -w apps/universal typecheck`
   - `npm run -w apps/universal lint`
   - targeted `npm run -w apps/universal test -- ...` when logic changes
2. For theme, routing, auth, or browser-runtime changes, keep or add regression coverage before merge.
