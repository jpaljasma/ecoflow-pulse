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

## Milestone Task Kickoff Workflow
When starting any new milestone task from `docs/architecture/README.md`:

1. Switch to an up-to-date `main`.
2. When picking a task, ensure that dependencies are satisfied first.
3. Create a dedicated `codex/<topic>` branch for that one task.
4. Mark the selected milestone task status from `TODO` to `PROGRESS` before implementation starts.
5. Read `docs/architecture/README.md` + all `docs/architecture/config*.md` files relevant to the task.
6. Share an implementation overview first (scope, files, acceptance criteria, risks).
7. Ask focused clarification questions when requirements are ambiguous; do not start coding until critical gaps are resolved.
8. Implement step by step, keeping docs updated in the same branch.
9. Run tests/lint relevant to the change, then commit/push and open a PR.

### Milestone Breakdown Tracking (required)
1. When a milestone task is large, break it into sub-steps directly in `docs/architecture/README.md` under that task.
2. Use markdown checkboxes to track sub-steps, for example:
   - `[x] done`
   - `[ ] next`
3. Update checkbox state in every implementation chunk commit so milestone progress is always visible.

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
4. For commits touching Markdown, run `make lint` before push.
5. Markdown linting uses repo-level sane defaults in `.markdownlint.json`; avoid broad doc reflow/polish unless the task explicitly requires it.
6. Before committing, inspect `git status` for accidental duplicate/editor-backup files (for example `* 2.go`, `* 2.tsx`, `* 2.sql`, `* 2.md`) and remove them from the branch; do not ship cleanup debt or stray copies in PRs.

## Universal UI Data Rules
1. Universal app routes, query params, and outbound UI links must use canonical UUID device IDs only; do not pass raw serial numbers in UI parameters.
2. Solar history views must reuse a single fetched payload for today totals, yesterday totals, delta, and comparison series, and must refresh again after the local midnight boundary so day-over-day charts roll forward automatically.
3. Solar history comparison windows must use explicit client-computed local-day bounds:
   - `today`: current local midnight -> now
   - `yesterday`: previous local midnight -> current local midnight
   - do not derive yesterday by subtracting elapsed milliseconds from `now`; that breaks spring-forward and fall-back DST transitions.
4. Energy-impact UI must use measured solar generation for the displayed period only; do not annualize, extrapolate, or invent month/year/lifetime avoided-emissions values unless those real solar totals exist in the app.
5. Avoided-emissions factors must be versioned in code/docs and clearly labeled in the UI when using a default or fallback region.
6. When energy-impact UI mixes methodologies (for example marginal-grid avoided emissions and tree-equivalent lifecycle comparisons), label the distinction explicitly in both the widget detail text and the explainer screen.

## Time Handling Rules
1. Persisted/backend timestamps use UTC semantics; user-facing day/month/year periods use local calendar semantics unless a feature explicitly says otherwise.
2. For local period windows:
   - build boundaries in local time first,
   - then convert those boundaries to UTC/epoch values for transport and storage.
3. Never derive previous day/month/year windows by subtracting elapsed milliseconds from `now` when the feature is meant to follow local calendar boundaries.
   - use calendar arithmetic such as `setDate`, `setMonth`, or `setFullYear`,
   - account for spring-forward and fall-back DST transitions explicitly.
4. For rolling "past 12 months" style views, prefer calendar-year backshift (`setFullYear(-1)`) over fixed `365d` subtraction so leap years and DST shifts do not silently skew the intended local-period window.
5. Any new date-range helper used by user-facing UI should have tests covering DST-sensitive boundaries when the logic depends on local calendar time.
6. When a UI mixes UTC-backed storage with local-day presentation, document which parts are local-calendar math and which parts are UTC query transport.

## Frontend Learnings
1. Apply DRY aggressively for reusable UI and state patterns:
   - when the same styling or interaction appears in more than one place, extract a shared primitive/helper instead of duplicating per-screen tweaks,
   - prefer one app-owned wrapper component over repeated raw third-party component overrides when the team wants a consistent default behavior.
2. For arbitrary rgba/hex colors on Tamagui components, prefer `style={{ ... }}` when prop typing is token-restricted; use theme-token props when a stable token exists.
3. Prefer small in-app explainer screens over raw external doc jumps for user-facing methodology questions; external docs can remain a secondary "full reference" link.
4. For environmental/energy widgets, visual styling should read as intentional and domain-specific:
   - prefer green/teal/blue palettes for sustainability impact
   - keep environmental icons simple and legible (for example a leaf badge)
   - keep methodology text factual and non-marketing.
5. When showing derived environmental metrics, keep the data provenance visible:
   - identify the measured source period (`today so far`, month, etc.),
   - identify the factor source/region,
   - keep error/fallback states explicit rather than silently switching methodology.
