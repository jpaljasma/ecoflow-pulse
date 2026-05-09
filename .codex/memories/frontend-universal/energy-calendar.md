# frontend-universal Memory — Energy Calendar

## Current focus

Build primary Calendar route, API/parser/hooks/model, Energy date picker, nav, device link, and in-app Pulse mark.

## Files to inspect first

- `apps/universal/app/(tabs)/energy.tsx`
- `apps/universal/app/(tabs)/_layout.tsx`
- `apps/universal/src/shared/ui/PulseSidebarNav.tsx`
- `apps/universal/src/features/energy/api.ts`
- `apps/universal/src/features/energy/model.ts`
- `apps/universal/src/features/device-detail/components/DeviceDetailBody.tsx`

## Decisions made

- Calendar route is `/(tabs)/energy-calendar`.
- Calendar tiles use MaterialCommunityIcons-style money and solar icons, not emoji.
- Brand mark refresh is in-app chrome only.

## Open risks

- Calendar UI must avoid text overlap on mobile.
- Keep raw colors centralized or isolated behind helpers.

## Next step

Write failing model and navigation tests, then implement UI in focused slices.
