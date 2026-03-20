# ADR-0024: Solar generation forecast service with energy truth, weather features, and site learning

- **Status:** Proposed
- **Date:** 2026-03-18
- **Depends on:** ADR-0019, ADR-0023

## Context

The current weather profile widgets can display a rough solar outlook by combining forecast irradiance with
device-level PV hints. That is useful as a temporary UI signal, but it is not accurate enough to become a
product-grade solar generation forecast.

The platform already has two critical ingredients:

1. weather forecasts and archived forecast snapshots via `WeatherService` (ADR-0023)
2. actual solar generation truth via the energy/history path and rollups (ADR-0019, ADR-0018)

To become highly accurate, solar forecasting must be treated as a dedicated prediction domain with explicit
training/verification data, per-site learning, and continuous forecast-vs-actual scoring.

## Decision

We will introduce a dedicated internal solar forecast domain and service that uses:

- actual generation from the energy service as the source of truth
- weather forecasts and archived forecast snapshots as explanatory features
- site-specific identity and rolling calibration as the primary forecasting unit

The system will evolve in stages:

### Stage 1: deterministic baseline

Implement a deterministic solar forecast baseline that:

- uses real generation so far today from the energy service
- uses forecast irradiance inputs from `WeatherService`, preferring `global_tilted_irradiance` when available and
  falling back to `shortwave_radiation`
- derives a conservative site capacity estimate from recent observed generation, not raw device input ceilings
- forecasts the remaining part of today plus the next 7 days
- returns forecast values with explicit provenance and confidence notes

### Stage 2: verification and calibration

Persist a per-site forecast training record that captures:

- forecast issue time
- forecast horizon hour/day
- forecasted generation
- actual later generation
- forecast weather features at issue time
- archived/verified weather later available for that hour
- site metadata snapshot and timezone/daylight context

Compute continuous verification metrics for:

- hourly MAE / RMSE
- daily kWh MAE
- peak-power magnitude error
- peak-power timing error
- error by horizon bucket (`same_day`, `day_1`, `day_3`, `day_7`)

### Stage 3: site learning

Introduce site-specific statistical calibration using:

- rolling bias and seasonality
- sun-position features
- cloud/radiation/temperature interactions
- clipping / battery-full / export-limit detection
- site-level shading and persistent underperformance patterns inferred from history

### Stage 4: model-based inference

Add a dedicated inference model that predicts hourly site generation and daily totals from:

- forecast weather features
- recent actual generation history
- site identity features
- temporal features (hour-of-day, day-of-year, season)
- device/system envelope features

The deterministic baseline remains as a fallback and benchmark. New models must beat the baseline on held-out
verification metrics before promotion.

## Architectural shape

The locked system shape remains:

- Expo client
- Node REST BFF
- Go internal gRPC layer
- data plane / projections / storage

The new solar forecast service should be implemented as a dedicated internal domain boundary, not embedded as
ad-hoc widget logic. It may begin inside the existing `cmd/ecoflow-grpc-api` runtime, but the domain must remain
separable so it can later move to its own service if needed.

### Inputs

- energy/history actual generation truth
- weather forecast bundles and verification snapshots
- site metadata and user/profile timezone context
- device topology / power envelope metadata

### Outputs

- today solar outlook:
  - actual so far
  - forecast remaining
  - forecast total
  - estimated peak power and timing
- 7-day daily forecast totals
- optional hourly forecast series
- per-response provenance and confidence

## Data model requirements

Persist forecast-training records keyed by site identity and issue time.

Each record should be able to answer:

- what we predicted
- what weather we used
- what actually happened later
- how wrong we were

This dataset is required for continuous verification, model evaluation, and retraining.

## SLO and observability requirements

Solar forecast is user-facing and must expose:

- request-based availability and latency
- verification freshness
- forecast coverage by horizon
- calibration quality by site/fleet aggregate
- client-observed BFF endpoint health for the public path

Dashboards should show:

- forecast request volume and latency
- forecast-vs-actual error distributions
- active unique forecasted sites
- model/baseline selection mix
- stale data / missing truth / missing weather feature rates

## Consequences

### Positive

- uses real generation truth instead of guessy device ceilings
- creates the dataset required for genuine model improvement
- keeps weather, energy truth, and inference concerns cleanly separated
- supports both deterministic and ML forecasting under one contract

### Tradeoffs

- adds a new cross-domain training dataset and verification pipeline
- requires careful site identity handling and feature-versioning
- increases storage and observability requirements

## Follow-ups

1. Add a milestone task for `solar-forecastd` planning and implementation.
2. Define the internal gRPC contract for solar outlook / forecast responses.
3. Implement deterministic v1 using energy truth + weather forecast remainder.
4. Add forecast-training persistence and verification dashboards.
5. Add site-calibrated statistical forecasting before ML promotion.