6. App pages should be scrollable by default:
   - static/detail/info screens should use a `ScrollView` (or equivalent web overflow container) for the main body,
   - do not assume desktop-height layouts fit on mobile or split-screen,
   - treat non-scrollable pages as a bug unless there is a strong interaction reason not to scroll.
7. Expensive long-range history views should be loaded on demand and cached:
   - keep current-period widgets on the live refresh path,
   - defer trailing-month/year fetches until the user explicitly selects them,
   - once loaded, prefer cached results over realtime polling unless the feature explicitly requires live long-range refresh.
8. Dynamic UI must not be jumpy:
   - loading, toggle, and refresh states should preserve layout height/width whenever possible,
   - prefer stale-while-refresh behavior over clearing content during async transitions,
   - reserve space for status text, buttons, and badges that may change label,
   - avoid scroll-position jumps, card collapse/expand flicker, and large reflow on routine state changes,
   - treat visible layout jank during normal interaction as a bug, not cosmetic polish.
9. Universal app theming must preserve the working root-theme contract:
   - the root Tamagui provider follows base `light` / `dark` from system appearance,
   - palette variants such as `original-*` / `new-*` are applied as nested themes, not as the root provider theme names,
   - do not replace the root `light` / `dark` provider path with custom theme names unless the web root/theme-class behavior is revalidated end-to-end.
10. Web dark mode must use browser-native appearance signals explicitly:
   - on web, resolve dark mode from `window.matchMedia('(prefers-color-scheme: dark)')` and subscribe to changes,
   - do not assume `useColorScheme()` alone is sufficient for browser dark-mode scheduling/updates,
   - when applying theme changes on web, ensure `html`, `body`, and the app root receive the resolved background/color-scheme so the page cannot stay visually light while the app theme says dark.
11. Theme changes require regression coverage:
   - keep tests for default family/variant resolution,
   - keep tests for persisted theme preference migration and storage,
   - when changing root theme wiring, verify the universal web app still follows system dark mode before merge.
12. Theme color usage must stay semantic and centralized:
   - keep reusable UI colors in the shared theme catalog/semantic theme layer, not duplicated inline across components,
   - derive component states (hover, muted fills, chart frames, badge backgrounds) from named theme colors instead of inventing new ad-hoc hex/rgba literals in feature code,
   - treat repeated raw color literals in Expo/Tamagui feature components as theme debt to eliminate, not as an acceptable steady state.
13. Universal web telemetry reconnect must stay browser-native:
   - on web, websocket reconnect should retry the current browser-origin endpoint directly,
   - do not rotate through native-dev host fallbacks such as `127.0.0.1` or `localhost` after a browser disconnect,
   - keep regression coverage for the web reconnect path so deploy rollouts do not silently reintroduce multi-second reconnect delays.
14. Expo web-dev on loopback must prefer the secure local edge when the platform ingress terminates TLS:
   - if the browser app is served from loopback `:8081` and no explicit `EXPO_PUBLIC_API_URL` / `EXPO_PUBLIC_WS_URL` override is set, default browser API/WS traffic to `https://localhost` / `wss://localhost`,
   - do not rely on `http://localhost` requests that immediately redirect to HTTPS, because browser cross-origin redirects can surface as misleading CORS failures,
   - when local browser-origin support changes, keep explicit docs/tests for the `http://localhost:8081` -> `https://localhost/api/*` path.
15. Expo public env handling must treat blank strings as unset:
   - Docker/Helm build args often materialize missing `EXPO_PUBLIC_*` values as empty strings rather than omitting them,
   - browser runtime config must fall back to secure localhost defaults when those values are blank,
   - keep regression coverage so empty-string build args cannot silently produce broken API/WS URLs like `?token=...`.
16. Local browser auth + realtime at the shared edge depends on all public ingress paths staying intact:
   - keep `/realms` and `/resources` routed to Keycloak for OIDC discovery/login assets,
   - keep `/ws` routed to the realtime gateway,
   - treat missing edge routes as production-grade regressions because they break login or websocket telemetry in ways that look like frontend bugs.
17. Profile timezone UX must stay selection-only and IANA-backed:
   - use a type-ahead picker over real IANA timezone values,
   - do not allow arbitrary free-text timezone submission in the UI,
   - validate timezone values on both client and server.
18. Public rollout settings must preserve seamless auth/realtime availability:
   - public-facing Deployments should use RollingUpdate with `maxUnavailable: 0` and non-zero `maxSurge`,
   - keep readiness probes accurate enough that traffic never shifts onto pods before they can serve auth/bootstrap/realtime requests,
   - use graceful termination windows plus `preStop` drain behavior where needed so in-flight HTTP/WebSocket traffic is not cut off abruptly,
   - protect multi-replica public workloads with Pod Disruption Budgets so voluntary disruptions cannot take the whole user path down at once.
