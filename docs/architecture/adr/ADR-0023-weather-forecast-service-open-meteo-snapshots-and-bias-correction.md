# ADR-0023: Weather Forecast Service via Open-Meteo With Canonical Snapshots

**Status:** Accepted  
**Date:** 2026-03-18  
**Owners:** Platform / API / App  
**Related:** [ADR-0001-architecture-universal-client-node-rest-bff-go-grpc-data-plane.md](./ADR-0001-architecture-universal-client-node-rest-bff-go-grpc-data-plane.md), [ADR-0004-cache-valkey-redis-compatible-with-replication-sentinel.md](./ADR-0004-cache-valkey-redis-compatible-with-replication-sentinel.md), [ADR-0005-databases-postgres-timescaledb-for-control-plane-rollups.md](./ADR-0005-databases-postgres-timescaledb-for-control-plane-rollups.md), [ADR-0013-grpc-server-baseline-go-high-throughput.md](./ADR-0013-grpc-server-baseline-go-high-throughput.md), [ADR-0020-authenticated-entry-routing-profile-and-device-authorization.md](./ADR-0020-authenticated-entry-routing-profile-and-device-authorization.md), [../README.md](../README.md)

---

## Context
ADR-0020 added opt-in profile weather-location consent so the product can show forecast-aware experiences without exposing raw location parameters on public app routes. We now need an internal weather service that:

- serves forecast data through the existing universal client -> Node BFF -> Go gRPC architecture,
- uses Open-Meteo as the upstream forecast source of record,
- avoids wasteful upstream fragmentation for unit-system variants,
- controls free-tier upstream usage safely,
- keeps profile weather experiences stable during cache hits, transient upstream failures, and brief platform restarts.

This work is operationally different from telemetry/history:

- upstream availability and rate budget matter,
- Open-Meteo's returned latitude/longitude/elevation identify the actual forecast grid cell better than the raw request coordinates,
- forecast snapshots need to survive process restarts rather than only latest-cache state,
- the profile UI needs a stable forecast response shape without pretending model-derived values are measured station truth.

## Decision
- Add a new internal gRPC `pulse.weather.v1.WeatherService` with:
  - `Get7DayForecast`
- Register `WeatherService` inside the existing `cmd/ecoflow-grpc-api` runtime rather than creating a separate binary or public-facing weather service in v1.
- Expose weather to product clients only through authenticated Node BFF endpoints that resolve latitude/longitude/timezone from the logged-in user's saved profile; the public API must not accept raw weather coordinates or tuning parameters.
- Use Open-Meteo as the upstream source set:
  - Forecast API for hot-path 7-day forecast + past-day actuals,
  - Historical Forecast API only for operational backfill after downtime.
- Always fetch metric data upstream and convert to imperial locally in Go; do not split cache entries or upstream traffic by unit-system request.
- Canonicalize location/cache identity from the upstream-returned grid-cell latitude, longitude, and elevation, plus panel tilt bucket (1 degree) and azimuth bucket (5 degrees).
- Use a two-layer storage model:
  - Valkey hot cache with roughly 50-minute TTL and stale-serve support,
  - Postgres snapshot store for issued forecast bundles, hourly target timestamps, and recent-active refresh metadata.
- Apply a bounded global upstream budget manager with weighted daily units, burst protection, stale-serve fallback, and scheduled refresh batching where request shape allows.
- Return both raw and corrected forecast values for client shape stability. Corrected values currently mirror raw values because the yesterday-verification correction loop was retired.

## Rationale
This keeps the product architecture aligned with existing choices while solving the weather-specific reliability problem:

- using the existing gRPC runtime preserves the ADR-0013 server baseline and avoids introducing another deployable before weather traffic justifies it,
- profile-resolved coordinates preserve the privacy boundary chosen in ADR-0020,
- metric-only upstream fetches reduce cache fragmentation and external spend,
- grid-cell canonicalization matches the actual upstream forecast cell instead of overfitting to user-entered coordinates,
- Valkey keeps the hot path fast while Postgres snapshots make restart recovery deterministic,
- a budget manager prevents background refreshes and cache misses from silently exhausting the free tier,
- retaining raw plus corrected fields keeps the client contract stable while avoiding active local correction heuristics.

## Consequences
### Positive
- Weather behavior stays inside the existing universal -> Node -> Go service shape.
- The app can render useful forecast UI from profile consent alone, without exposing coordinate inputs in public routes.
- Upstream usage is bounded explicitly instead of being an accidental side effect of traffic spikes.
- Forecast bundles are snapshotted durably for restart recovery and future operational backfills.

### Tradeoffs
- `cmd/ecoflow-grpc-api` now hosts another domain and background refresh behavior, which increases bootstrap/config surface area.
- Weather storage adds Postgres tables that are not core telemetry rollups, so migration/test coverage must remain disciplined.
- Open-Meteo archived "actuals" are still model-derived and may differ from later station truth; the API must keep provenance explicit.
- Scheduled refresh batching only works for shared request shapes, so custom panel parameters may remain less cache-efficient than the profile-default path.

## Implementation notes
- Requests accept latitude, longitude, unit system, optional panel tilt, optional panel azimuth, and optional timezone at the internal gRPC layer for future internal consumers.
- The public BFF path fixes panel defaults (`tilt=45`, `azimuth=0`) and uses saved profile timezone when present, otherwise upstream `timezone=auto`.
- Responses must keep model-dependent fields nullable, especially hourly UV and solar fields.
- Weather-code handling must retain raw WMO code and expose a local human-readable label/icon mapping.
- Comments and docs must note that Open-Meteo "actuals" are archived model outputs that may later be replaced by station-based truth if the product evolves.
- UI surfaces must include Open-Meteo CC BY attribution.

## Acceptance criteria
- `WeatherService` is available through the existing internal Go gRPC runtime and follows the ADR-0013 baseline.
- Public weather routes are authenticated and profile-driven; they do not accept raw coordinate parameters.
- Forecast cache identity uses upstream-returned grid-cell metadata, not only raw request coordinates.
- Successful forecast bundles are snapshotted durably for recovery and operational backfill.
- Upstream budget exhaustion serves stale cache or rejects instead of continuing to call upstream blindly.
- Universal profile weather widgets show forecast data and attribution through the authenticated app flow.

## Amendment 2026-05-24
Yesterday verification was retired from the Energy surface and data plane. The public BFF route, gRPC RPC, Previous Runs fallback, verification storage, rolling bias state, and local correction loop were removed. The forecast payload retains raw and corrected fields for client compatibility, with corrected values mirroring raw values until a future reviewed correction model is introduced.
