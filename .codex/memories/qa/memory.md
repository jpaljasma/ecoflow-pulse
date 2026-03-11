# QA Memory

## Current focus

- Gate each future slice with targeted validation instead of one late full sweep.

## Must-cover areas

- DST-sensitive local window math
- authz for all-device vs single-device scope
- BFF contract validation and error mapping
- frontend URL-state sync and loading stability
- Playwright coverage for `/energy`

## Validations recorded

- `go test ./internal/energydashboard -count=1` passed for DST-sensitive preset windows, scope normalization, and summary math helpers.
- `go test ./cmd/ecoflow-grpc-api -count=1` passed for single-device and fleet-scope dashboard RPC coverage.
- `npm run -w apps/pulse-platform typecheck` passed after adding Node dashboard client types and normalization.
- `npm run -w apps/pulse-platform test -- history_routes.test.ts` passed after adding BFF dashboard route coverage.
- `npm run -w apps/universal typecheck` passed after adding the `/energy` tab route, energy client/hooks/model, and local settings store.
- `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts` passed for route normalization and persisted price-setting coverage.
- `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts src/features/history/powerTrend.test.ts` passed after wiring the live energy-history chart.
- `npm run -w apps/universal e2e:web -- energy.spec.ts devices.spec.ts` passed with mocked browser coverage for `/energy`.
- `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding interactive compare-toggle coverage for `/energy`.

## Next step

- Add frontend-focused tests around the remaining interactive rendered behavior: preset changes, scope changes, and layout stability during refresh.