19. Full local platform restarts must converge automatically:
   - after Docker/k3d restarts, the cluster should recover to healthy without manual pod babysitting,
   - use startup probes and dependency-aware readiness where normal warmup would otherwise look like a crash loop,
   - treat “works after manual restart/delete” as an incomplete fix, not acceptable steady state.
20. Graceful deploy behavior applies to background workers too:
   - routine deploys must not abruptly cut ingest, projection, rollup, archive, inference, or repair workers,
   - workers must stop accepting new work, drain or hand off in-flight work safely, and exit without corrupting state or causing duplicate side effects,
   - deployment/restart behavior that creates avoidable data gaps, replay debt, or crash-loop churn is an availability bug.

## Local Telemetry Pipeline Rules
1. Prefer in-cluster containerized workers over long-running local `go run` loops.
2. Use `make dev-up` + `make services-up` as the default local runtime path for ingest/projection/archive.
3. Do not reintroduce the deleted terminal-based telemetry dashboard/runtime (`cmd/ecoflow-mqtt-sub` / `make mqtt`); local product validation should happen through the universal web/app surface backed by the k3d platform.
3. Keep worker image flow reproducible:
   - `make services-image-build-local`
   - `make services-image-import-local`
4. Keep the local public/API path multi-replica by default for round-robin validation:
   - Node REST BFF/public app: `2` replicas,
   - WebSocket gateway: `2` replicas,
   - Go gRPC API: `3` replicas.
5. Keep local `pulse-services` worker deployments multi-replica by default so rollout restarts do not create single-pod gaps:
   - ingest, inference, projection, rollup, archive: `3` replicas each in local/dev defaults.
6. Service rollouts must wait on the platform dependency endpoints they consume before applying/restarting workloads:
   - at minimum: CNPG rw service, NATS, Valkey, and MinIO.
7. For local websocket HA validation, remember Kubernetes balances on connection establishment; use reconnects or multiple clients to exercise more than one gateway pod.
8. For local Valkey replication+sentinel, lock/write paths must target a writable primary endpoint; avoid random replica fan-out endpoints for lease writes.
9. Valkey durability for the default 99.99% baseline must not rely on ephemeral memory-only pods:
   - keep AOF enabled,
   - back Valkey data nodes with PVCs,
   - use Sentinel-managed failover/recovery settings that preserve service continuity across cold restarts,
   - treat PVC loss as a storage incident, not normal restart behavior.
9. Historical rollup regeneration must be non-destructive by default:
   - do not delete a requested rollup window before rebuilding it,
   - prefer direct archive-to-rollup rebuilds with bounded transactional chunk replacement over replaying through NATS when the goal is to overwrite historical buckets safely.
10. Quota-derived normalized telemetry frames are replay-relevant and must remain archiveable for future rebuild accuracy; do not reintroduce archive skip behavior for `source=quota` without a new ADR.

## Browser Edge Learnings
1. Browser-facing HTTP/2/HTTP/3 support is an ingress/public-edge concern, not a Node runtime concern.
2. HTTP/3 at ingress requires all of:
   - ingress runtime built with QUIC/HTTP/3 support,
   - UDP `443` exposure at the edge,
   - QUIC listener config in the generated server block,
   - `Alt-Svc` advertising HTTP/3.
3. Local trusted TLS should use a CA issuer plus leaf cert, then trust the CA; trusting a bare self-signed leaf cert is not reliable for browser/`curl` verification.
4. For local `platform-up`, webhook-backed ingress/cert-manager resources need readiness gates before the second Helm reconcile:
   - ingress-nginx controller + admission endpoints,
   - cert-manager controller/webhook/cainjector.
5. HTTP/2 server push remains out of scope; prefer preload / `103 Early Hints` and optional `preconnect`.

## Go Lint Hygiene Rules
1. `golangci-lint run ./...` must pass before commit.
2. Treat `errcheck` as mandatory: explicitly handle close errors (or intentionally ignore with `_ = ...`).
3. Avoid ineffective assignments and dead branches; remove no-op placeholder logic instead of suppressing it.
4. Keep deprecated gRPC APIs out of tests/runtime paths (prefer `grpc.NewClient` over deprecated dial patterns).
5. If helper code is intentionally retained but currently unused, annotate with a short `//nolint:unused` reason.

