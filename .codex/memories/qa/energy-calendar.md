# qa Memory — Energy Calendar

## Current focus

Build validation matrix and verify backend, BFF, universal model, navigation, and visual behavior.

## Files to inspect first

- `apps/universal/e2e`
- `apps/universal/playwright.config.ts`
- `apps/universal/src/features/energy/model.test.ts`
- `apps/pulse-platform/test/history_routes.test.ts`
- `cmd/ecoflow-grpc-api/telemetry_service_test.go`

## Decisions made

- Targeted tests precede broad gates.
- Browser visual QA compares final render against saved mockups.
- Browser plugin could open the route, but deterministic visual data required
  Playwright route interception, so screenshot QA used the repo Playwright path.
- Final screenshots:
  - `/tmp/energy-calendar-dark-desktop.png`
  - `/tmp/energy-calendar-light-desktop.png`
  - `/tmp/energy-calendar-dark-mobile.png`
  - `/tmp/energy-calendar-light-mobile.png`

## Open risks

- Full live-data visual QA still depends on a real authenticated environment.

## Next step

Package the branch after principal review.
