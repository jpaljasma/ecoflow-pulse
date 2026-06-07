# QA Memory - Pulse Pi 5 Appliance

## Current focus

- Validate the post-live-rollout Pi docs slice before starting capacity
  burn-in from the refreshed appliance artifact.

## Files to inspect first

- `.codex/tasks/pulse-pi5-appliance.md`
- `docs/architecture/config-06-pi5-appliance.md`
- `Makefile`
- Existing edge collector, archive worker, and deploy tests for touched slices.
- `cmd/pulse-edge-collector/main_test.go`
- `deploy/pulse-edge/pulse-edge-collector.service`
- `docs/how-to/run-pulse-edge-collector.md`
- `proto/pulse/edge/v1/edge.proto`
- `internal/edgecollector/envelope.go`
- `cmd/ecoflow-grpc-api/edge_service_test.go`
- `deploy/appliance/pi5/test-host-prepare.sh`
- `deploy/appliance/pi5/test-install-dry-run.sh`
- `deploy/env/pi/values.platform.yaml`
- `deploy/env/pi/values.services.yaml`
- `deploy/appliance/pi5/test-go-runtime-render.sh`
- `deploy/appliance/pi5/test-release-values-render.sh`
- `deploy/appliance/pi5/pulse-appliance-create-runtime-secret.sh`
- `deploy/env/pi/release.example.yaml`
- `deploy/charts/pulse-services/templates/workers.yaml`
- `docs/how-to/run-pi5-appliance-backup-cutover.md`
- `docs/how-to/run-pi5-appliance-capacity-burn-in.md`
- `docs/reference/commands.md`

## Decisions made

- Plan-only PR validation is Markdown lint.
- 2026-06-04: `make lint` passed for the plan-only PR.
- Runtime slices need targeted tests close to the changed packages.
- Worker shutdown, ingest retry, archive outbox, and BLE retry changes need race
  or failure-mode coverage.
- 2026-06-04: `make appliance-pi-validate` passed for shellcheck, host fstab
  fixture coverage, and Pi Helm lint.
- 2026-06-04: Installer dry-run coverage proves the appliance command path
  reaches host/K3s skips, Helm dependency builds, the Keycloak bootstrap pass,
  runtime-secret preflight, and services Helm apply without needing Pi
  hardware.

## Open risks

- Full appliance acceptance requires real Pi hardware; CI can only validate
  render, fixture, unit, and script behavior.
- Capacity burn-in criteria must be recorded from the target 8GB Pi, not from
  desktop k3d.
- Host script tests cover fstab option merging but not real EEPROM, PCIe, or
  systemd behavior.
- Installer tests are dry-run only; real K3s install, CNPG readiness, Keycloak
  login, and hostPort reachability need appliance hardware.
- Direct gRPC transport needs targeted collector tests and a Pi hardware check
  after the loopback hostPort service is installed.
- Durable retry must prove process restart does not drop pending telemetry and
  must avoid writing collector secrets to disk.
- Archive upload outbox must prove local ACK after SSD fsync, deferred manifest
  writes until remote upload, replay after restart, and fail-closed behavior
  when a manifest-backed entry has no manifest store.
- The cutover runbook must fail closed when the archive upload outbox has
  pending entries and must keep GCS object storage outside hosted shutdown.
- Runtime-cap changes need Helm render validation in addition to lint because
  a syntactically valid chart can still omit the Pi `GOMAXPROCS` /
  `GOMEMLIMIT` entries.
- Live Pi prep found a deploy-blocking scaffold gap: default Pi values still
  render placeholder images, and GCS archive auth needs a mountable local
  credential secret.
- Release-input validation must prove digest image refs render without
  `pi-placeholder`, the services chart renders the GCS credentials mount, and
  the helper scripts pass ShellCheck.
- Image-publishing validation must prove the GitHub workflow is actionlint
  clean and the renderer can produce chart-consumable release values with
  optional pull secrets.
- Live Pi rollout hardening must prove the render no longer needs temporary
  data-plane/Keycloak overrides, migration validation accepts `appliance`, the
  Pi overlay disables `nats-box`, and operator-run status handles NVMe SMART
  permission limits as a warning.
- The docs slice must keep the GitHub CLI install/auth commands current for
  Raspberry Pi OS, document direct artifact download by workflow run ID, and
  make chart dependency caching and CNPG CRD recovery behavior clear enough for
  operator use.

## Next step

- This docs branch passed `make lint`, `git diff --check`, and the duplicate
  editor-backup file scan. After it lands, use the Pi's upgraded status output
  to begin the 24h burn-in.
