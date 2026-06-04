# Developer Documentation

This project uses the Diataxis framework for documentation structure.

- Tutorials: learning-oriented, step-by-step guides.
- How-to guides: task-oriented instructions for specific outcomes.
- Reference: factual, lookup-style documentation.
- Explanation: conceptual context and design rationale.

## Documentation Map

### Tutorials

- [`tutorials/getting-started-local.md`](tutorials/getting-started-local.md)

### How-to Guides

Provider integrations include local/provider seeding, the Pulse MQTT emulator,
Pecron E1000LFP cloud, and Anker SOLIX Cloud MQTT onboarding.

- [`how-to/configure-environment.md`](how-to/configure-environment.md)
- [`how-to/add-solar-panel-to-db.md`](how-to/add-solar-panel-to-db.md)
- [`how-to/generate-pv-fingerprints.md`](how-to/generate-pv-fingerprints.md)
- [`how-to/train-panel-select-model.md`](how-to/train-panel-select-model.md)
- [`how-to/maintain-ci-required-checks.md`](how-to/maintain-ci-required-checks.md)
- [`how-to/setup-gke-dev-project.md`](how-to/setup-gke-dev-project.md)
- [`how-to/configure-keycloak-social-providers-local.md`](how-to/configure-keycloak-social-providers-local.md)
- [`how-to/seed-local-provider-data.md`](how-to/seed-local-provider-data.md)
- [`how-to/pulse-mqtt-emulator-local.md`](how-to/pulse-mqtt-emulator-local.md)
- [`how-to/discover-ecoflow-ble-devices.md`](how-to/discover-ecoflow-ble-devices.md)
- [`how-to/run-pulse-edge-collector.md`](how-to/run-pulse-edge-collector.md)
- [`how-to/configure-pecron-cloud.md`](how-to/configure-pecron-cloud.md)
- [`how-to/configure-anker-solix-cloud-mqtt.md`](how-to/configure-anker-solix-cloud-mqtt.md)
- [`how-to/dr-backup-restore-local.md`](how-to/dr-backup-restore-local.md)
- [`how-to/run-pi5-appliance-backup-cutover.md`](how-to/run-pi5-appliance-backup-cutover.md)
- [`how-to/rollout-schema-migrations-dev-staging-prod.md`](how-to/rollout-schema-migrations-dev-staging-prod.md)
- [`how-to/prepare-pgroll-transition-local.md`](how-to/prepare-pgroll-transition-local.md)
- [`how-to/migrate-local-data-plane-to-gke-cloud.md`](how-to/migrate-local-data-plane-to-gke-cloud.md)
- [`how-to/reduce-hosted-gke-cost.md`](how-to/reduce-hosted-gke-cost.md)
- [`how-to/no-planned-outage-cloud-cost-rollout.md`](how-to/no-planned-outage-cloud-cost-rollout.md)

### Reference

- [`reference/repository-layout.md`](reference/repository-layout.md)
- [`reference/commands.md`](reference/commands.md)
- [`reference/local-dev-prerequisites.md`](reference/local-dev-prerequisites.md)
- [`reference/configuration.md`](reference/configuration.md)
- [`reference/telemetry-model.md`](reference/telemetry-model.md)
- [`reference/solar-avoided-emissions.md`](reference/solar-avoided-emissions.md)
- [`reference/tree-equivalent.md`](reference/tree-equivalent.md)
- [`reference/ev-us-europe-database-report.md`](reference/ev-us-europe-database-report.md)

### Explanation

- [`explanation/architecture.md`](explanation/architecture.md)
- [`architecture/config-06-pi5-appliance.md`](architecture/config-06-pi5-appliance.md)
- [`explanation/telemetry-and-estimation.md`](explanation/telemetry-and-estimation.md)
- [`explanation/ui-visual-system.md`](explanation/ui-visual-system.md)

## Recommended Reading Order

- [`tutorials/getting-started-local.md`](tutorials/getting-started-local.md)
- [`reference/configuration.md`](reference/configuration.md)
- [`explanation/architecture.md`](explanation/architecture.md)
