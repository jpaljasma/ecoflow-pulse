# Frontend Universal Memory

## Current focus

- Expand the existing `/energy` vertical slice toward full spec coverage without inventing backend history that does not exist yet.

## Implemented scope

- Add `apps/universal/app/(tabs)/energy.tsx`
- update tab layout
- create `src/features/energy/*`
- keep URL/device state canonical and UUID-based
- use spec-aligned deep-link params (`device`, `preset`, `compare`, `tz`) while still reading legacy aliases
- keep the page scrollable and layout-stable during loading
- persist local grid-price settings client-side for v1
- render real summary, power history, and energy history from the backend contract
- render archive-backed historical PV maxima/last-seen data inside the PV envelope section when available

## Next step

- Add broader rendered-route coverage for scope/preset changes and review whether the current PV-history presentation matches the spec closely enough for product review.
