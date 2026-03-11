# DPU Storm Guard Findings

Date recorded: 2026-03-11

## Context

- User asked whether active DPU Storm Guard state is visible in stored telemetry.
- Local raw file logs were not sufficient for the current event, so the check used:
  - local k3d context: `k3d-pulse-local`
  - Postgres archive manifest in cluster
  - MinIO raw archive bucket: `pulse-telemetry-raw`

## Archive lookup

- Active provider-device ids found in the March 11, 2026 archive window:
  - `R351ZABAPH331057`
  - `Y711ZABA9H2P0294`
- Explicit Storm Guard fields were found for:
  - `Y711ZABA9H2P0294`
- No explicit Storm Guard fields were found in the same narrow window for:
  - `R351ZABAPH331057`

## Evidence

Archived quota payload for `Y711ZABA9H2P0294` at `2026-03-11T13:30:21.002Z` contained:

- `stormPatternOpenFlag: true`
- `stormPatternEnable: true`
- `stormPatternEndTime: 1773306000`
- `plugInInfoPvLFlag: 1`
- `plugInInfoPvLType: 2`
- `powGetPvL: 0.0`
- `powGetPvH: 0.0`
- `evChgMode: "EV_CHG_MODE_REDUNDANCY_PV"`

Decoded `stormPatternEndTime`:

- `1773306000` -> `2026-03-12T09:00:00Z`

## Interpretation

- The device telemetry does expose Storm Guard state directly.
- The archive confirms Storm Guard was enabled and open/active on `Y711ZABA9H2P0294`.
- The currently observed payload does not expose a richer upstream reason code such as a named weather event or severity; it exposes state and end time, not the cause.

## Related non-Storm DPU finding

For `R351ZABAPH331057`, the inspected archive window showed weak or near-zero MPPT input rather than an explicit Storm Guard flag, for example:

- `pv2InVol` around `10.5V`
- `pv2InAmp` around `0.157A`
- `pv2InWatts: 0`
- `inHvMpptPwr` around `1.5-1.7W`

This should not be conflated with confirmed Storm Guard state unless future payloads show the explicit storm fields above.

## Future implementation note

If Storm Guard is added to the product surface:

- prefer a dedicated backend extraction/mapping path for:
  - `stormPatternEnable`
  - `stormPatternOpenFlag`
  - `stormPatternEndTime`
- label it as device-reported state
- do not infer weather cause from PV lockout alone
- treat `stormPatternEndTime` as UTC epoch seconds and convert to local time in the UI
