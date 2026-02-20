# AGENTS

## Scope
This file defines repository-level working rules for humans and AI agents.

## Required Git Workflow
Always use a branch -> pull request -> merge workflow.

1. Start from an up-to-date `main`.
2. Create a dedicated feature branch before making code changes.
3. Commit changes only on the feature branch.
4. Push the branch to `origin`.
5. Open a pull request to `main`.
6. Merge only through the pull request after checks pass.

## Branch Rules
1. Do not commit directly to `main`.
2. Use clear branch names. For Codex-created branches, use `codex/<topic>`.
3. Keep branches focused on one logical change.

## Pull Request Rules
1. Include a clear title and summary of what changed and why.
2. Include test results for the change.
3. Update documentation when behavior or configuration changes.
4. Resolve review feedback before merge.
5. PR body formatting must use real newlines (no literal `\n`).
6. When using GitHub CLI, prefer `gh pr create --body-file <file>` or `gh pr edit --body-file <file>` for multiline Markdown.
7. After creating/updating a PR, verify rendered body with `gh pr view --json body` and fix immediately if escaped newlines appear.
8. PR creation workflow is mandatory: push branch, run `gh pr create`, then verify with `gh pr view <number> --json url,title,body`.
9. If PR creation fails due to branch/rules ambiguity, push to a fresh `codex/*` branch and retry PR creation there.

## Security Review Feedback Rules
1. Always check PR inline review comments and code-scanning findings before merge.
2. If `gh pr view --comments` is empty but feedback is expected, fetch inline comments via:
   - `gh api repos/<owner>/<repo>/pulls/<pr-number>/comments`
3. Treat code-scanning/Copilot security findings as mandatory fixes unless explicitly waived by a maintainer.
4. For numeric conversions, never narrow `int64/float64` to `int` without explicit bounds checks.
5. After fixing review findings:
   - rerun relevant tests and lint,
   - push the fix commit to the same PR branch,
   - update the PR body with a brief note that feedback was addressed.

## Documentation Hygiene Rules
1. Before committing code changes, update developer documentation under `docs/` if any runtime behavior, architecture, telemetry mapping, UI behavior, or configuration changed.
2. Keep the root `README.md` and `docs/README.md` links/navigation accurate when docs structure or key capabilities change.
3. Treat documentation updates as part of the same feature branch and commit series; do not defer doc sync to later cleanup commits.
4. When available, run markdown lint checks before pushing docs-heavy changes.

## Merge Rules
1. Merge only after CI and required checks are green.
2. Prefer squash merge unless the repository maintainers request otherwise.
3. Delete merged feature branches.

## Exceptions
Direct commits to `main` are allowed only if explicitly requested by a maintainer for an urgent reason.

## Locked Architecture Compliance Rules
These rules are mandatory for all new platform work and are sourced from:
- `docs/architecture/README.md`
- `docs/architecture/adr/*.md`

### Source-of-truth rule
1. Treat `docs/architecture/README.md` + accepted ADRs as authoritative architecture contracts.
2. If implementation or design would deviate, create a new ADR first; do not silently drift.
3. Accepted ADRs are immutable except status/superseded metadata.

### Locked system shape (must preserve)
1. Tiering:
   - Expo universal client (Web/iOS/Android)
   - Node REST BFF (public)
   - Go gRPC data/API layer (internal)
   - data plane (ingest/projection/storage)
   - dedicated WebSockets gateway (public realtime)
2. Flow:
   - ingestion -> normalization/derivation -> projection/read models -> UI rendering

### Locked platform choices (do not substitute without ADR)
1. Cloud/K8s:
   - GKE first, region `us-east1`, portable to EKS later
2. Messaging:
   - NATS JetStream
3. Hot cache:
   - Valkey (Redis-compatible), replication + Sentinel (no cluster mode in v1)
4. Databases:
   - Postgres (control plane) + TimescaleDB (rollups/history), operated via CloudNativePG
5. Replay archive:
   - object storage with protobuf + zstd (local MinIO, prod GCS)
6. Auth:
   - Keycloak OIDC with Google/Facebook
7. Realtime:
   - dedicated WebSockets gateway with backpressure/downsampling ladder
8. Local development:
   - k3d Kubernetes with one-command bringup, in-cluster dependencies by default

