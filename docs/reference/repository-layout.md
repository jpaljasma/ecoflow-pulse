# Reference: Repository Layout

Top-level structure:

- `apps/`
  - `pulse-platform`: Node REST BFF workspace (public device/history/inference adapter over internal gRPC).
  - `pulse-realtime-gateway`: Node WebSocket gateway workspace (JWT/noop auth, gRPC authz, Valkey snapshot reads, NATS delta fanout, per-session backpressure ladder).
  - `universal`: Expo universal dashboard (Web/iOS/Android).
- `cmd/`
  - `ecoflow-db-migrate-job`: forward-only in-cluster schema migration runner for Helm/Argo rollout hooks.
  - `ecoflow-dev-seed`: explicit local/dev control-plane seeding command (user + provider credentials + initial provider-device bindings).
  - `ecoflow-grpc-api`: internal gRPC API bootstrap server reused for both `telemetry` mode (health + telemetry + control-plane + inference services) and `energy` mode (health + energy/history services).
  - `ecoflow-inference-worker`: online insight projection worker (ingest envelopes + control-plane metadata -> Valkey device insights read model).
  - `ecoflow-ingest-worker`: distributed MQTT ingest assignment loop + session runner entrypoint.
  - `ecoflow-user-subject-reconcile`: one-shot verified-email bootstrap helper for remapping restored users onto a new Keycloak subject after cloud auth cutover.
  - `ecoflow-rollup-worker`: Timescale rollup pipeline worker (ingest envelopes -> minute/hour/day rollup upserts).
  - `ecoflow-archive-worker`: distributed raw archive writer (JetStream ingest -> protobuf+zstd objects in provider-aware object storage).
  - `ecoflow-replay-cli`: archive replay CLI (device listing + per-device/fleet shard-time replay modes over provider-aware object storage).
  - `ecoflow-loadtest-ingest-bridge`: local load-test helper that accepts HTTP ingest payloads and publishes canonical telemetry envelopes to NATS.
  - `ecoflow-gap-detector`: projection lag detector that enqueues targeted replay jobs.
  - `ecoflow-gap-repair-worker`: gap-repair queue consumer that replays missing windows back into ingest subjects.
  - `ecoflow-pv-fingerprint`: PV feature extraction from training telemetry CSV.
  - `ecoflow-panel-select-train`: panel selection model training + replay.
  - `ecoflow-smoke`: smoke checks against EcoFlow API.
  - `ecoflow-server`: API server entrypoint.
- `pkg/`
  - `ecoflow`: core API client and signing.
  - `ecoflowmqtt`: MQTT subscriber primitives.
  - `panelselect`: panel selection model, feature tracker, and predictor.
  - `ecoflowserver`: server helpers and middleware.
  - `logger`: structured logging package with bounded async queueing and queue-depth metrics.
- `internal/`
  - `controlplane`: control-plane store abstractions and implementations (Postgres + in-memory).
  - `dbmigrate`: forward-only SQL migration loader/runner used by rollout jobs.
  - `hashutil`: shared non-cryptographic hashing helpers; default internal hash home for `XXH3_128`.
  - `inference`: online inference read-model store, derivation logic, control-plane context resolver, and worker runtime.
  - `pgsearchpath`: shared Postgres DSN helper for versioned `search_path` cutovers.
  - `ingestworker`: distributed assignment poller/reconciler + provider session lifecycle manager.
  - `provideradapter`: provider discovery/certification clients plus registry-backed adapter wiring for control-plane and ingest runtime dispatch.
  - `archiveworker`: archive pipeline primitives (durable ingest consumer + shard/hour batching + provider-aware object-store writer).
  - `rollupworker`: rollup pipeline primitives (durable ingest consumer + explicit metric extraction + Timescale upserts).
  - `replaycli`: manifest/object replay runtime (manifest query + object decode + replay publish runner over MinIO or GCS).
  - `gaprepair`: projection lag detection + replay queue consumer/publisher primitives.
  - `grpcserver`: standardized gRPC server builder (keepalive, HTTP/2 tuning, graceful shutdown).
  - `grpcmw`: standard gRPC middleware chain scaffolding (request-id, logging, recovery, auth hook).
  - `telemetrybus`: deterministic NATS subject + shard routing helpers for M2 ingest/replay paths, plus shared JetStream handler-drain tracking used during worker shutdown.
