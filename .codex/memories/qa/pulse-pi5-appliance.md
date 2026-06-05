# QA Memory - Pulse Pi 5 Appliance

## Current focus

- Validate the Pi image-publishing slice that produces the digest release
  artifact needed before capacity burn-in.

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

## Next step

- Phase 3 archive outbox foundation validation passed with targeted archive
  Go tests, Pi/local services Helm lint, Pi render inspection,
  `make appliance-pi-validate`, targeted archive `go test -race`,
  `make test-race`, `make lint`, and `git diff --check`.
- Review follow-up validation added a restart/offline overwrite regression and
  passed targeted archive Go tests, targeted archive race tests,
  `make test-race`, `make lint`, and `git diff --check`.
- Phase 3 pending-outbox status/rebuild guard validation passed with targeted
  Go tests for the archive outbox counter, status helper, and rollup rebuild
  guard; `make appliance-pi-validate`; targeted race tests; `make test-race`;
  `make lint`; and `git diff --check`.
- Current QA focus is complete for this slice: `make appliance-pi-validate` and
  `make lint` passed. Real 24h burn-in still requires the workflow-published
  arm64 images, the release artifact, local secrets, target Pi hardware, and
  live devices.