### Locked data/replay constraints
1. Retention:
   - raw telemetry archive: 30 days
   - minute rollups: 90 days
   - hourly rollups: 3 years
   - daily rollups: 3 years
2. Replay:
   - authoritative replay source is object archive; JetStream replay is operational-only (24-72h)
3. Replay modes must support:
   - per-device replay
   - fleet/shard time-range replay
   - gap repair

### Locked security boundary rules
1. Client auth uses Authorization Code + PKCE via Keycloak.
2. Node REST must validate JWT via JWKS.
3. Node forwards user JWT to Go gRPC metadata.
4. Go validates JWT again and enforces device-level authz (no trust-by-proxy).

### Locked realtime behavior rules
1. WS gateway sends snapshot-on-connect from Valkey, then delta stream from NATS.
2. Backpressure degradation ladder must be implemented and preserved:
   - 250ms -> 500ms -> 1s -> key-metrics-only -> paused
3. Reconnect/resubscribe behavior and UX states are required in clients.

### Locked milestone execution order
1. Execute in order unless explicitly re-planned with documented rationale:
   - M0 platform baseline
   - M1 identity/control plane
   - M2 telemetry pipeline + archive + replay
   - M3 rollups + history + comparisons
   - M4 websocket realtime UX hardening
   - M5 testing/operability/DR-lite
   - M6 online ML recommendations

### Architecture-change workflow
1. Any architecture-affecting PR must:
   - reference relevant ADR(s) or architecture section
   - update `docs/architecture/README.md` and/or ADR index as needed
   - explain compatibility impact and migration path
2. If a decision changes:
   - add new ADR with supersedes pointer
   - mark old ADR as superseded (do not rewrite history)

## Local Development Principles (Developer Experience)
These are mandatory implementation principles for local workflows and tooling quality.

### Core DX principles
1. Local-first and reproducible:
   - a new developer must be able to boot the stack from a clean checkout with documented commands,
   - avoid hidden prerequisites and machine-specific manual steps.
2. Production-parity where it matters:
   - prefer local runtime shape that mirrors deployed architecture (Kubernetes, service boundaries, auth, streaming),
   - avoid local-only shortcuts that hide distributed/system behavior.
3. One-command lifecycle:
   - keep bringup/teardown ergonomic and idempotent (`dev-up`, `dev-down` style),
   - commands should be safe to rerun and recover partial setups.
4. Fast feedback loops:
   - provide quick paths for lint/typecheck/tests before full-stack runs,
   - prioritize deterministic failures with actionable error output.
5. Deterministic behavior:
   - pin toolchain/runtime versions where possible,
   - keep seeded training/testing flows reproducible.

### Local platform expectations (aligned with ADR-0009)
1. Local development should run on k3d Kubernetes by default.
2. Platform dependencies should run in-cluster (NATS, Postgres/Timescale, Valkey, Keycloak, MinIO).
3. Services should run in-cluster by default; local out-of-cluster runs are optional debug paths.
4. Keep local topology close to GKE deployment shape to reduce environment drift.

### Operational UX standards
1. Configuration clarity:
   - all required env vars must be documented with sane local defaults.
2. Observability by default:
   - local runs should emit useful logs/metrics for debugging connection, replay, and ingestion issues.
3. Backpressure/safety visible locally:
   - queue depth, drop behavior, and reconnect states should be inspectable in local mode.
4. Data safety:
   - destructive cleanup commands must be explicit and opt-in.
5. Documentation freshness:
   - whenever local workflow changes, update `/docs` runbooks and command references in the same PR.

## ML Training Workflow (ETA Models)
Use the built-in trainer at `cmd/ecoflow-ml-train` for fast, repeatable tuning.

### Models to train
1. `d2m` (DELTA 2 Max specific)
2. `dpu` (DELTA Pro Ultra specific)
3. `generic` (cross-device fallback)

### Efficiency requirements
Always use all three optimization techniques together:
1. Stratified sampling: preserve `profile x mode x transition` coverage.
2. Successive halving: evaluate many candidates on partial data, keep top subset for full data.
3. Feature precompute cache: rely on cached rolling features in trainer (prefix sums + window caches).

### Standard training commands
Use full telemetry CSV unless explicitly testing subsets.

```bash
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile d2m -candidates 4000 -stages 0.15,0.4,1.0 -seed 88
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile dpu -candidates 4000 -stages 0.15,0.4,1.0 -seed 88
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile generic -candidates 4000 -stages 0.15,0.4,1.0 -seed 808
```

