# Project Manager Memory

## Current focus

- Keep the Energy Dashboard work aligned with the approved loop plan.
- Keep backend, BFF, QA, and milestone tracking synchronized as each slice lands.
- Treat the current universal route as a near-midpoint slice: summary, power, and energy history are real; final spec parity still depends on historical per-port PV support and broader QA/product review.

## Decisions so far

- Tracking was added to `docs/architecture/README.md` under a new Post-M6 follow-up section.
- The canonical task board is `.codex/tasks/energy-dashboard.md`.
- The first BFF API surface is `GET /api/v1/energy/dashboard` inside the existing history routes plugin to preserve shared auth and rate limiting behavior.

## Open risks

- The spec expects explicit energy buckets and per-port history that may not exist in current rollups.
- If the audit shows missing source data, the critical path expands to schema + rollup changes before BFF/UI work can begin.

## Audit result

- Audit completed on 2026-03-11: backend expansion is required before downstream work.
- No human decision is needed yet because the repo already has reusable fleet authz and provider-metadata patterns; the missing piece is historical storage/query shape.

## Next step

- Push the remaining work into QA depth and the historical PV-envelope gap while keeping the interim derived-energy contract explicit in review notes.
