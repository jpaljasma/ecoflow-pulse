# bff-node Memory — Energy Calendar

## Current focus

Expose `/api/v1/energy/calendar` and pass selected-date dashboard requests through Node safely.

## Files to inspect first

- `apps/pulse-platform/src/routes/history.ts`
- `apps/pulse-platform/src/grpc/telemetryClient.ts`
- `apps/pulse-platform/test/history_routes.test.ts`
- `apps/pulse-platform/test/telemetry_client.test.ts`

## Decisions made

- Route params use canonical UUID `deviceId` only when `scope=device`.
- `date` is `YYYY-MM-DD`.

## Open risks

- Keep BFF schema narrow and aligned with proto.

## Next step

Write failing route/client tests for calendar and date pass-through.