### Selection and reproducibility
1. Compare `best_score` first, then `best_coverage`.
2. Keep and record the exact seed used for final params.
3. If quality is close, prefer the simpler/stabler window configuration.

### Applying trained params
1. Update profile constants in `cmd/ecoflow-mqtt-sub/estimates_profiled.go`.
2. Keep model selection priority:
   - device-specific profile
   - generic profile
   - unit (MPPT/device) estimate fallback
3. Run:
   - `go test ./cmd/ecoflow-mqtt-sub`
   - `go test ./cmd/ecoflow-ml-train`
   - `go test ./...`

### Practical Retraining Runbook (D2M + DPU + Generic)
Use this runbook when telemetry behavior changes (new charging patterns, AC+solar hybrid, low-power idle drift).

1. Capture a fresh segment before retraining:
   - Observe at least 5 minutes while the target behavior is active.
   - Prefer mixed conditions when possible: AC charging + solar + active AC out.
   - Confirm new rows landed in `logs/telemetry_training.csv` before training.

2. Train D2M and Generic after D2M capture:
```bash
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile d2m -candidates 4000 -stages 0.15,0.4,1.0 -seed 88
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile generic -candidates 4000 -stages 0.15,0.4,1.0 -seed 808
```

3. Retrain DPU only when needed:
```bash
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile dpu -candidates 4000 -stages 0.15,0.4,1.0 -seed 88
```

4. Compare against currently deployed params on the same dataset:
   - Only apply new params if `best_score` is lower with equal/better `coverage`.
   - Keep `coverage` near 1.0 for DPU and high for D2M/generic.
   - If results are close, prefer parameter sets that are stable across multiple seeds.

5. Seed sweep guidance:
   - Run 4-8 seeds for the same profile.
   - Choose the winner by:
     1) lowest `best_score`
     2) highest `best_coverage`
     3) simpler/stabler windows

6. Post-update validation:
   - Verify top-state model selection still follows:
     - `New` (device-specific), then `Generic`, then `MPPT`.
   - Verify source icon logic during hybrid charging (AC + solar) updates correctly.
   - Run full tests (`go test ./...`) before commit.

## Solar Panel DB Workflow (including Purchase Links)
Use this when panel metadata/schema changes (for example new fields like `purchase_link`).

1. Treat panel DB updates as end-to-end schema changes:
   - importer (`cmd/ecoflow-panel-db-import`),
   - compact index (`data/solar_panels/solar_panel_specs_v13.index.json`),
   - runtime loader + snapshot mapping (`cmd/ecoflow-mqtt-sub/panel_db.go`, `main.go`),
   - recommendation structs (`cmd/ecoflow-mqtt-sub/viewmodel.go`),
   - tests and docs.

2. For panel purchase links, use this source priority:
   1) explicit CSV `Purchase_link`,
   2) curated override map (`data/solar_panels/panel_purchase_links_v13.json`),
   3) deterministic domain-aware fallback search URL.

3. Keep regeneration reproducible:
   - run `./scripts/regenerate_solar_panel_db.sh`,
   - ensure script passes `-link-map` (or `SOLAR_PANEL_LINK_MAP`) to importer.

4. Validate after regeneration:
   - `go test ./cmd/ecoflow-panel-db-import`
   - `go test ./cmd/ecoflow-mqtt-sub`
   - `go test ./...`
   - `make lint`
   - verify non-empty link coverage with `jq`.

5. Documentation must be updated in the same branch:
   - `/docs/how-to/add-solar-panel-to-db.md`,
   - `/docs/reference/repository-layout.md`,
   - `/docs/reference/commands.md`,
   - `/docs/reference/telemetry-model.md` when runtime behavior changes.

## Solar Panel Detection Model Workflow (Irradiance-Aware)
Use this when panel detection starts misclassifying under low sun, clipping, or mixed D2M/DPU patterns.

1. Use repo-local CSV as canonical source:
   - `data/solar_panels/solar_panel_specs_with_ecoflow_compat_cold_voc_and_safety_margins_v13.csv`
   - avoid ad-hoc absolute paths outside the repo.

2. Always regenerate DB artifacts before retraining detection:
   - `./scripts/regenerate_solar_panel_db.sh`
   - `./scripts/train_panel_select_model.sh`

