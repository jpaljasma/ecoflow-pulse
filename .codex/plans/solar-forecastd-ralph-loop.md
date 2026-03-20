# Solar Forecastd Ralph-Loop Plan

Status: In progress  
Last updated: 2026-03-18

## Goal

Build the deterministic baseline for a high-accuracy solar generation forecast system using:

- energy-service truth for actual solar generation so far
- weather forecast features from `WeatherService`
- conservative site calibration from observed production
- universal profile surfaces that clearly separate actual generation from forecast remainder

## Locked defaults

- Actual generation truth comes from the energy/history path, not inferred device limits.
- Weather remains an explanatory feature source, not the source of truth for generation.
- The deterministic baseline must stay as the benchmark and fallback for later ML models.
- Today’s outlook must be expressed as actual-so-far plus forecasted remainder.
- Future daily outlooks must be conservative and calibrated from observed production before any device-limit fallback.

## Workstreams

1. Product + architecture
   - ADR-0024
   - milestone tracking in `docs/architecture/README.md`
   - Ralph-loop plan/task scaffolding
2. Deterministic forecast baseline
   - shared solar outlook helpers
   - fleet solar history integration
   - actual-so-far plus remaining forecast calculation
   - conservative capacity calibration from observed solar
3. UI integration
   - profile route wiring
   - current weather widget solar outlook
   - 7-day forecast solar summaries
4. Verification runway
   - preserve clean seams for future forecast-vs-actual persistence
   - preserve clean seams for future model-based inference

## Validation targets

- `npm run -w apps/universal test -- src/features/weather/model.test.ts`
- `npm run -w apps/universal test -- src/features/weather/api.test.ts src/features/weather/hooks.test.ts`
- `npm run -w apps/universal typecheck`
- `npm run -w apps/universal e2e:web -- profile-weather.spec.ts`
- `make lint`

## Notes

- The first deterministic slice should improve trust immediately, even if it is not yet a dedicated backend service.
- The next architectural step after this slice is a dedicated internal solar forecast contract with persisted forecast-vs-actual training records.
