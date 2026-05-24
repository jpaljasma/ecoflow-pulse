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

## Current Snapshot Freshness

The live projection snapshot tracks both an observation timestamp and a
last-changed timestamp per numeric metric. Volatile fields such as PV watts,
load, AC/DC input, battery power, temperature, and remaining-time estimates
expire independently after the freshness window.

Some provider snapshots keep arriving after the device-side values have stopped
moving. Current-power cohorts are therefore also treated as stale when recent
frames carry the same PV/input/output/remaining-time values for longer than the
flatline window. A slow trickle remains live when a sibling current signal is
still moving, but a frozen provider snapshot no longer leaks stale PV watts or
multi-day ETA values into the current read model.

EcoFlow idle/pause states and sentinel remaining-time values are used as a
second signal when they contradict non-zero PV/input values with no matching
load or battery sink. The same stale-current classifier is applied to live
projection snapshots and historical rollup extraction so repair/rebuild runs do
not preserve flatlined solar buckets.

When no fresh current telemetry remains, current power and ETA are reported as
zero/offline; aggregate battery SOC may remain as the last known reserve.

## Smoothing

Smoothing layers reduce UI oscillation:

- rolling average for PV channels and total flow,
- rolling state net smoothing for charge/discharge/idle classification.

This improves readability and avoids rapid mode flapping on small transient deltas.

UI output is also decoupled from telemetry ingestion through an asynchronous
bounded render queue. This keeps telemetry processing responsive when terminal
output is slow.

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