## Go Race Testing Rules
1. Run `make test-race` for concurrency-sensitive changes (leases, worker loops, projection/archive ingest paths, gRPC streaming paths).
2. `make test-race` is the PR-critical race scope and must stay fast/stable.
3. Use `make test-race-stress` for repeated contention checks (`RACE_STRESS_COUNT` default `5`) before merging changes that touch lock ownership, queueing, session lifecycle, or async publish paths.
4. Keep stress race checks opt-in (manual) and avoid making them mandatory per-PR unless explicitly requested by maintainers.
5. Do not reuse package-level mutable maps/slices in gRPC fallback responses; clone shared defaults per request so concurrent RPCs never share writable state.
6. For bounded async worker queues, shutdown must signal cancellation/closed state before or alongside channel close so concurrent `Publish`/`Close` paths cannot panic or hang on send/close races.
7. For JetStream worker shutdown, do not spawn timeout-abandoned `sub.Drain()` goroutines; stop new deliveries first and wait on explicit in-flight handler tracking with a bounded timeout.
8. When exposing queue-depth telemetry for async pipelines, prefer a single source of truth (for example the buffered channel length) over separate producer/consumer depth counters that can race under load.
9. Every Go program under `cmd/` should keep at least one regression test covering real bootstrap behavior (for example env/config parsing, argument normalization, or helper logic); do not leave main packages completely untested.
10. When a package owns a hot request/worker path, keep at least one benchmark in that package and update it when the hot path changes materially.

## SLO Rules
1. When defining SLOs, follow the Google SRE service-level objective model: choose user-relevant SLIs first, then define objective targets/error budgets separately from the dashboard presentation.
2. The default availability target for Pulse is `99.99% uptime` across both the public user path and the critical ingest/transform/archive data path.
3. Treat deploy safety as part of that SLO, not separate ops polish:
   - routine deploys must be effectively invisible to signed-in users,
   - auth/bootstrap/device/realtime paths must stay available throughout normal rollouts,
   - any deploy behavior that causes avoidable user-visible interruption is an availability bug.
4. For gRPC APIs, the default SLI set is request-based availability plus latency distributions; use throughput as context, not as the objective itself.
5. Availability/error-rate SLO views must be request-based (`good / total`) over gRPC status codes, not process uptime.
6. Latency SLO views should include at least `P95` and `P99`, and when a target is claimed (for example `99.99%` availability), show the target/error budget on the dashboard explicitly.
7. SLO dashboards should support filtering by endpoint or method so per-RPC behavior is inspectable without cloning whole dashboards.

## Service Logging Throughput Rules
1. All long-running services/workers and operational CLIs must use `pkg/logger` (`BuildServiceLogger`) for consistent structured logging behavior.
2. Keep high-volume payload logs off the hot path:
   - payload logging must remain `DEBUG` level only,
   - payload logging must be sampled (for example every `N` messages), never per-message at `INFO`.
3. Async logging is the default path:
   - bounded queue with drop-on-full for low-priority logs,
   - warning/error logs bypass queue synchronously.
4. Emit async logger SLO metrics (`queue_depth`, `dropped_total`, etc.) on a periodic ticker with jitter (`StartAsyncMetricsReporter`).
5. Async logger shutdown must be graceful and race-safe; avoid send/close channel races in hot logging paths.

## M1 Auth Implementation Rules
1. Treat M1 auth as complete only when all four layers are wired and validated together:
   - Expo PKCE client flow (Keycloak OIDC),
   - Node JWKS JWT middleware package,
   - Go gRPC JWT validation/interceptors,
   - control-plane RBAC (`viewer/admin`) device registry paths.
2. Session management must be automatic and deploy-safe:
   - persisted browser/mobile sessions must refresh access tokens automatically before expiry,
   - manual refresh buttons are diagnostic-only and are not an acceptable primary session path,
   - transient deploy/restart churn must not clear a recoverable session or force a user-visible relogin.
3. For Expo PKCE work, keep runtime configuration env-driven:
   - `EXPO_PUBLIC_OIDC_ISSUER_URL`,
   - `EXPO_PUBLIC_OIDC_CLIENT_ID`,
   - `EXPO_PUBLIC_OIDC_AUDIENCE`,
   - `EXPO_PUBLIC_OIDC_SCOPES`.
4. Every auth change must run both JS and Go validation gates:
   - `npm run typecheck --workspace @ecoflow-pulse/node-jwks-auth`
   - `npm run test --workspace @ecoflow-pulse/node-jwks-auth`
   - `npm run -w apps/universal typecheck`
   - `npm run -w apps/universal lint`
   - `npm run -w apps/universal test`
   - `go test ./...`
5. Keep frontend CI aligned with auth package coverage (do not allow `node-jwks-auth` tests/typecheck to be optional when auth files change).
5. Authenticated product flows must use real authn/authz during validation:
   - do not rely on `noop` or dev-subject shortcuts for login, profile, onboarding, or device-ownership acceptance,
   - local acceptance should exercise real Keycloak-issued JWTs end to end through the public app, realtime gateway, and Go gRPC auth boundary.
6. Deploys must be seamless for signed-in users:
   - rolling deploys must drain active connections safely instead of surfacing transient user-facing auth/bootstrap errors,
   - a valid browser session must survive routine pod rollouts and platform restarts,
   - treat deploy-induced logout, onboarding fallback, websocket interruption without recovery, or visible bootstrap failures as bugs, not acceptable rollout behavior.

