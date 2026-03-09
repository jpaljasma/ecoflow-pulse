# pgroll Plans

This directory is reserved for future `pgroll` migration plans.

Current repository state:

- active rollout path: raw SQL `*.up.sql` migrations through
  `ecoflow-db-migrate-job`
- minimal `pgroll` adoption: local tooling + planning only
- not active yet: application/runtime search-path cutover for simultaneous old
  and new schema serving

When the runtime cutover work is planned, store versioned `pgroll` plan files
here and keep them alongside the existing SQL migrations until the final
transition away from the raw SQL rollout path is approved.
