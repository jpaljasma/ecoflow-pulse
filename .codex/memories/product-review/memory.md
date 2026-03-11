# Product Review Memory

## Current focus

- Hold final review until the current usable frontend slice also has enough QA depth to trust route behavior.
- Route QA now covers render smoke, compare toggle, scope switching, and preset switching on `/energy`.

## Final acceptance lens

- Match the approved Energy page spec, not just local implementation convenience.
- Treat missing summary cards, broken comparison framing, wrong time windows, or weak PV/battery sections as blocking defects.
- Treat placeholder-empty charts as expected only during backend vertical-slice work; they are not acceptable for final signoff.

## Next step

- Run a full spec-gap pass now that route QA is no longer shallow.

## Review outcome

- Recommendation: accepted as a truthful v1 dashboard branch and ready for PR closeout.
- Reason: implementation, QA, backend contract coverage, and local k3d walkthrough are now strong enough for merge; interim-derived energy buckets are explicitly accepted for v1 instead of treated as a remaining blocker.
- Local k3d validation on 2026-03-11 confirmed the deployed `/energy` page renders real fleet and device-scoped data at `https://localhost/energy`, and the final localhost review accepted the current UX polish and deep-link behavior.

## Accepted v1 notes

- The hero energy chart exposes optional series for `AC input`, `AC output`, `DC output`, `Battery charge`, and `Battery discharge`.
- The secondary power chart includes battery power plus previous-period overlays with clearer relative-time labeling.
- The battery section includes a compact flow strip and SOC band, and battery colors are now consistent across the dashboard.
- The PV section includes historical maxima, headroom, bottleneck text, and responsive all-device cards without horizontal scrolling.
- The backend still derives several energy buckets from existing rollup power averages; this is accepted for v1 and no longer treated as a release blocker.

## Recommendation for next implementation step

- Proceed with PR closeout and merge workflow on the current branch.
- Keep persisted explicit energy buckets as a follow-up improvement, not a blocker for this release candidate.