## Milestone Closure Rules
1. Do not mark milestone tasks `DONE` until all listed acceptance criteria are explicitly validated with real command output.
2. Record acceptance evidence in `docs/architecture/README.md` in the same branch/commit series as the implementation.
3. For local platform validation, ensure commands target `k3d-pulse-local` explicitly (context-pinned) so local checks cannot accidentally run against GKE.
5. Whenever new developer-facing commands or Make targets are introduced/changed, update `docs/reference/commands.md` in the same commit with command meanings/default behavior.
6. If implementation/validation fails due to missing local tooling (for example `k3d`, `kubectl`, `helm`), update developer docs in the same branch to record the requirement and where it is needed.

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
4. M2 contract rules:
   - canonical archive frame is `TelemetryEnvelope` (`proto/pulse/envelope/v1`)
   - envelope and payload versions must be explicit fields in each frame
   - NATS subject naming/sharding must use shared helpers (`internal/telemetrybus`), not ad-hoc string formatting

### Locked security boundary rules
1. Client auth uses Authorization Code + PKCE via Keycloak.
2. Node REST must validate JWT via JWKS.
3. Node forwards user JWT to Go gRPC metadata.
4. Go validates JWT again and enforces device-level authz (no trust-by-proxy).

### Locked control-plane schema rules
1. Control-plane relational IDs must use `UUID` with PostgreSQL-native `uuidv7()` defaults.
2. `users.keycloak_subject` is required and globally unique.
3. `users.email` is nullable, indexed, and non-unique.
4. `devices.ecoflow_sn` is required and globally unique.
5. `user_devices` keeps composite PK `(user_id, device_id)` and role check constraints.
6. `created_at` and `updated_at` are always UTC semantics with `TIMESTAMPTZ`, and are always application-managed (no DB default/trigger-owned timestamp writes).

### Provider integration + ingest rules (ADR-0014)
1. Multi-provider support is mandatory in the control plane:
   - keep `users/devices/user_devices`,
   - add provider-scoped entities (`provider_credentials`, `provider_devices`) for integration identity and metadata.
2. Credential handling:
   - user-facing APIs are write-only for secrets (never return plaintext secret values),
   - multiple credentials per user/provider are allowed,
   - credentials must support `is_active` enable/disable lifecycle.
3. Discovery behavior:
   - `DiscoverDevices()` is manual trigger in v1 (no periodic background discovery),
   - discovered provider devices must be linked to canonical `devices` and user ownership via existing authz primitives.
4. Session/lease behavior:
   - only one active MQTT session globally per `(provider, provider_device_id)`,
   - distributed lock/lease uses Valkey with TTL + heartbeat + token/fencing safety,
   - disable/deactivate flows must use graceful drain (event-driven), not hard stop.
5. Initial dev seeding:
   - explicit command only (no automatic startup seeding),
   - seed from env-provided credentials and configured SN list.
6. Provider-device mapping:
   - do not assume adjacent models share the same quota group layout,
   - verify field families against official provider docs before reusing another model's mapper path,
   - when field families differ, add/keep model-specific regression tests instead of relying on naming similarity.

### Migration safety follow-up (locked direction)
1. After M1 baseline lands, adopt `pgroll` for safe online PostgreSQL schema migrations with reversible rollout and simultaneous multi-schema serving during transitions.

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

### Internal gRPC baseline compliance (ADR-0013)
When touching Go internal API services, enforce the ADR-0013 baseline:
1. Use shared server builder from `internal/grpcserver` (no bespoke grpc.NewServer wiring per service).
2. Always keep these options enabled:
   - keepalive params + enforcement policy,
   - HTTP/2 transport tuning (`MaxConcurrentStreams`, connection/window sizes),
   - explicit message/header limits (`MaxRecvMsgSize`, `MaxSendMsgSize`, `MaxHeaderListSize`).
3. Unary and stream interceptor chains must include, in a consistent order:
   - request-id propagation,
   - recovery,
   - auth hook,
   - structured logging.
4. Reflection is allowed only in `local/dev`; never in `staging/prod`.
5. Health service registration is required for bootstrap/runtime liveness checks.
6. Graceful SIGTERM drain is mandatory (`GracefulStop` with timeout fallback to `Stop`).
7. Protobuf generation is standardized through Buf:
   - source of truth: `proto/`,
   - configs: `buf.yaml`, `buf.gen.yaml`,
   - generated output: `gen/`,
   - command: `buf generate`.
8. After changing proto or grpc server wiring, always run:
   - `buf generate`
   - `go test ./...`
9. Next security hardening step after baseline bootstrap:
   - replace `NoopAuthorizer` with Keycloak JWKS JWT validation,
   - enforce `user_devices` RBAC at Go boundary (no trust-by-proxy from Node).
