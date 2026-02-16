# Explanation: Telemetry and Estimation

This project combines noisy real-time telemetry with layered estimation logic
to keep the UI stable and useful.

## Telemetry Reality

EcoFlow payloads can be:

- partial (sparse updates),
- duplicated across aliases,
- differently scaled by message type or device family,
- temporarily stale or contradictory across channels.

Because of this, one payload is rarely sufficient to infer stable state.

## Derivation Strategy

The dashboard derives values using precedence rules and fallbacks:

- prefer direct channel watts where available,
- use voltage/current fallback when direct watts are missing and signals are credible,
- guard against impossible values (scale mismatches, sentinel times,
  out-of-range hints),
- preserve explicit zero states when confirmed, to avoid stale carryover.

## Smoothing

Smoothing layers reduce UI oscillation:

- rolling average for PV channels and total flow,
- rolling state net smoothing for charge/discharge/idle classification.

This improves readability and avoids rapid mode flapping on small transient deltas.

## ETA Modeling

ETA rows combine:

- heuristic estimates from current net flow and known capacities,
- ML-assisted estimates from recent power history and trend adaptation.

Confidence controls which estimate is surfaced as primary state text:

- high ML confidence: prefer ML estimate,
- otherwise: use heuristic or device-reported remain when stronger.

## Minute Buckets

Minute telemetry stores energy (`Wh/min`) rather than instantaneous power.

This makes downstream trend, cost, and savings analysis straightforward and
aligns with future TSDB export plans.
