# AGENTS

## Scope
This file adds shared UI and chart-specific rules for `apps/universal/src/shared/ui`.

Follow this file when changing shared chart components, chart model helpers, axes, legends, hover/tap inspection, bucket normalization, or telemetry chart styling.

## Shared Chart Rules
1. Treat chart x-axes as part of the data contract:
   - absolute time-of-day charts must keep bucket index `0` anchored to the displayed window start,
   - do not right-align or `slice(-points)` time-of-day history payloads unless the chart is explicitly rolling-relative,
   - when extending a daylight window, pad missing future buckets at the end so existing buckets keep their clock labels.
2. Keep bucket math centralized in model helpers:
   - put normalization, cumulative-to-bucket conversion, path construction, and hit-test index helpers in shared chart model files,
   - cover those helpers with focused Vitest tests before changing chart rendering,
   - do not duplicate bucket padding or cumulative-series detection inside individual chart components.
3. Hover/tap tooltips must reflect real bucket identity:
   - tooltip labels, values, and crosshair position must come from the same normalized bucket index,
   - future or out-of-window buckets must render as empty rather than borrowing data from an earlier or later bucket,
   - web hover and native tap paths must use equivalent bucket-index logic.
4. Legends must distinguish totals from running comparisons:
   - use `Today so far` for current-day partial totals,
   - use `Yesterday so far` only for the matched elapsed/current-data baseline,
   - keep full previous-day values labeled separately as `Yesterday total`.
5. Comparison charts must preserve accessibility and semantics:
   - render current and comparison periods with distinct stroke styles, not color alone,
   - use semantic theme colors from `src/shared/theme` rather than ad-hoc literals,
   - keep legend labels horizontal/wrapping where possible so chart height is reserved for the plot.
6. User-facing calendar charts must use local-calendar semantics:
   - compute day/month/year boundaries with calendar operations before transport,
   - do not infer yesterday or previous periods by subtracting elapsed milliseconds,
   - include DST-sensitive tests when local calendar boundaries affect buckets or labels.

## Validation
1. For chart model or normalization changes, run targeted chart/helper tests, for example:
   - `npm run -w apps/universal test -- src/shared/ui/solarGeneratedChartModel.test.ts`
2. For rendered chart component changes, also run:
   - `npm run -w apps/universal typecheck`
   - `npm run -w apps/universal lint`
3. If chart behavior depends on BFF/backend response shape, add or update the corresponding server-side regression tests in the same branch.