10. Testing is mandatory for gRPC baseline code:
   - add/maintain regression tests in `internal/grpcserver` and `internal/grpcmw`,
   - add service behavior tests for bootstrap handlers in `cmd/ecoflow-grpc-api`,
   - run `go test ./...` before commit.
11. For performance changes, run workload-calibrated benchmarks before tuning:
   - derive per-device message-rate/payload baselines from `logs/mqtt_payload_raw-*.log`,
   - use these baselines to tune benchmark profiles (steady + burst),
   - do not apply GC tuning blindly.
12. GC tuning policy for gRPC services:
   - profile first (`pprof`, `gctrace`, benchmarks),
   - prefer setting `GOMEMLIMIT` to protect against OOM,
   - increase `GOGC` only if profiles show GC overhead dominates and memory headroom exists,
   - re-validate with the same benchmark profile after each tuning change.
13. Keep a 10k-device synthetic soak gate available (opt-in):
   - command shape: `ECOFLOW_GRPC_10K_SOAK=1 GOGC=200 GOMEMLIMIT=128MiB go test ./cmd/ecoflow-grpc-api -run TestTelemetryServerP99LatencyAndHeapStable10k -count=1 -v`,
   - use env overrides for thresholds: steady p99, burst p99, and heap delta.
14. Locking policy for gRPC hot paths:
   - do not add per-request mutexes unless contention profiling proves necessity,
   - prefer lock-free atomics for monotonic counters/request-id suffixes,
   - if a lock is required, keep critical sections minimal and never hold locks across I/O or channel sends.
15. Goroutine/channel policy for streaming and fanout:
   - use bounded channels and explicit drop/merge/sample behavior for slow consumers,
   - avoid unbounded goroutine creation in request paths and startup bursts,
   - prefer one long-lived worker/fanout loop per shared stream source over per-message goroutines.
16. Allocation policy for throughput-sensitive handlers:
   - keep immutable shared default maps/struct templates for common response fields,
   - avoid repeated variadic slice growth in middleware logging paths,
   - benchmark alloc/op before/after each optimization and keep regressions out.
17. Mandatory perf validation after grpc hot-path changes:
   - run benchmark suites covering observed fleet mix + startup bursts + 10k synthetic scenarios,
   - capture p99 latency and heap delta results and include them in PR validation notes,
   - re-run mutex/block profiles when adding synchronization primitives.

### CI governance (locked by ADR-0010 once accepted)
1. Treat CI gates as architecture controls, not optional repo hygiene.
2. Required checks for merges to `main`:
   - `go-test`
   - `go-test-race-critical`
   - `frontend-ci`
   - `proto-ci`
   - `CodeQL`
3. Keep workflow/check names stable to avoid breaking required-status wiring.
4. When CI workflow scope changes (paths, jobs, check names), update:
   - ruleset/branch protection required checks,
   - relevant architecture docs/ADR status and references.
   - use wrapper/aggregator jobs when sharding is needed so required check names can remain stable.
5. Frontend CI must validate at minimum:
   - `npm run -w apps/universal typecheck`
   - `npm run -w apps/universal lint`
   - `npm run -w apps/universal test`
   - Expo web build/export sanity check
   - Playwright web E2E smoke (`npm run -w apps/universal e2e:web`) with deterministic API route mocking at browser boundary
6. Protobuf contract changes must pass both local and CI lint:
   - local: `make lint` (includes `buf lint` and `actionlint`)
   - CI: `proto-ci` GitHub Actions check (`buf lint`)
7. Node↔Go protobuf compatibility must be validated with runtime contract tests:
   - local: `make test-proto-contract`
   - CI: `frontend-ci` must install Go and run realtime-gateway tests that execute the Go fixture tool (`cmd/proto-contract-fixture`) for cross-language wire compatibility.
8. Any PR that edits `.github/workflows/*.yml` must run local workflow lint before push:
   - preferred: `make lint`
   - minimum: `actionlint`

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

### Dev cloud cost-min policy (ADR-0011 + dev-cost guide)
1. Prime directive (non-negotiable):
   - daily development happens on local k3d,
   - GKE dev is used only for integration/cloud-only validation.
2. Use GKE dev only for:
   - OAuth/social redirect flows on real domains/devices,
   - ingress/TLS/cert-manager validation,
   - workload identity + external secrets behavior,
   - cloud object storage lifecycle/retention checks,
   - autoscaling/node-lifecycle/cloud realism tests.
3. Cost floor for GKE dev:
   - one shared zonal Standard cluster in `us-east1-b` for dev usage,
   - avoid multiple dev clusters unless explicitly approved in architecture decisions.
4. Keep expensive defaults off:
   - no always-on public ingress/load balancers by default,
   - use port-forward for routine debugging where possible,
   - keep observability lite in dev.