- `proto/`
  - `pulse/controlplane/v1/control_plane.proto`: control-plane gRPC contract (`Create/List/Update/Activate credentials`, `ListDevices`, `DiscoverDevices`, MQTT validation-backed provider enablement, and inactive provider-device import).
  - `pulse/inference/v1/inference.proto`: online inference gRPC contract (`GetDeviceInsights`, `ListFleetInsights`).
  - `pulse/telemetry/v1/telemetry.proto`: telemetry gRPC contract (snapshot + server-stream updates).
  - `pulse/envelope/v1/envelope.proto`: canonical ingest/archive `TelemetryEnvelope` contract.
  - `pulse/replay/v1/replay.proto`: targeted replay request contract (`GapRepairRequest`).
- `gen/`
  - generated protobuf/gRPC Go stubs (via `buf generate`).
- `buf.yaml`, `buf.gen.yaml`
  - protobuf module and generation configuration for reproducible codegen.
- `logs/`
  - `mqtt.log`: run log and raw payload stream.
  - `telemetry_history.jsonl`: minute telemetry persistence.
  - `pv_fingerprint.csv`: generated per-port PV fingerprint features.
- `data/solar_panels/`
  - `solar_panel_specs_v13.index.json`: compact panel capabilities index
    with derived fields such as `module_efficiency_pct` and
    `module_efficiency_source` (`reported`, `notes`, `estimated_*`), and
    `purchase_link`.
  - `panel_purchase_links_v13.json`: curated panel-id to purchase-link override map
    used during panel DB regeneration.
  - `panel_select_model.json`: trained panel selection model artifact.
- `data/ev/`
  - `ev_us_europe_database.json`: stored U.S.+Europe EV reference dataset used
    for future range/consumption equivalence work and current premium-EV miles
    baseline derivation.
- `deploy/`
  - `charts/pulse-platform`: platform umbrella chart scaffold.
    - includes chart-managed bootstrap resources such as CNPG contracts and
      Keycloak realm/provider bootstrap (`templates/keycloak-*.yaml`).
  - `charts/pulse-services`: services umbrella chart scaffold.
  - `db/migrations`: control-plane SQL migrations (M1+ schema evolution).
  - `db/pgroll/plans`: reserved location for future `pgroll` migration plans.
  - `env/local`, `env/dev`, and `env/cloud`: values files for local, cost-min dev, and hosted cloud deploys.
  - `env/dev/recommended`: recommended (non-auto-applied) runtime policies,
    including ingest worker HPA baseline manifests and migration-hook values.
  - `argocd/apps`: direct Argo CD apps for both dev and cloud (`pulse-platform`, `pulse-services`, `pulse-platform-cloud`, `pulse-services-cloud`).
  - `tilt/k3d-config.yaml`: k3d local cluster config.
- `docs/`: developer documentation in Diataxis layout.
- `load/k6/`: k6 load-test harness (ingest publish + websocket fanout + history query scenarios for M5 validation).

Key dashboard-focused files:

- `apps/universal/app/index.tsx`: universal fleet dashboard route.
- `apps/universal/src/shared/providers/AppProvider.tsx`: root providers, theme, and query wiring.
- `apps/pulse-platform/src/routes/devices.ts`: public REST device endpoints consumed by the app.
- `apps/pulse-realtime-gateway/src/server.ts`: websocket gateway bootstrap and auth mode wiring.
- `apps/pulse-realtime-gateway/src/live/*`: realtime snapshot + delta delivery pipeline.

Hashing conventions:

- Default new internal non-cryptographic hashes to `XXH3_128`, preferably through `internal/hashutil`.
- Keep SHA-2/HMAC hashing only for cryptographic/security-sensitive flows or when an external contract explicitly requires that algorithm.
