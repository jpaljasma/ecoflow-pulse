# Reference: Repository Layout

Top-level structure:

- `apps/`
  - `pulse-platform`: Node REST BFF workspace (public history/query adapter over internal gRPC).
  - `pulse-realtime-gateway`: Node WebSocket gateway workspace (JWT/noop auth, gRPC authz, Valkey snapshot reads, NATS delta fanout, per-session backpressure ladder).
  - `universal`: Expo universal dashboard (Web/iOS/Android).
- `cmd/`
  - `ecoflow-dev-seed`: explicit local/dev control-plane seeding command (user + provider credentials + initial provider-device bindings).
  - `ecoflow-grpc-api`: internal gRPC API bootstrap server (health + telemetry + control-plane + inference services).
  - `ecoflow-inference-worker`: online insight projection worker (ingest envelopes + control-plane metadata -> Valkey device insights read model).
  - `ecoflow-ingest-worker`: distributed MQTT ingest assignment loop + session runner entrypoint.
  - `ecoflow-rollup-worker`: Timescale rollup pipeline worker (ingest envelopes -> minute/hour/day rollup upserts).
  - `ecoflow-archive-worker`: distributed raw archive writer (JetStream ingest -> protobuf+zstd objects).
  - `ecoflow-replay-cli`: archive replay CLI (device listing + per-device/fleet shard-time replay modes).
  - `ecoflow-loadtest-ingest-bridge`: local load-test helper that accepts HTTP ingest payloads and publishes canonical telemetry envelopes to NATS.
  - `ecoflow-gap-detector`: projection lag detector that enqueues targeted replay jobs.
  - `ecoflow-gap-repair-worker`: gap-repair queue consumer that replays missing windows back into ingest subjects.
  - `ecoflow-mqtt-sub`: live MQTT dashboard and telemetry processing runtime.
  - `ecoflow-pv-fingerprint`: PV feature extraction from training telemetry CSV.
  - `ecoflow-panel-select-train`: panel selection model training + replay.
  - `ecoflow-smoke`: smoke checks against EcoFlow API.
  - `ecoflow-server`: API server entrypoint.
- `pkg/`
  - `ecoflow`: core API client and signing.
  - `ecoflowmqtt`: MQTT subscriber primitives.
  - `panelselect`: panel selection model, feature tracker, and predictor.
  - `ecoflowserver`: server helpers and middleware.
  - `logger`: structured logging package.
- `internal/`
  - `controlplane`: control-plane store abstractions and implementations (Postgres + in-memory).
  - `inference`: online inference read-model store, derivation logic, control-plane context resolver, and worker runtime.
  - `ingestworker`: distributed assignment poller/reconciler + provider session lifecycle manager.
  - `archiveworker`: archive pipeline primitives (durable ingest consumer + shard/hour batching + MinIO-compatible object writer).
  - `rollupworker`: rollup pipeline primitives (durable ingest consumer + explicit metric extraction + Timescale upserts).
  - `replaycli`: manifest/object replay runtime (manifest query + object decode + replay publish runner).
  - `gaprepair`: projection lag detection + replay queue consumer/publisher primitives.
  - `grpcserver`: standardized gRPC server builder (keepalive, HTTP/2 tuning, graceful shutdown).
  - `grpcmw`: standard gRPC middleware chain scaffolding (request-id, logging, recovery, auth hook).
  - `telemetrybus`: deterministic NATS subject + shard routing helpers for M2 ingest/replay paths.
- `proto/`
  - `pulse/controlplane/v1/control_plane.proto`: control-plane gRPC contract (`Create/List/Activate credentials`, `ListDevices`, `DiscoverDevices`).
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
  - `env/local` and `env/dev`: values files for local/dev deploys.
  - `env/dev/recommended`: recommended (non-auto-applied) runtime policies,
    including ingest worker HPA baseline manifests.
  - `argocd/apps`: direct Argo CD apps (`pulse-platform`, `pulse-services`).
  - `tilt/k3d-config.yaml`: k3d local cluster config.
- `docs/`: developer documentation in Diataxis layout.
- `load/k6/`: k6 load-test harness (ingest publish + websocket fanout + history query scenarios for M5 validation).

Key dashboard-focused files:

- `cmd/ecoflow-mqtt-sub/main.go`: entrypoint and orchestration.
- `cmd/ecoflow-mqtt-sub/mqtt_runtime.go`: MQTT connect/reconnect/read runtime.
- `cmd/ecoflow-mqtt-sub/file_locking.go`: safe append sinks and per-device lock files.
- `cmd/ecoflow-mqtt-sub/ui_async.go`: asynchronous UI output writer with bounded queue.
- `cmd/ecoflow-mqtt-sub/viewmodel.go`: dashboard projection logic.
- `cmd/ecoflow-mqtt-sub/renderer.go`: ASCII table rendering.
- `cmd/ecoflow-mqtt-sub/estimates.go`: ETA and ML estimation logic.
- `cmd/ecoflow-mqtt-sub/formatters.go`: display and unit formatting helpers.
- `cmd/ecoflow-mqtt-sub/*_logic.go`: domain-specific mapping helpers
  (battery, PV, MPPT).