5. Mandatory scale-down when idle:
   - scale stateless workloads to 0 or 1 when not actively testing,
   - reduce node pool minimums (baseline low, Spot min=0 where used).
6. Namespace guardrails are required for dev cloud usage:
   - apply ResourceQuota + LimitRange in `pulse-dev` to avoid accidental overprovisioning.
7. Delivery workflow:
   - local-first implementation and testing,
   - promote to GKE dev only for cloud-only validation gates.

### GKE bootstrap and park/wake execution learnings
1. For project billing link in automation scripts, prefer:
   - `gcloud billing projects link <project> --billing-account <account>`
   over beta-only variants.
2. After switching project context, set ADC quota project to avoid drift/warnings:
   - `gcloud auth application-default set-quota-project <project>`.
3. If a cluster is created with raw `gcloud container clusters create`, default nodepool name is typically `default-pool`:
   - run park/wake with `GKE_BASELINE_NODEPOOL=default-pool` unless a custom baseline pool exists.
4. Argo app health wait loops that require `Synced + Healthy` depend on app auto-sync policy:
   - ensure Application manifests include `spec.syncPolicy.automated` (`prune`, `selfHeal`) when using non-interactive `argocd-wait-apps` workflows.
5. Argo app manifests in this repo track `targetRevision: main` by default:
   - `make argocd-dev-up` validates what is on `main`,
   - validate in-flight branch chart changes with local `helm dependency update` + `helm lint` (or explicit branch-targeted app manifests when needed).
6. Preferred explicit pre-merge GKE app validation flow:
   - patch Argo Application `spec.source.targetRevision` to current branch,
   - add `argocd.argoproj.io/refresh=hard`,
   - wait for `Synced + Healthy`,
   - validate expected workloads/CRDs,
   - restore `targetRevision=main` and wait again.

### Kubernetes bringup hardening (local/dev)
1. `make dev-up` must be single-run and self-healing:
   - developers should not need manual reruns to recover transient startup races.
2. For Helm installs that create CRD-backed resources (for example CNPG `Cluster`):
   - implement retry + backoff around `helm upgrade --install`,
   - explicitly wait for operator/webhook readiness before reconcile pass.
3. Enforce dependency order in make targets:
   - platform apply -> platform readiness gates -> services apply -> services readiness gates.
4. Add explicit wait targets and use them in `dev-up`:
   - `kubectl rollout status` for deployments/statefulsets,
   - `kubectl wait --for=condition=Ready` for CRDs (for example CNPG cluster condition).
5. Wait targets must be safe for optional workloads:
   - if a namespace/release has no pods yet, return success with a clear message instead of failing bringup.
6. Keep local Helm dependency work off the hot redeploy path:
   - for code-only local redeploys, do not refresh or rebuild Helm dependencies,
   - for `pulse-platform`, reuse vendored chart packages in `deploy/charts/pulse-platform/charts` and only run `helm dependency build --skip-refresh` when `Chart.yaml` / `Chart.lock` changed or vendored tarballs are missing,
   - for `pulse-services`, skip `helm dependency build` because the chart has no external dependencies.
7. Local image-import targets must refresh running workloads when tags stay constant:
   - if `services-up` or a similar local target rebuilds/imports a `:local` image without changing the tag,
   - it must also restart the affected deployments (or force an equivalent pod-template change) so pods actually run the imported image.

### Valkey ingest lease baseline (ADR-0014)
1. Lease operations must use Lua with token checks and fencing:
   - acquire, renew, release must remain atomic and token-validated.
2. Use cluster-aware keying with ADR hash tags:
   - `pulse:v1:ingest:lease:{provider|provider_device_id}`
   - `pulse:v1:ingest:session:{provider|provider_device_id}`
   - `pulse:v1:ingest:fence:{provider|provider_device_id}`
3. Lock timings are defaults unless explicitly tuned:
   - lease TTL `45s`,
   - heartbeat `15s` with jitter,
   - graceful drain before release on shutdown/deactivation.
4. Use official `valkey-go` for lease manager and keep lock path cache disabled:
   - `DisableCache=true` for lease clients to avoid client-tracking/cache coupling.
5. Lease manager changes must include concurrency-focused tests:
   - single-winner contention,
   - fencing increment on re-acquire,
   - token mismatch rejection,
   - heartbeat + drain/release lifecycle.

### Ingest worker scaling baseline (ADR-0014)
1. Startup/reconcile worker pool defaults are policy-driven:
   - `start_workers = clamp(4*GOMAXPROCS, 8, 64)`
   - `start_queue_size = start_workers * 8`
2. These defaults must remain overridable via env/config:
   - `INGEST_START_WORKERS`
   - `INGEST_START_QUEUE_SIZE`
3. Keep startup work bounded:
   - do not create unbounded goroutines for assignment start bursts,
   - use bounded channels/worker pools and explicit queue caps.
