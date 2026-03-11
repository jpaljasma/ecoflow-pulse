# BFF Node Memory

## Current focus

- Keep the new Energy dashboard API contract stable for frontend consumption.

## Implemented scope

- `GET /api/v1/energy/dashboard`
- query validation for `scope`, `deviceId`, `preset`, `timezone`, optional comparison and pricing fields
- auth header forwarding
- direct gRPC response passthrough after normalization in `src/grpc/telemetryClient.ts`
- route tests for single-device scope, fleet scope, and validation failures

## Important note

- Boolean query flags must not use `z.coerce.boolean()` here; URL strings like `false` coerce incorrectly. The route now uses an explicit boolean parser for `includeComparison`.

## Next step

- Support the frontend with any small view-model adjustments once chart sections and PV-port details need richer response fields.