3. Keep trainer scoring irradiance-aware:
   - fit on robust envelope stats (`p95` + `max`) for watts/volts/amps,
   - apply MPPT safety constraints (voltage/current/watts + cold Voc margin),
   - include clipping penalty so over-cap layouts do not dominate,
   - use a shoulder-hours irradiance curve so low-sun windows do not collapse into tiny-panel classes.

4. Validate with replay after each retrain:
   - verify expected dominant classes by SN/profile/port from `logs/telemetry_training.csv`,
   - specifically check known ground truth setups:
     - D2M low: EcoFlow 220W bifacial portable
     - D2M high: 4x125W EcoFlow bifacial modular
     - DPU low: 2x400W JJN bifacial
   - require model/test gates:
     - `make lint`
     - `make test`
     - `make build`

## Universal App UI Workflow (Expo/Tamagui)
Use this when working on `apps/universal` dashboard layout, telemetry rendering, and responsive behavior.

1. Keep behavior centralized in shared UI components:
   - power-flow glyph logic must be rendered via shared component (`PowerFlowGlyph`),
   - trendline rendering must be shared (`SparklineTrend`) so Home + Details behave identically,
   - avoid duplicating glyph/render logic in page files.

2. Responsive trend layout rules:
   - desktop: 2-column split (`Load Trend` / `PV Trend`, 50/50),
   - mobile: stacked trend containers,
   - trend data must be fixed-length and padded so charts start flat and grow with data.

3. Scroll container rule for web:
   - keep app shell fixed,
   - scroll only inside the content pane,
   - enforce `flex: 1` + `minHeight: 0` on wrapper and `overflowY: auto` on web container to avoid resize-related scroll lock.

4. Telemetry visual feedback rules:
   - values in `[-0.5, 0.5]` for AC/DC/PV/Load should be muted (label + value),
   - use monochrome glyph labels for tintable icons in muted state,
   - cold temperature (`<= 2C`) should use snowflake + blue style.

5. Mock telemetry mapping rules:
   - prefer `telemetry_training.csv` for mock runtime playback,
   - map metrics by device serial and latest timestamp,
   - when multiple pack temps are present in a row, use median.

6. Before every UI commit:
   - run `npm run -w apps/universal typecheck`
   - run `npm run -w apps/universal lint`
7. Performance defaults for telemetry-heavy screens:
   - prefer per-device telemetry selectors (`useTelemetryDeviceSnapshot`) over passing large `byId` maps through props,
   - keep fleet/device trend aggregation in telemetry engine/store, not component-local `setInterval` state,
   - avoid page/card-level global rerenders for relative time labels; compute inactivity in snapshots and keep any "time ago" refresh isolated to leaf text components only when needed,
   - use virtualized list rendering on web and native (`FlatList`/virtualized path) instead of manual mapped grids for device cards.

7. Cross-platform image loading rules (web + iOS):
   - treat brand assets and UI chrome assets as bundled/static first (logos, menu glyphs, icons),
   - for product photos, support bundled fallback for reliability and remote URI only when explicitly configured,
   - always include an error fallback path for fleet summary thumbnails and device cards/details (do not assume URI availability),
   - avoid custom web fetch/blob/object-URL image loops; prefer direct URI rendering with cache-friendly components,
   - if image requests spike, inspect Network for repeated same-filename fetches and verify image source stability per component.

8. iOS layout/safe-area rules:
   - top header/logo rows must account for safe area insets to avoid notch overlap,
   - avoid nested `VirtualizedList` inside plain `ScrollView` with same direction (RN warning, broken windowing),
   - keep list/detail scroll behavior explicit per platform (`FlatList` header pattern on index, dedicated scroll container on detail).

9. Navigation interaction rules:
   - for iOS close/dismiss from detail, prefer `router.back()` when possible so transition direction feels native,
   - keep replace fallback to home route only when stack back is unavailable.

10. Mock telemetry transport reliability rules (web vs iOS):
   - in `mock://` API mode, prefer polling transport by default and avoid WS reconnect loops,
   - do not rely on web-only relative paths (`/logs/...`) on native; provide absolute host-based candidates for iOS/Android,
   - keep multiple native URL candidates for mock files (`/logs` and `/mock`, plus host fallbacks) so incremental updates continue,
   - if UI shows `connected` but data is stale, trace last successful mock fetch path and verify per-second refresh still advances.