4. HPA policy is two-level by design:
   - in-pod bounded pool handles local bursts first,
   - HPA scales replicas when sustained pressure remains.
5. Recommended HPA baseline (documented manifest):
   - CPU target `65%`, memory target `70%`,
   - fast scale-up, conservative scale-down with stabilization,
   - custom metrics follow-up for unassigned devices + reconcile/lease p95.

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
1. Keep training outputs reproducible and record the exact winning seed/params.
2. Validate with:
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

1. Retrain DPU only when needed:
```bash
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile dpu -candidates 4000 -stages 0.15,0.4,1.0 -seed 88
```

1. Compare against currently deployed params on the same dataset:
   - Only apply new params if `best_score` is lower with equal/better `coverage`.
   - Keep `coverage` near 1.0 for DPU and high for D2M/generic.
   - If results are close, prefer parameter sets that are stable across multiple seeds.

2. Seed sweep guidance:
   - Run 4-8 seeds for the same profile.
   - Choose the winner by:
     1) lowest `best_score`
     2) highest `best_coverage`
     3) simpler/stabler windows

3. Post-update validation:
   - Verify top-state model selection still follows:
     - `New` (device-specific), then `Generic`, then `MPPT`.
   - Verify source icon logic during hybrid charging (AC + solar) updates correctly.
   - Run full tests (`go test ./...`) before commit.

## Solar Panel DB Workflow (including Purchase Links)
Use this when panel metadata/schema changes (for example new fields like `purchase_link`).

1. Treat panel DB updates as end-to-end schema changes:
   - importer (`cmd/ecoflow-panel-db-import`),
   - compact index (`data/solar_panels/solar_panel_specs_v13.index.json`),
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

8. Cross-platform image loading rules (web + iOS):
   - treat brand assets and UI chrome assets as bundled/static first (logos, menu glyphs, icons),
   - for product photos, support bundled fallback for reliability and remote URI only when explicitly configured,
   - always include an error fallback path for fleet summary thumbnails and device cards/details (do not assume URI availability),
   - avoid custom web fetch/blob/object-URL image loops; prefer direct URI rendering with cache-friendly components,
   - if image requests spike, inspect Network for repeated same-filename fetches and verify image source stability per component.

9. iOS layout/safe-area rules:
   - top header/logo rows must account for safe area insets to avoid notch overlap,
   - avoid nested `VirtualizedList` inside plain `ScrollView` with same direction (RN warning, broken windowing),
   - keep list/detail scroll behavior explicit per platform (`FlatList` header pattern on index, dedicated scroll container on detail).

10. Navigation interaction rules:
   - for iOS close/dismiss from detail, prefer `router.back()` when possible so transition direction feels native,
   - keep replace fallback to home route only when stack back is unavailable.

11. Auth-aware realtime client rules:
   - when OIDC is configured, gate both REST queries and websocket connect on persisted auth-store hydration,
   - expose explicit `auth_required` client transport state instead of attempting anonymous realtime fallback,
   - keep websocket lifecycle ownership in the provider/context layer so token refresh/reconnect does not clear per-screen subscriptions.

12. Historical solar metric rules:
   - `solar_generated_wh` from backend storage/query results is authoritative for history/comparison views,
   - UI code must never derive historical solar energy from `pvAvgW` or raw power samples,
   - any solar-history repair, gap-fill, or regeneration logic must live in backend ingestion/query/storage paths, not in React components.

## Realtime Gateway Workflow (M4)
Use this when working on `apps/pulse-realtime-gateway` and the live telemetry delivery path.

1. Preserve the existing Expo websocket contract unless there is an explicit protocol migration:
   - client messages: `subscribe`, `unsubscribe`, `ping`
   - server messages: `telemetry`, `device_status`

2. Enforce access control in both places:
   - gateway-level JWT/noop auth during websocket upgrade,
   - Go gRPC device-level authz at the service boundary.

3. Keep the live data plane split cleanly:
   - authz remains in Go gRPC,
   - initial state comes from Valkey projection snapshots,
   - live deltas/heartbeats come from NATS,
   - buffer deltas/heartbeats until the snapshot is emitted, then flush in order.

4. Websocket contract tests must use buffered message collectors rather than one-shot listeners attached after emits:
   - snapshot-first delivery is often synchronous,
   - tests should queue inbound messages so assertions do not race transport timing.

5. For reconnect behavior:
   - retry only retryable transport/service failures,
   - treat authz/not-found/invalid-argument as terminal,
   - do not reconnect on terminal business failures.

6. Backpressure is a required delivery contract, not a cosmetic optimization:
   - per-device delivery lanes degrade through `fast -> steady -> slow -> key-only -> paused`,
   - recovery must happen automatically after quiet ticks,
   - tests must explicitly cover `key-only` suppression and `paused` recovery.
