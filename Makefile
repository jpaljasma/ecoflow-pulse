SHELL := $(shell command -v zsh 2>/dev/null || command -v bash 2>/dev/null || echo /bin/sh)

GO ?= go
NPM ?= npm
WEB_PORT ?= 8081
K6 ?= k6
K3D ?= k3d
HELM ?= helm
KUBECTL ?= kubectl
DOCKER ?= docker
PGROLL ?= pgroll
PGROLL_REQUIRED ?= 0
DOCKER_BUILDKIT ?= 1
LOCAL_IMAGE_PLATFORM ?= $(shell arch="$$(uname -m)"; if [ "$$arch" = "arm64" ] || [ "$$arch" = "aarch64" ]; then printf 'linux/arm64'; elif [ "$$arch" = "x86_64" ] || [ "$$arch" = "amd64" ]; then printf 'linux/amd64'; else printf 'linux/amd64'; fi)
CLOUD_IMAGE_PLATFORM ?= linux/amd64
CLOUD_ARTIFACT_REGISTRY_HOST ?= us-east1-docker.pkg.dev
DOCKER_CONFIG_LOCAL ?= $(CURDIR)/.tmp/docker-noauth
DOCKER_BUILDX_CONFIG_LOCAL ?= $(CURDIR)/.tmp/docker-buildx
GCLOUD ?= gcloud
CODESIGN ?= codesign
LOCAL_PLATFORM_AUTO_TRUST_TLS ?= 1
K3D_CLUSTER_NAME ?= pulse-local
K3D_CONTEXT ?= k3d-$(K3D_CLUSTER_NAME)
K3D_SET_CURRENT_CONTEXT ?= 0
K3D_CONFIG ?= deploy/tilt/k3d-config.yaml
PLATFORM_CHART ?= deploy/charts/pulse-platform
SERVICES_CHART ?= deploy/charts/pulse-services
LOCAL_PLATFORM_VALUES ?= deploy/env/local/values.platform.yaml
LOCAL_SERVICES_VALUES ?= deploy/env/local/values.services.yaml
PI_PLATFORM_VALUES ?= deploy/env/pi/values.platform.yaml
PI_SERVICES_VALUES ?= deploy/env/pi/values.services.yaml
APPLIANCE_PI_INSTALL_ARGS ?=
CLOUD_PLATFORM_VALUES ?= deploy/env/cloud/values.platform.yaml
CLOUD_SERVICES_VALUES ?= deploy/env/cloud/values.services.yaml
CLOUD_COST_MIN_PLATFORM_VALUES ?= deploy/env/cloud/values.platform.cost-min.yaml
CLOUD_COST_MIN_SERVICES_VALUES ?= deploy/env/cloud/values.services.cost-min.yaml
PLATFORM_RELEASE ?= pulse-platform
SERVICES_RELEASE ?= pulse-services
CLOUD_PLATFORM_RELEASE ?= pulse-platform-cloud
CLOUD_SERVICES_RELEASE ?= pulse-services-cloud
PLATFORM_NAMESPACE ?= pulse-platform
SERVICES_NAMESPACE ?= pulse-services
DELETE_CLUSTER ?= 0
WAIT_TIMEOUT ?= 600s
LOCAL_ROLLOUT_WAIT_MODE ?= available
LOCAL_ROLLOUT_STATUS = LOCAL_ROLLOUT_WAIT_MODE="$(LOCAL_ROLLOUT_WAIT_MODE)" KUBECTL="$(KUBECTL)" sh scripts/local-rollout-status.sh "$(K3D_CONTEXT)"
LOCAL_ROLLOUT_STATUS_STRICT = LOCAL_ROLLOUT_WAIT_MODE=strict KUBECTL="$(KUBECTL)" sh scripts/local-rollout-status.sh "$(K3D_CONTEXT)"
HELM_RETRY_MAX ?= 6
HELM_RETRY_DELAY_SEC ?= 5
GKE_PROJECT_ID ?=
GKE_CLUSTER_NAME ?= pulse-dev
GKE_CLUSTER_ZONE ?= us-east1-b
GKE_CLOUD_PROJECT_ID ?= ecoflow-pulse-dev-260221-01
GKE_CLOUD_CLUSTER_NAME ?= pulse-cloud
GKE_CLOUD_CLUSTER_REGION ?= us-east1
GKE_DEV_NAMESPACE ?= pulse-dev
GKE_GUARDRAILS_DIR ?= deploy/env/dev/guardrails
GKE_BASELINE_NODEPOOL ?= baseline-pool
GKE_SPOT_NODEPOOL ?= spot-pool
GKE_STATELESS_DEPLOYMENTS ?= node-bff ws-gateway query-api projection ingest
GKE_WAKE_REPLICAS ?= 2
GKE_PARK_REPLICAS ?= 0
GKE_BASELINE_MIN_ACTIVE ?= 2
GKE_BASELINE_MIN_PARKED ?= 1
GKE_BASELINE_MAX ?= 4
GKE_SPOT_MIN ?= 0
GKE_SPOT_MAX ?= 4
ARGOCD_HELM_REPO ?= https://argoproj.github.io/argo-helm
ARGOCD_HELM_CHART ?= argo/argo-cd
ARGOCD_CHART_VERSION ?= 9.4.3
ARGOCD_RELEASE ?= argocd
ARGOCD_NAMESPACE ?= argocd
ARGOCD_VALUES_DEV ?= deploy/env/dev/values.argocd.yaml
ARGOCD_VALUES_CLOUD ?= deploy/env/cloud/values.argocd.yaml
ARGOCD_APPS_DIR ?= deploy/argocd/apps
ARGOCD_APPS ?= pulse-platform pulse-services
ARGOCD_CLOUD_APPS ?= pulse-platform-cloud pulse-services-cloud
ARGOCD_APP_WAIT_ATTEMPTS ?= 60
ARGOCD_APP_WAIT_SLEEP_SEC ?= 10
DB_MIGRATIONS_DIR ?= deploy/db/migrations
DB_MIGRATION_NAMESPACE ?= pulse-platform
DB_MIGRATION_CLUSTER ?= pulse-platform-core
DB_MIGRATION_SECRET ?= pulse-platform-core-app
DB_MIGRATION_DB ?= pulse
PGROLL_LOCAL_PORT ?= 15433
DB_MIGRATION_LOCAL_PORT ?= 15434
CLOUD_DB_LOCAL_PORT ?= 25432
CLOUD_DB_FORWARD_ADDRESS ?= 127.0.0.1
CLOUD_DB_ENV_FILE ?= .tmp/cloud-postgres.env
CLOUD_DB_FORWARD_PID_FILE ?= .tmp/cloud-db-forward.pid
CLOUD_DB_FORWARD_LABEL ?= com.ecoflow-pulse.cloud-db-forward
CLOUD_DB_FORWARD_CONTAINER ?= ecoflow-pulse-cloud-db-forward
CLOUD_FORWARD_IMAGE ?= ecoflow-pulse/cloud-forward:local
CLOUD_FORWARD_DOCKERFILE ?= deploy/docker/cloud-forward.Dockerfile
CLOUD_FORWARD_DOCKER_PLATFORM ?= linux/amd64
CLOUD_FORWARD_RESTART ?= unless-stopped
CLOUD_FORWARD_KUBECONFIG_DIR ?= $(HOME)/.kube
CLOUD_FORWARD_GCLOUD_CONFIG_DIR ?= $(HOME)/.config/gcloud
CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC ?= 10
CLOUD_FORWARD_SUPERVISOR_RESTART_DELAY_SEC ?= 2
CLOUD_FORWARD_SUPERVISOR_STARTUP_GRACE_SEC ?= 10
CLOUD_FORWARD_SUPERVISOR_FAILURE_THRESHOLD ?= 2
CLOUD_REALTIME_FORWARD_ADDRESS ?= 127.0.0.1
CLOUD_NATS_LOCAL_PORT ?= 24222
CLOUD_VALKEY_LOCAL_PORT ?= 26380
CLOUD_NATS_SERVICE ?= $(CLOUD_PLATFORM_RELEASE)-nats
CLOUD_VALKEY_SERVICE ?= $(CLOUD_PLATFORM_RELEASE)-valkey
CLOUD_NATS_FORWARD_PID_FILE ?= .tmp/cloud-nats-forward.pid
CLOUD_NATS_FORWARD_LABEL ?= com.ecoflow-pulse.cloud-nats-forward
CLOUD_NATS_FORWARD_CONTAINER ?= ecoflow-pulse-cloud-nats-forward
CLOUD_VALKEY_FORWARD_PID_FILE ?= .tmp/cloud-valkey-forward.pid
CLOUD_VALKEY_FORWARD_LABEL ?= com.ecoflow-pulse.cloud-valkey-forward
CLOUD_VALKEY_FORWARD_CONTAINER ?= ecoflow-pulse-cloud-valkey-forward
LOCAL_CLOUD_DB_FORWARD_ADDRESS ?= 0.0.0.0
LOCAL_CLOUD_DB_HOST ?= host.docker.internal
LOCAL_CLOUD_DB_PORT ?= $(CLOUD_DB_LOCAL_PORT)
LOCAL_CLOUD_DB_SERVICES_VALUES ?= .tmp/local-cloud-db.services.values.yaml
LOCAL_CLOUD_REALTIME_FORWARD_ADDRESS ?= 0.0.0.0
LOCAL_CLOUD_REALTIME_HOST ?= host.docker.internal
LOCAL_CLOUD_NATS_PORT ?= $(CLOUD_NATS_LOCAL_PORT)
LOCAL_CLOUD_VALKEY_PORT ?= $(CLOUD_VALKEY_LOCAL_PORT)
LOCAL_CLOUD_REALTIME_PLATFORM_VALUES ?= .tmp/local-cloud-realtime.platform.values.yaml
DEV_DEPLOY_DATA_MODE ?= auto
CLOUD_KUBE_CONTEXT ?= gke_$(GKE_CLOUD_PROJECT_ID)_$(GKE_CLOUD_CLUSTER_REGION)_$(GKE_CLOUD_CLUSTER_NAME)
CLOUD_HELM_TAKE_OWNERSHIP ?= 1
CLOUD_HELM_SERVER_SIDE ?= true
CLOUD_HELM_FORCE_CONFLICTS ?= 1
CLOUD_HELM_FORCE_CONFLICTS_ARGS = $(if $(filter 1 true TRUE yes YES,$(CLOUD_HELM_FORCE_CONFLICTS)),--force-conflicts,)
CLOUD_HELM_EXTRA_ARGS ?=
CLOUD_PLATFORM_HELM_SET_ARGS ?=
CLOUD_SERVICES_HELM_SET_ARGS ?=
PGROLL_PLAN ?=
DB_SEED_LOCAL_PORT ?= 15432
DB_SEED_USER_SUBJECT ?= dev-user@example.com
DB_SEED_USER_EMAIL ?= dev-user@example.com
DB_SEED_SERIALS ?= DEMOD2M00001057,DEMODPU0000294
KEYCLOAK_REALM_NAME ?= pulse
KEYCLOAK_ADMIN_USER ?= admin
VALKEY_BENCH_NAMESPACE ?= pulse-platform
VALKEY_BENCH_SERVICE ?= pulse-platform-valkey
VALKEY_BENCH_LOCAL_PORT ?= 6389
VALKEY_BENCH_ADDRS ?= 127.0.0.1:$(VALKEY_BENCH_LOCAL_PORT)
ARCHIVE_INTEGRATION_NAMESPACE ?= pulse-platform
ARCHIVE_INTEGRATION_SERVICE ?= pulse-platform-minio
ARCHIVE_INTEGRATION_SECRET ?= pulse-platform-minio
ARCHIVE_INTEGRATION_LOCAL_PORT ?= 9000
ARCHIVE_INTEGRATION_ENDPOINT ?= 127.0.0.1:$(ARCHIVE_INTEGRATION_LOCAL_PORT)
DR_BACKUP_ROOT ?= $(CURDIR)/.tmp/dr-backups
DR_BACKUP_NAME ?= latest
DR_BACKUP_DIR ?= $(DR_BACKUP_ROOT)/$(DR_BACKUP_NAME)
DR_REPORT_FILE ?= $(DR_BACKUP_DIR)/report.env
DR_ARCHIVE_BUCKET ?= pulse-telemetry-raw
DR_MINIO_LOCAL_PORT ?= 19000
DR_MINIO_ENDPOINT ?= 127.0.0.1:$(DR_MINIO_LOCAL_PORT)
DR_MINIO_DOCKER_ENDPOINT ?= host.docker.internal:$(DR_MINIO_LOCAL_PORT)
DR_MINIO_MC_IMAGE ?= minio/mc:latest
SERVICES_IMAGE_REPO ?= ecoflow-pulse/services
SERVICES_IMAGE_TAG ?= local
SERVICES_IMAGE ?= $(SERVICES_IMAGE_REPO):$(SERVICES_IMAGE_TAG)
SERVICES_CLOUD_IMAGE_REPO ?= us-east1-docker.pkg.dev/$(GKE_CLOUD_PROJECT_ID)/ecoflow-pulse/services
SERVICES_CLOUD_IMAGE_TAG ?= cloud-latest
SERVICES_CLOUD_IMAGE ?= $(SERVICES_CLOUD_IMAGE_REPO):$(SERVICES_CLOUD_IMAGE_TAG)
SERVICES_IMAGE_DOCKERFILE ?= deploy/docker/pulse-services.Dockerfile
PLATFORM_APP_IMAGE_REPO ?= ecoflow-pulse/pulse-platform
PLATFORM_APP_IMAGE_TAG ?= local
PLATFORM_APP_IMAGE ?= $(PLATFORM_APP_IMAGE_REPO):$(PLATFORM_APP_IMAGE_TAG)
PLATFORM_APP_CLOUD_IMAGE_REPO ?= us-east1-docker.pkg.dev/$(GKE_CLOUD_PROJECT_ID)/ecoflow-pulse/pulse-platform
PLATFORM_APP_CLOUD_IMAGE_TAG ?= cloud-latest
PLATFORM_APP_CLOUD_IMAGE ?= $(PLATFORM_APP_CLOUD_IMAGE_REPO):$(PLATFORM_APP_CLOUD_IMAGE_TAG)
PLATFORM_APP_IMAGE_DOCKERFILE ?= deploy/docker/pulse-platform.Dockerfile
PLATFORM_APP_BUILD_ARG_VARS ?= EXPO_PUBLIC_API_URL EXPO_PUBLIC_WS_URL EXPO_PUBLIC_OIDC_ISSUER_URL EXPO_PUBLIC_OIDC_CLIENT_ID EXPO_PUBLIC_OIDC_AUDIENCE EXPO_PUBLIC_OIDC_SCOPES EXPO_PUBLIC_CLOUD_API_URL EXPO_PUBLIC_CLOUD_WS_URL EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE EXPO_PUBLIC_CLOUD_OIDC_SCOPES EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE EXPO_PUBLIC_LOCAL_DATA_PLANE
REALTIME_GATEWAY_IMAGE_REPO ?= ecoflow-pulse/pulse-realtime-gateway
REALTIME_GATEWAY_IMAGE_TAG ?= local
REALTIME_GATEWAY_IMAGE ?= $(REALTIME_GATEWAY_IMAGE_REPO):$(REALTIME_GATEWAY_IMAGE_TAG)
REALTIME_GATEWAY_CLOUD_IMAGE_REPO ?= us-east1-docker.pkg.dev/$(GKE_CLOUD_PROJECT_ID)/ecoflow-pulse/pulse-realtime-gateway
REALTIME_GATEWAY_CLOUD_IMAGE_TAG ?= cloud-latest
REALTIME_GATEWAY_CLOUD_IMAGE ?= $(REALTIME_GATEWAY_CLOUD_IMAGE_REPO):$(REALTIME_GATEWAY_CLOUD_IMAGE_TAG)
REALTIME_GATEWAY_IMAGE_DOCKERFILE ?= deploy/docker/pulse-realtime-gateway.Dockerfile
SERVICES_AUTO_BUILD_IMAGE ?= 1
DEV_DEPLOY_HELM ?= auto
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOFLAGS ?= -tags=moderncompress -mod=mod
LDFLAGS ?=
ECOFLOW_BLE_DISCOVER_BIN ?= bin/ecoflow-ble-discover
ECOFLOW_BLE_DISCOVER_ARGS ?= -duration=20s
ECOFLOW_BLE_DISCOVER_PLIST ?= cmd/ecoflow-ble-discover/Info.plist
ECOFLOW_BLE_DISCOVER_RUN ?= 1
PULSE_EDGE_COLLECTOR_BIN ?= bin/pulse-edge-collector
PULSE_EDGE_PI5_BIN_DIR ?= bin/linux-arm64
PULSE_EDGE_PI5_BUNDLE_DIR ?= .tmp/pulse-edge-pi5-linux-arm64
PULSE_EDGE_PI5_BUNDLE ?= .tmp/pulse-edge-pi5-linux-arm64.tar.gz
PULSE_EDGE_PI5_GOARM64 ?= v8.2
PULSE_EDGE_PI5_CGO_ENABLED ?= 0
PULSE_EDGE_PI5_LDFLAGS ?= -s -w
RACE_CRITICAL_PKGS ?= ./internal/ingestworker ./internal/ingestlease ./internal/projectionworker ./internal/archiveworker ./internal/telemetrybus ./internal/edgecollector ./cmd/ecoflow-grpc-api ./cmd/pulse-edge-collector
RACE_STRESS_COUNT ?= 5
LOCAL_KUBECTL = $(KUBECTL) --context $(K3D_CONTEXT)
LOCAL_HELM = $(HELM) --kube-context $(K3D_CONTEXT)
LOCAL_HELM_UPGRADE_FLAGS ?= --server-side=true --force-conflicts
LOCAL_PLATFORM_HELM_VALUES_ARGS = $(foreach values,$(LOCAL_PLATFORM_VALUES),-f $(values))
PLATFORM_HELM_APPLY = $(LOCAL_HELM) upgrade --install $(PLATFORM_RELEASE) $(PLATFORM_CHART) --namespace $(PLATFORM_NAMESPACE) --create-namespace $(LOCAL_HELM_UPGRADE_FLAGS) $(LOCAL_PLATFORM_HELM_VALUES_ARGS)
LOCAL_PLATFORM_MANIFEST ?= $(CURDIR)/.tmp/pulse-platform.rendered.yaml
K6_SCRIPT ?= load/k6/main.js
K6_API_BASE_URL ?= http://127.0.0.1
K6_WS_URL ?= ws://127.0.0.1/ws
K6_USER_SUBJECT ?= dev-user@example.com
K6_DURATION ?= 1m
K6_INGEST_RATE ?= 20
K6_INGEST_PRE_ALLOCATED_VUS ?= 8
K6_INGEST_MAX_VUS ?= 32
K6_QUERY_RATE ?= 1
K6_QUERY_PRE_ALLOCATED_VUS ?= 2
K6_QUERY_MAX_VUS ?= 8
K6_WS_VUS ?= 20
GRPC_LOAD_BENCH_TIME ?= 3s
GRPC_LOAD_10K_BENCH_TIME ?= 2s
K6_WS_SESSION_TIMEOUT_MS ?= 4000
K6_WS_POST_TELEMETRY_HOLD_MS ?= 200
K6_WS_THINK_TIME_MS ?= 100
K6_SEED_COUNT ?= 5
K6_QUERY_WINDOW_MS ?= 900000
K6_QUERY_RESOLUTION ?= minute
K6_INGEST_P95_MS ?= 750
K6_QUERY_P95_MS ?= 1200
K6_WS_SUCCESS_RATE ?= 0.95
K6_DEVICE_ID ?=
K6_DEVICE_SERIAL_NUMBER ?=
K6_NATS_NAMESPACE ?= pulse-platform
K6_NATS_SERVICE ?= pulse-platform-nats
K6_NATS_LOCAL_PORT ?= 14222
K6_INGEST_BRIDGE_ADDR ?= 127.0.0.1:19090
K6_TELEMETRY_SUBJECT_PREFIX ?= pulse
REGEN_DB_LOCAL_PORT ?= 15433
REGEN_NATS_LOCAL_PORT ?= 14223
REGEN_MINIO_LOCAL_PORT ?= 19001
REGEN_FROM ?=
REGEN_TO ?=
REGEN_MAX_OBJECTS ?= 0

export GOCACHE
export GOMODCACHE
export GOFLAGS

CMDS := $(patsubst cmd/%,%,$(wildcard cmd/*))

.PHONY: lint test test-race test-race-stress bench bench-ingestlease-integration test-archive-integration test-pipeline-integration test-proto-contract test-db-migrations-ci test-grpc-load-harness test-grpc-soak-10k test-web-e2e test-mobile-e2e test-load-k6 build smoke ecoflow-ble-discover pulse-edge-collector pulse-edge-collector-linux-arm64 pulse-edge-pi5-bundle pecron-smoke ingest-worker inference-worker rollup-worker projection-worker archive-worker replay-cli gap-detector gap-repair-worker docker-local-ready k3d-local-ready helm-local-ready chart-deps-local services-image-build-local services-image-import-local services-image-local-up services-image-build-cloud services-image-push-cloud platform-app-image-build-local platform-app-image-build-cloud platform-app-image-push-cloud realtime-gateway-image-build-local realtime-gateway-image-build-cloud realtime-gateway-image-push-cloud public-images-build-local public-images-build-cloud public-images-push-cloud public-images-import-local public-images-local-up public-deployments-restart-local k3d-up platform-up platform-wait platform-recover-local dev-grafana edge-verify-http3-local local-trust-platform-tls local-trust-platform-tls-system local-cloud-db-env local-cloud-realtime-env services-up services-up-cloud-db services-wait dev-up dev-up-cloud-db local-up local-up-cloud-db local-deploy local-deploy-cloud-db local-down local-status dev-web-deploy dev-web-deploy-cloud-realtime dev-deploy dev-deploy-cloud-db dev-archive-audit dev-archive-reconcile dev-regen-data dev-down db-migrate-up-local db-migrate-down-local db-migrate-verify-local db-migrate-cycle-local db-migrate-e2e-local db-seed-dev-local pgroll-init-local pgroll-status-local pgroll-start-local pgroll-complete-local pgroll-rollback-local dr-backup-local dr-restore-local dr-drill-local auth-keycloak-verify-local gke-context gke-cloud-context cloud-context gke-dev-guardrails gke-park gke-wake scale-down scale-up argocd-bootstrap-dev argocd-apps-dev argocd-wait-apps argocd-dev-up argocd-bootstrap-cloud argocd-apps-cloud argocd-wait-apps-cloud argocd-cloud-up cloud-up cloud-refresh cloud-health-gate cloud-platform-apply cloud-services-apply cloud-deploy cloud-cost-min-deploy cloud-forward-image-build cloud-db-forward cloud-db-forward-start cloud-db-forward-stop cloud-db-forward-status cloud-db-env cloud-realtime-forward cloud-realtime-forward-start cloud-realtime-forward-stop cloud-realtime-forward-status cloud-status web web-stop clean

lint:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@git ls-files -z '*.go' | xargs -0 gofmt -w
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "running golangci-lint"; \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found; running go vet"; \
		$(GO) vet ./...; \
	fi
	@if ! command -v markdownlint >/dev/null 2>&1; then \
		echo "markdownlint not found; install with: brew install markdownlint-cli"; \
		exit 1; \
	fi
	@if ! command -v buf >/dev/null 2>&1; then \
		echo "buf not found; install from https://buf.build/docs/installation/"; \
		exit 1; \
	fi
	@if ! command -v actionlint >/dev/null 2>&1; then \
		echo "actionlint not found; install with: brew install actionlint"; \
		exit 1; \
	fi
	@echo "running buf lint"
	@buf lint
	@echo "running actionlint"
	@actionlint
	@echo "running markdownlint"
	@git ls-files -z '*.md' | xargs -0 markdownlint --config .markdownlint.json

test:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) test ./...

test-race:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) test -race $(RACE_CRITICAL_PKGS)

test-race-stress:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) test -race $(RACE_CRITICAL_PKGS) -count=$(RACE_STRESS_COUNT)

bench:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) test ./... -run '^$$' -bench . -benchmem -count=1

bench-ingestlease-integration:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	log_file=/tmp/pulse-valkey-bench-portforward.log; \
	echo "starting valkey port-forward on $(VALKEY_BENCH_ADDRS) (log: $$log_file)"; \
	$(KUBECTL) -n $(VALKEY_BENCH_NAMESPACE) port-forward svc/$(VALKEY_BENCH_SERVICE) $(VALKEY_BENCH_LOCAL_PORT):6379 >$$log_file 2>&1 & \
	pf_pid=$$!; \
	cleanup() { kill $$pf_pid >/dev/null 2>&1 || true; }; \
	trap cleanup EXIT INT TERM; \
	sleep 2; \
	VALKEY_INTEGRATION_ADDRS="$(VALKEY_BENCH_ADDRS)" $(GO) test ./internal/ingestlease -tags integration -run '^TestRunHeartbeatNoGoroutineLeakIntegration$$' -count=1; \
	VALKEY_INTEGRATION_ADDRS="$(VALKEY_BENCH_ADDRS)" $(GO) test ./internal/ingestlease -tags integration -run '^$$' -bench 'BenchmarkLeaseManager.*Integration' -benchmem -count=1

test-archive-integration:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	log_file=/tmp/pulse-minio-integration-portforward.log; \
	echo "starting minio port-forward on $(ARCHIVE_INTEGRATION_ENDPOINT) (log: $$log_file)"; \
	$(KUBECTL) -n $(ARCHIVE_INTEGRATION_NAMESPACE) port-forward svc/$(ARCHIVE_INTEGRATION_SERVICE) $(ARCHIVE_INTEGRATION_LOCAL_PORT):9000 >$$log_file 2>&1 & \
	pf_pid=$$!; \
	cleanup() { kill $$pf_pid >/dev/null 2>&1 || true; }; \
	trap cleanup EXIT INT TERM; \
	sleep 2; \
	access_key="$${ARCHIVE_OBJECT_ACCESS_KEY:-}"; \
	secret_key="$${ARCHIVE_OBJECT_SECRET_KEY:-}"; \
	if [ -z "$$access_key" ] || [ -z "$$secret_key" ]; then \
		access_key="$$( $(KUBECTL) -n $(ARCHIVE_INTEGRATION_NAMESPACE) get secret $(ARCHIVE_INTEGRATION_SECRET) -o jsonpath='{.data.rootUser}' | base64 -d )"; \
		secret_key="$$( $(KUBECTL) -n $(ARCHIVE_INTEGRATION_NAMESPACE) get secret $(ARCHIVE_INTEGRATION_SECRET) -o jsonpath='{.data.rootPassword}' | base64 -d )"; \
	fi; \
	ARCHIVE_STORE_INTEGRATION=1 \
	ARCHIVE_OBJECT_ENDPOINT="$(ARCHIVE_INTEGRATION_ENDPOINT)" \
	ARCHIVE_OBJECT_ACCESS_KEY="$$access_key" \
	ARCHIVE_OBJECT_SECRET_KEY="$$secret_key" \
	$(GO) test ./internal/archiveworker -tags integration -run 'TestMinIOObjectStore.*Integration' -count=1 -v

test-pipeline-integration:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@echo "running end-to-end telemetry pipeline integration suite with Testcontainers"
	PIPELINE_INTEGRATION=1 $(GO) test ./internal/pipelineintegration -tags integration -count=1 -v

test-proto-contract:
	@echo "running Node↔Go protobuf contract tests"
	$(NPM) run test --workspace @ecoflow-pulse/pulse-realtime-gateway -- --run test/proto_contract.test.ts

test-db-migrations-ci:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@echo "running migration cycle + pgroll + e2e validation suite with Testcontainers"
	PGROLL_REQUIRED="$(PGROLL_REQUIRED)" PGROLL_BIN="$(PGROLL)" \
	$(GO) test ./internal/integrationtest -run 'Test(MigrationsCycleAndE2E|PgrollPlansCycleAndRollback|RestoreDeviceImportUpsertConstraintsRepairsDriftedImportSchema|RestoreControlPlaneKeysAllowsEdgeCollectorMigrationOnDriftedSchema)' -count=1 -v

test-grpc-load-harness:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@echo "running grpc unary + streaming load harness"
	$(GO) test ./cmd/ecoflow-grpc-api -run '^$$' -bench 'BenchmarkTelemetry(GetSnapshotObservedFleetMix|SubscribeObservedBurst)$$' -benchmem -benchtime=$(GRPC_LOAD_BENCH_TIME)

test-grpc-soak-10k:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@echo "running opt-in grpc 10k synthetic soak harness"
	ECOFLOW_GRPC_10K_SOAK=1 $(GO) test ./cmd/ecoflow-grpc-api -run '^$$' -bench 'BenchmarkTelemetry(GetSnapshotObservedFleetMix10k|SubscribeObservedStartupSpike10k)$$' -benchmem -benchtime=$(GRPC_LOAD_10K_BENCH_TIME)

test-web-e2e:
	@echo "running Playwright web E2E suite"
	$(NPM) run -w apps/universal e2e:web

test-mobile-e2e:
	@echo "running Maestro mobile E2E smoke flow"
	$(NPM) run -w apps/universal e2e:mobile

test-load-k6:
	@if ! command -v $(K6) >/dev/null 2>&1; then \
		echo "$(K6) not found. Install k6 first: https://grafana.com/docs/k6/latest/set-up/install-k6/"; \
		exit 1; \
	fi
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@if ! command -v curl >/dev/null 2>&1; then \
		echo "curl not found. Install curl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	pf_log=/tmp/pulse-k6-nats-portforward.log; \
	bridge_log=/tmp/pulse-k6-ingest-bridge.log; \
	echo "starting nats port-forward on 127.0.0.1:$(K6_NATS_LOCAL_PORT) (log: $$pf_log)"; \
	$(KUBECTL) -n $(K6_NATS_NAMESPACE) port-forward svc/$(K6_NATS_SERVICE) $(K6_NATS_LOCAL_PORT):4222 >$$pf_log 2>&1 & \
	pf_pid=$$!; \
	bridge_pid=; \
	cleanup() { \
		if [ -n "$$bridge_pid" ]; then \
			kill $$bridge_pid >/dev/null 2>&1 || true; \
		fi; \
		kill $$pf_pid >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	sleep 2; \
	echo "starting loadtest ingest bridge on $(K6_INGEST_BRIDGE_ADDR) (log: $$bridge_log)"; \
	NATS_URLS="nats://127.0.0.1:$(K6_NATS_LOCAL_PORT)" \
	TELEMETRY_SUBJECT_PREFIX="$(K6_TELEMETRY_SUBJECT_PREFIX)" \
	LOADTEST_INGEST_BIND_ADDR="$(K6_INGEST_BRIDGE_ADDR)" \
	go run ./cmd/ecoflow-loadtest-ingest-bridge >$$bridge_log 2>&1 & \
	bridge_pid=$$!; \
	for attempt in $$(seq 1 20); do \
		if curl -fsS "http://$(K6_INGEST_BRIDGE_ADDR)/healthz" >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 0.5; \
		if [ $$attempt -eq 20 ]; then \
			echo "ingest bridge did not become ready; see $$bridge_log"; \
			exit 1; \
		fi; \
	done; \
	LOADTEST_API_BASE_URL="$(K6_API_BASE_URL)" \
	LOADTEST_WS_URL="$(K6_WS_URL)" \
	LOADTEST_INGEST_URL="http://$(K6_INGEST_BRIDGE_ADDR)/ingest" \
	LOADTEST_USER_SUBJECT="$(K6_USER_SUBJECT)" \
	LOADTEST_DURATION="$(K6_DURATION)" \
	LOADTEST_INGEST_RATE="$(K6_INGEST_RATE)" \
	LOADTEST_INGEST_PRE_ALLOCATED_VUS="$(K6_INGEST_PRE_ALLOCATED_VUS)" \
	LOADTEST_INGEST_MAX_VUS="$(K6_INGEST_MAX_VUS)" \
	LOADTEST_QUERY_RATE="$(K6_QUERY_RATE)" \
	LOADTEST_QUERY_PRE_ALLOCATED_VUS="$(K6_QUERY_PRE_ALLOCATED_VUS)" \
	LOADTEST_QUERY_MAX_VUS="$(K6_QUERY_MAX_VUS)" \
	LOADTEST_WS_VUS="$(K6_WS_VUS)" \
	LOADTEST_WS_SESSION_TIMEOUT_MS="$(K6_WS_SESSION_TIMEOUT_MS)" \
	LOADTEST_WS_POST_TELEMETRY_HOLD_MS="$(K6_WS_POST_TELEMETRY_HOLD_MS)" \
	LOADTEST_WS_THINK_TIME_MS="$(K6_WS_THINK_TIME_MS)" \
	LOADTEST_SEED_COUNT="$(K6_SEED_COUNT)" \
	LOADTEST_QUERY_WINDOW_MS="$(K6_QUERY_WINDOW_MS)" \
	LOADTEST_QUERY_RESOLUTION="$(K6_QUERY_RESOLUTION)" \
	LOADTEST_INGEST_P95_MS="$(K6_INGEST_P95_MS)" \
	LOADTEST_QUERY_P95_MS="$(K6_QUERY_P95_MS)" \
	LOADTEST_WS_SUCCESS_RATE="$(K6_WS_SUCCESS_RATE)" \
	LOADTEST_DEVICE_ID="$(K6_DEVICE_ID)" \
	LOADTEST_DEVICE_SERIAL_NUMBER="$(K6_DEVICE_SERIAL_NUMBER)" \
	$(K6) run $(K6_SCRIPT)

build:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" bin
	@for cmd in $(CMDS); do \
		echo "building $$cmd"; \
		$(GO) build -ldflags "$(LDFLAGS)" -o "bin/$$cmd" "./cmd/$$cmd"; \
	done

docker-local-ready:
	@if ! command -v $(DOCKER) >/dev/null 2>&1; then \
		echo "$(DOCKER) not found. Install Docker Desktop first."; \
		exit 1; \
	fi
	@if ! $(DOCKER) info >/dev/null 2>&1; then \
		echo "Docker daemon is not running. Start Docker Desktop and retry."; \
		exit 1; \
	fi
	@mkdir -p "$(DOCKER_CONFIG_LOCAL)"
	@mkdir -p "$(DOCKER_BUILDX_CONFIG_LOCAL)"
	@mkdir -p "$(DOCKER_CONFIG_LOCAL)/cli-plugins"
	@if [ ! -f "$(DOCKER_CONFIG_LOCAL)/config.json" ]; then \
		printf '{\n  "auths": {}\n}\n' > "$(DOCKER_CONFIG_LOCAL)/config.json"; \
	fi
	@set -euo pipefail; \
		plugin_dst="$(DOCKER_CONFIG_LOCAL)/cli-plugins/docker-buildx"; \
		plugin_src=""; \
		if [ -x "$$HOME/.docker/cli-plugins/docker-buildx" ]; then \
			plugin_src="$$HOME/.docker/cli-plugins/docker-buildx"; \
		elif [ -x "/Applications/Docker.app/Contents/Resources/cli-plugins/docker-buildx" ]; then \
			plugin_src="/Applications/Docker.app/Contents/Resources/cli-plugins/docker-buildx"; \
		fi; \
		if [ -n "$$plugin_src" ]; then \
			ln -sf "$$plugin_src" "$$plugin_dst"; \
		fi

k3d-local-ready:
	@if ! command -v $(K3D) >/dev/null 2>&1; then \
		echo "$(K3D) not found. Install k3d first."; \
		exit 1; \
	fi

helm-local-ready:
	@if ! command -v $(HELM) >/dev/null 2>&1; then \
		echo "$(HELM) not found. Install helm first."; \
		exit 1; \
	fi

chart-deps-local: helm-local-ready
	@if [ -z "$(CHART)" ]; then \
		echo "CHART is required"; \
		exit 1; \
	fi
	@set -euo pipefail; \
		chart="$(CHART)"; \
		lock="$$chart/Chart.lock"; \
		charts_dir="$$chart/charts"; \
		if [ ! -f "$$lock" ]; then \
			if grep -Eq '^[[:space:]]*repository:' "$$chart/Chart.yaml"; then \
				echo "Chart.lock missing for $$chart; running helm dependency build --skip-refresh"; \
				$(HELM) dependency build --skip-refresh "$$chart"; \
			else \
				echo "chart $$chart has no external dependencies; skipping helm dependency build"; \
			fi; \
			exit 0; \
		fi; \
		dep_count="$$(grep -Ec '^- name:' "$$lock" || true)"; \
		if [ "$$dep_count" -eq 0 ]; then \
			echo "chart $$chart has no external dependencies; skipping helm dependency build"; \
			exit 0; \
		fi; \
		if [ -n "$$(git status --porcelain --untracked-files=all -- "$$chart/Chart.yaml" "$$lock")" ]; then \
			echo "chart dependency metadata changed for $$chart; running helm dependency build --skip-refresh"; \
			$(HELM) dependency build --skip-refresh "$$chart"; \
			exit 0; \
		fi; \
		if [ -d "$$charts_dir" ]; then \
			vendored_count="$$(find "$$charts_dir" -mindepth 1 -maxdepth 1 -name '*.tgz' | wc -l | tr -d '[:space:]')"; \
		else \
			vendored_count=0; \
		fi; \
		if [ "$$vendored_count" != "$$dep_count" ]; then \
			echo "vendored chart packages missing for $$chart; running helm dependency build --skip-refresh"; \
			$(HELM) dependency build --skip-refresh "$$chart"; \
			exit 0; \
		fi; \
		echo "chart dependencies already vendored for $$chart; skipping helm dependency build"

.PHONY: appliance-pi-shellcheck appliance-pi-test appliance-pi-helm-lint appliance-pi-validate appliance-pi-install appliance-pi-upgrade appliance-pi-wait appliance-pi-status

appliance-pi-shellcheck:
	@if ! command -v shellcheck >/dev/null 2>&1; then \
		echo "shellcheck not found. Install shellcheck first."; \
		exit 1; \
	fi
	shellcheck deploy/appliance/pi5/*.sh

appliance-pi-test:
	bash deploy/appliance/pi5/test-host-prepare.sh
	bash deploy/appliance/pi5/test-install-dry-run.sh
	bash deploy/appliance/pi5/test-status-outbox.sh

appliance-pi-helm-lint: helm-local-ready
	$(HELM) lint $(PLATFORM_CHART) -f $(PI_PLATFORM_VALUES)
	$(HELM) lint $(SERVICES_CHART) -f $(PI_SERVICES_VALUES)
	HELM="$(HELM)" bash deploy/appliance/pi5/test-go-runtime-render.sh

appliance-pi-validate: appliance-pi-shellcheck appliance-pi-test appliance-pi-helm-lint

appliance-pi-install:
	bash deploy/appliance/pi5/pulse-appliance-install.sh install $(APPLIANCE_PI_INSTALL_ARGS)

appliance-pi-upgrade:
	bash deploy/appliance/pi5/pulse-appliance-install.sh upgrade $(APPLIANCE_PI_INSTALL_ARGS)

appliance-pi-wait:
	bash deploy/appliance/pi5/pulse-appliance-install.sh wait $(APPLIANCE_PI_INSTALL_ARGS)

appliance-pi-status:
	bash deploy/appliance/pi5/pulse-appliance-install.sh status $(APPLIANCE_PI_INSTALL_ARGS)

services-image-build-local: docker-local-ready
	@echo "building services image $(SERVICES_IMAGE) for $(LOCAL_IMAGE_PLATFORM) from $(SERVICES_IMAGE_DOCKERFILE)"
	@if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" BUILDX_CONFIG="$(DOCKER_BUILDX_CONFIG_LOCAL)" DOCKER_BUILDKIT=1 $(DOCKER) build --platform $(LOCAL_IMAGE_PLATFORM) -f $(SERVICES_IMAGE_DOCKERFILE) -t $(SERVICES_IMAGE) .; \
	else \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build --platform $(LOCAL_IMAGE_PLATFORM) -f $(SERVICES_IMAGE_DOCKERFILE) -t $(SERVICES_IMAGE) .; \
	fi

services-image-import-local: k3d-local-ready
	@echo "importing services image $(SERVICES_IMAGE) into k3d cluster $(K3D_CLUSTER_NAME)"
	$(K3D) image import $(SERVICES_IMAGE) -c $(K3D_CLUSTER_NAME)

services-image-local-up:
	@$(MAKE) --no-print-directory services-image-build-local
	@$(MAKE) --no-print-directory services-image-import-local

services-image-build-cloud: docker-local-ready
	@echo "building cloud services image $(SERVICES_CLOUD_IMAGE) for $(CLOUD_IMAGE_PLATFORM) from $(SERVICES_IMAGE_DOCKERFILE)"
	@if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" BUILDX_CONFIG="$(DOCKER_BUILDX_CONFIG_LOCAL)" DOCKER_BUILDKIT=1 $(DOCKER) build --platform $(CLOUD_IMAGE_PLATFORM) -f $(SERVICES_IMAGE_DOCKERFILE) -t $(SERVICES_CLOUD_IMAGE) .; \
	else \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build --platform $(CLOUD_IMAGE_PLATFORM) -f $(SERVICES_IMAGE_DOCKERFILE) -t $(SERVICES_CLOUD_IMAGE) .; \
	fi

services-image-push-cloud: services-image-build-cloud
	@echo "pushing cloud services image $(SERVICES_CLOUD_IMAGE)"
	@set -euo pipefail; \
		token="$$(CLOUDSDK_CONFIG="$${CLOUDSDK_CONFIG:-}" $(GCLOUD) auth print-access-token)"; \
		printf '%s' "$$token" | DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) login -u oauth2accesstoken --password-stdin https://$(CLOUD_ARTIFACT_REGISTRY_HOST) >/dev/null; \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) push $(SERVICES_CLOUD_IMAGE)

platform-app-image-build-local: docker-local-ready
	@echo "building public app image $(PLATFORM_APP_IMAGE) for $(LOCAL_IMAGE_PLATFORM) from $(PLATFORM_APP_IMAGE_DOCKERFILE)"
	@set -euo pipefail; \
		if [ -f .env ]; then \
			set -a; source ./.env; set +a; \
		fi; \
		EXPO_PUBLIC_LOCAL_DATA_PLANE="$${EXPO_PUBLIC_LOCAL_DATA_PLANE:-local}"; \
		if [ "$${EXPO_PUBLIC_LOCAL_DATA_PLANE:-}" = "cloud" ]; then \
			unset EXPO_PUBLIC_CLOUD_API_URL EXPO_PUBLIC_CLOUD_WS_URL EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE EXPO_PUBLIC_CLOUD_OIDC_SCOPES; \
		fi; \
		set --; \
		for var in $(PLATFORM_APP_BUILD_ARG_VARS); do \
			case "$$var" in \
				EXPO_PUBLIC_API_URL) val="$${EXPO_PUBLIC_API_URL:-}" ;; \
				EXPO_PUBLIC_WS_URL) val="$${EXPO_PUBLIC_WS_URL:-}" ;; \
				EXPO_PUBLIC_OIDC_ISSUER_URL) val="$${EXPO_PUBLIC_OIDC_ISSUER_URL:-}" ;; \
				EXPO_PUBLIC_OIDC_CLIENT_ID) val="$${EXPO_PUBLIC_OIDC_CLIENT_ID:-}" ;; \
				EXPO_PUBLIC_OIDC_AUDIENCE) val="$${EXPO_PUBLIC_OIDC_AUDIENCE:-}" ;; \
				EXPO_PUBLIC_OIDC_SCOPES) val="$${EXPO_PUBLIC_OIDC_SCOPES:-}" ;; \
				EXPO_PUBLIC_CLOUD_API_URL) val="$${EXPO_PUBLIC_CLOUD_API_URL:-}" ;; \
				EXPO_PUBLIC_CLOUD_WS_URL) val="$${EXPO_PUBLIC_CLOUD_WS_URL:-}" ;; \
				EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL) val="$${EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL:-}" ;; \
				EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID) val="$${EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID:-}" ;; \
				EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE) val="$${EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE:-}" ;; \
				EXPO_PUBLIC_CLOUD_OIDC_SCOPES) val="$${EXPO_PUBLIC_CLOUD_OIDC_SCOPES:-}" ;; \
				EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE) val="$${EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE:-}" ;; \
				EXPO_PUBLIC_LOCAL_DATA_PLANE) val="$${EXPO_PUBLIC_LOCAL_DATA_PLANE:-}" ;; \
				*) val="" ;; \
			esac; \
			if [ -n "$$val" ]; then \
				set -- "$$@" --build-arg "$$var=$$val"; \
			fi; \
			done; \
			if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
				DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" BUILDX_CONFIG="$(DOCKER_BUILDX_CONFIG_LOCAL)" DOCKER_BUILDKIT=1 $(DOCKER) build --platform $(LOCAL_IMAGE_PLATFORM) "$$@" -f $(PLATFORM_APP_IMAGE_DOCKERFILE) -t $(PLATFORM_APP_IMAGE) .; \
			else \
				DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build --platform $(LOCAL_IMAGE_PLATFORM) "$$@" -f $(PLATFORM_APP_IMAGE_DOCKERFILE) -t $(PLATFORM_APP_IMAGE) .; \
			fi

platform-app-image-build-cloud: docker-local-ready
	@echo "building cloud public app image $(PLATFORM_APP_CLOUD_IMAGE) for $(CLOUD_IMAGE_PLATFORM) from $(PLATFORM_APP_IMAGE_DOCKERFILE)"
	@set -euo pipefail; \
		set --; \
		for var in $(PLATFORM_APP_BUILD_ARG_VARS); do \
			case "$$var" in \
				EXPO_PUBLIC_API_URL) val="$${EXPO_PUBLIC_API_URL:-}" ;; \
				EXPO_PUBLIC_WS_URL) val="$${EXPO_PUBLIC_WS_URL:-}" ;; \
				EXPO_PUBLIC_OIDC_ISSUER_URL) val="$${EXPO_PUBLIC_OIDC_ISSUER_URL:-}" ;; \
				EXPO_PUBLIC_OIDC_CLIENT_ID) val="$${EXPO_PUBLIC_OIDC_CLIENT_ID:-}" ;; \
				EXPO_PUBLIC_OIDC_AUDIENCE) val="$${EXPO_PUBLIC_OIDC_AUDIENCE:-}" ;; \
				EXPO_PUBLIC_OIDC_SCOPES) val="$${EXPO_PUBLIC_OIDC_SCOPES:-}" ;; \
				EXPO_PUBLIC_CLOUD_API_URL) val="$${EXPO_PUBLIC_CLOUD_API_URL:-}" ;; \
				EXPO_PUBLIC_CLOUD_WS_URL) val="$${EXPO_PUBLIC_CLOUD_WS_URL:-}" ;; \
				EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL) val="$${EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL:-}" ;; \
				EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID) val="$${EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID:-}" ;; \
				EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE) val="$${EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE:-}" ;; \
				EXPO_PUBLIC_CLOUD_OIDC_SCOPES) val="$${EXPO_PUBLIC_CLOUD_OIDC_SCOPES:-}" ;; \
				EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE) val="$${EXPO_PUBLIC_DEFAULT_CONNECTION_PROFILE:-}" ;; \
				EXPO_PUBLIC_LOCAL_DATA_PLANE) val="$${EXPO_PUBLIC_LOCAL_DATA_PLANE:-}" ;; \
				*) val="" ;; \
			esac; \
			if [ -n "$$val" ]; then \
				set -- "$$@" --build-arg "$$var=$$val"; \
			fi; \
		done; \
		if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
			DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" BUILDX_CONFIG="$(DOCKER_BUILDX_CONFIG_LOCAL)" DOCKER_BUILDKIT=1 $(DOCKER) build --platform $(CLOUD_IMAGE_PLATFORM) "$$@" -f $(PLATFORM_APP_IMAGE_DOCKERFILE) -t $(PLATFORM_APP_CLOUD_IMAGE) .; \
		else \
			DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build --platform $(CLOUD_IMAGE_PLATFORM) "$$@" -f $(PLATFORM_APP_IMAGE_DOCKERFILE) -t $(PLATFORM_APP_CLOUD_IMAGE) .; \
		fi

platform-app-image-push-cloud: platform-app-image-build-cloud
	@echo "pushing cloud public app image $(PLATFORM_APP_CLOUD_IMAGE)"
	@set -euo pipefail; \
		token="$$(CLOUDSDK_CONFIG="$${CLOUDSDK_CONFIG:-}" $(GCLOUD) auth print-access-token)"; \
		printf '%s' "$$token" | DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) login -u oauth2accesstoken --password-stdin https://$(CLOUD_ARTIFACT_REGISTRY_HOST) >/dev/null; \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) push $(PLATFORM_APP_CLOUD_IMAGE)

realtime-gateway-image-build-local: docker-local-ready
	@echo "building realtime gateway image $(REALTIME_GATEWAY_IMAGE) for $(LOCAL_IMAGE_PLATFORM) from $(REALTIME_GATEWAY_IMAGE_DOCKERFILE)"
	@if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" BUILDX_CONFIG="$(DOCKER_BUILDX_CONFIG_LOCAL)" DOCKER_BUILDKIT=1 $(DOCKER) build --platform $(LOCAL_IMAGE_PLATFORM) -f $(REALTIME_GATEWAY_IMAGE_DOCKERFILE) -t $(REALTIME_GATEWAY_IMAGE) .; \
	else \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build --platform $(LOCAL_IMAGE_PLATFORM) -f $(REALTIME_GATEWAY_IMAGE_DOCKERFILE) -t $(REALTIME_GATEWAY_IMAGE) .; \
	fi

realtime-gateway-image-build-cloud: docker-local-ready
	@echo "building cloud realtime gateway image $(REALTIME_GATEWAY_CLOUD_IMAGE) for $(CLOUD_IMAGE_PLATFORM) from $(REALTIME_GATEWAY_IMAGE_DOCKERFILE)"
	@if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" BUILDX_CONFIG="$(DOCKER_BUILDX_CONFIG_LOCAL)" DOCKER_BUILDKIT=1 $(DOCKER) build --platform $(CLOUD_IMAGE_PLATFORM) -f $(REALTIME_GATEWAY_IMAGE_DOCKERFILE) -t $(REALTIME_GATEWAY_CLOUD_IMAGE) .; \
	else \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build --platform $(CLOUD_IMAGE_PLATFORM) -f $(REALTIME_GATEWAY_IMAGE_DOCKERFILE) -t $(REALTIME_GATEWAY_CLOUD_IMAGE) .; \
	fi

realtime-gateway-image-push-cloud: realtime-gateway-image-build-cloud
	@echo "pushing cloud realtime gateway image $(REALTIME_GATEWAY_CLOUD_IMAGE)"
	@set -euo pipefail; \
		token="$$(CLOUDSDK_CONFIG="$${CLOUDSDK_CONFIG:-}" $(GCLOUD) auth print-access-token)"; \
		printf '%s' "$$token" | DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) login -u oauth2accesstoken --password-stdin https://$(CLOUD_ARTIFACT_REGISTRY_HOST) >/dev/null; \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) push $(REALTIME_GATEWAY_CLOUD_IMAGE)

public-images-build-local:
	@$(MAKE) --no-print-directory -j2 platform-app-image-build-local realtime-gateway-image-build-local

public-images-build-cloud:
	@$(MAKE) --no-print-directory -j2 platform-app-image-build-cloud realtime-gateway-image-build-cloud

public-images-push-cloud:
	@$(MAKE) --no-print-directory -j2 platform-app-image-push-cloud realtime-gateway-image-push-cloud

public-images-import-local: k3d-local-ready
	@echo "importing public app and realtime gateway images into k3d cluster $(K3D_CLUSTER_NAME)"
	$(K3D) image import $(PLATFORM_APP_IMAGE) $(REALTIME_GATEWAY_IMAGE) -c $(K3D_CLUSTER_NAME)

public-images-local-up:
	@$(MAKE) --no-print-directory public-images-build-local
	@$(MAKE) --no-print-directory public-images-import-local

smoke:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-smoke

ecoflow-ble-discover:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" bin
	@set -euo pipefail; \
		if [ "$$(uname -s)" = "Darwin" ]; then \
			echo "building $(ECOFLOW_BLE_DISCOVER_BIN) with macOS Bluetooth usage metadata"; \
			$(GO) build -ldflags "-linkmode=external -extldflags=-Wl,-sectcreate,__TEXT,__info_plist,$(ECOFLOW_BLE_DISCOVER_PLIST)" -o "$(ECOFLOW_BLE_DISCOVER_BIN)" ./cmd/ecoflow-ble-discover; \
			$(CODESIGN) --force --sign - --identifier com.ecoflow-pulse.ecoflow-ble-discover "$(ECOFLOW_BLE_DISCOVER_BIN)" >/dev/null; \
		else \
			echo "building $(ECOFLOW_BLE_DISCOVER_BIN)"; \
			$(GO) build -o "$(ECOFLOW_BLE_DISCOVER_BIN)" ./cmd/ecoflow-ble-discover; \
		fi; \
		if [ "$(ECOFLOW_BLE_DISCOVER_RUN)" = "0" ]; then \
			exit 0; \
		fi; \
		"$(ECOFLOW_BLE_DISCOVER_BIN)" $(ECOFLOW_BLE_DISCOVER_ARGS)

pulse-edge-collector:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" bin
	@echo "building $(PULSE_EDGE_COLLECTOR_BIN)"
	$(GO) build -o "$(PULSE_EDGE_COLLECTOR_BIN)" ./cmd/pulse-edge-collector

pulse-edge-collector-linux-arm64:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" "$(PULSE_EDGE_PI5_BIN_DIR)"
	@echo "building Raspberry Pi 5 linux/arm64 edge collector bundle binaries"
	CGO_ENABLED=$(PULSE_EDGE_PI5_CGO_ENABLED) GOOS=linux GOARCH=arm64 GOARM64=$(PULSE_EDGE_PI5_GOARM64) $(GO) build -trimpath -ldflags "$(PULSE_EDGE_PI5_LDFLAGS)" -o "$(PULSE_EDGE_PI5_BIN_DIR)/pulse-edge-collector" ./cmd/pulse-edge-collector
	CGO_ENABLED=$(PULSE_EDGE_PI5_CGO_ENABLED) GOOS=linux GOARCH=arm64 GOARM64=$(PULSE_EDGE_PI5_GOARM64) $(GO) build -trimpath -ldflags "$(PULSE_EDGE_PI5_LDFLAGS)" -o "$(PULSE_EDGE_PI5_BIN_DIR)/ecoflow-ble-discover" ./cmd/ecoflow-ble-discover

pulse-edge-pi5-bundle: pulse-edge-collector-linux-arm64
	@rm -rf "$(PULSE_EDGE_PI5_BUNDLE_DIR)" "$(PULSE_EDGE_PI5_BUNDLE)"
	@mkdir -p "$(PULSE_EDGE_PI5_BUNDLE_DIR)/bin" "$(PULSE_EDGE_PI5_BUNDLE_DIR)/config" "$(PULSE_EDGE_PI5_BUNDLE_DIR)/systemd" "$(PULSE_EDGE_PI5_BUNDLE_DIR)/docs"
	@cp "$(PULSE_EDGE_PI5_BIN_DIR)/pulse-edge-collector" "$(PULSE_EDGE_PI5_BUNDLE_DIR)/bin/pulse-edge-collector"
	@cp "$(PULSE_EDGE_PI5_BIN_DIR)/ecoflow-ble-discover" "$(PULSE_EDGE_PI5_BUNDLE_DIR)/bin/ecoflow-ble-discover"
	@cp deploy/pulse-edge/config.pi5.yaml "$(PULSE_EDGE_PI5_BUNDLE_DIR)/config/config.yaml"
	@cp deploy/pulse-edge/pulse-edge-collector.service "$(PULSE_EDGE_PI5_BUNDLE_DIR)/systemd/pulse-edge-collector.service"
	@cp docs/how-to/run-pulse-edge-collector.md "$(PULSE_EDGE_PI5_BUNDLE_DIR)/docs/run-pulse-edge-collector.md"
	@tar -C "$(dir $(PULSE_EDGE_PI5_BUNDLE_DIR))" -czf "$(PULSE_EDGE_PI5_BUNDLE)" "$(notdir $(PULSE_EDGE_PI5_BUNDLE_DIR))"
	@echo "wrote $(PULSE_EDGE_PI5_BUNDLE)"

pecron-smoke:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@set -euo pipefail; \
		incoming_email="$${PECRON_EMAIL:-}"; \
		incoming_password="$${PECRON_PASSWORD:-}"; \
		incoming_region="$${PECRON_REGION:-}"; \
		incoming_config="$${PECRON_CONFIG:-}"; \
		incoming_credential_id="$${PECRON_CREDENTIAL_ID:-}"; \
		incoming_target_suffix="$${PECRON_TARGET_SUFFIX:-}"; \
		incoming_db_dsn="$${CONTROL_PLANE_DB_DSN:-}"; \
		if [ -f "$(CLOUD_DB_ENV_FILE)" ]; then \
			set -a; source "$(CLOUD_DB_ENV_FILE)"; set +a; \
		fi; \
		if [ -f .env ]; then \
			set -a; source ./.env; set +a; \
		fi; \
		if [ -n "$$incoming_email" ]; then export PECRON_EMAIL="$$incoming_email"; fi; \
		if [ -n "$$incoming_password" ]; then export PECRON_PASSWORD="$$incoming_password"; fi; \
		if [ -n "$$incoming_region" ]; then export PECRON_REGION="$$incoming_region"; fi; \
		if [ -n "$$incoming_config" ]; then export PECRON_CONFIG="$$incoming_config"; fi; \
		if [ -n "$$incoming_credential_id" ]; then export PECRON_CREDENTIAL_ID="$$incoming_credential_id"; fi; \
		if [ -n "$$incoming_target_suffix" ]; then export PECRON_TARGET_SUFFIX="$$incoming_target_suffix"; fi; \
		if [ -n "$$incoming_db_dsn" ]; then export CONTROL_PLANE_DB_DSN="$$incoming_db_dsn"; fi; \
		$(GO) run ./cmd/pecron-smoke

ingest-worker:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-ingest-worker

inference-worker:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-inference-worker

rollup-worker:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-rollup-worker

projection-worker:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-projection-worker

archive-worker:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-archive-worker

replay-cli:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-replay-cli $(ARGS)

gap-detector:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-gap-detector

gap-repair-worker:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-gap-repair-worker

k3d-up:
	@if ! command -v $(K3D) >/dev/null 2>&1; then \
		echo "$(K3D) not found. Install k3d first."; \
		exit 1; \
	fi
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@if ! command -v $(DOCKER) >/dev/null 2>&1; then \
		echo "$(DOCKER) not found. Install Docker Desktop first."; \
		exit 1; \
	fi
	@if ! $(DOCKER) info >/dev/null 2>&1; then \
		echo "Docker daemon is not running. Start Docker Desktop and retry."; \
		exit 1; \
	fi
	@if $(K3D) kubeconfig get $(K3D_CLUSTER_NAME) >/dev/null 2>&1; then \
		echo "k3d cluster '$(K3D_CLUSTER_NAME)' already exists"; \
		$(K3D) cluster start $(K3D_CLUSTER_NAME) >/dev/null 2>&1 || true; \
	else \
		echo "creating k3d cluster '$(K3D_CLUSTER_NAME)' from $(K3D_CONFIG)"; \
		$(K3D) cluster create --config $(K3D_CONFIG); \
	fi
	@if [ "$(K3D_SET_CURRENT_CONTEXT)" = "1" ]; then \
		$(KUBECTL) config use-context $(K3D_CONTEXT); \
	else \
		echo "skipping global kubectl context switch; using context-pinned local commands for $(K3D_CONTEXT)"; \
	fi
	$(LOCAL_KUBECTL) wait --for=condition=Ready node --all --timeout=$(WAIT_TIMEOUT)
	$(LOCAL_KUBECTL) get nodes

platform-up: helm-local-ready
	@$(MAKE) --no-print-directory chart-deps-local CHART=$(PLATFORM_CHART)
	@set -euo pipefail; \
		if [ -f .env ]; then \
			set -a; source ./.env; set +a; \
		fi; \
		ns="$(PLATFORM_NAMESPACE)"; \
		valkey_recreate_for_immutable_upgrade() { \
			sts_name="$(PLATFORM_RELEASE)-valkey-node"; \
			if ! $(LOCAL_KUBECTL) -n "$$ns" get statefulset "$$sts_name" >/dev/null 2>&1; then \
				return 1; \
			fi; \
			echo "detected immutable Valkey StatefulSet change; recreating $$sts_name to apply durable storage topology"; \
			$(LOCAL_KUBECTL) -n "$$ns" delete statefulset "$$sts_name" --wait=true; \
			$(LOCAL_KUBECTL) -n "$$ns" delete pod -l app.kubernetes.io/instance=$(PLATFORM_RELEASE),app.kubernetes.io/name=valkey,app.kubernetes.io/component=node --ignore-not-found=true --wait=true; \
		}; \
		run_platform_helm() { \
			set -- "$$@"; \
			helm_log="$$(mktemp)"; \
			if [ -n "$${PULSE_PLATFORM_DEV_SUBJECT:-}" ]; then \
				set -- "$$@" --set-string "runtime.publicApp.env.devUserSubject=$${PULSE_PLATFORM_DEV_SUBJECT}"; \
			fi; \
			if [ -n "$${KEYCLOAK_SOCIAL_GOOGLE_CLIENT_ID:-}" ] && [ -n "$${KEYCLOAK_SOCIAL_GOOGLE_CLIENT_SECRET:-}" ]; then \
				set -- "$$@" --set "keycloakRealm.google.enabled=true"; \
				set -- "$$@" --set-string "keycloakRealm.google.clientId=$${KEYCLOAK_SOCIAL_GOOGLE_CLIENT_ID}"; \
				set -- "$$@" --set-string "keycloakRealm.google.clientSecret=$${KEYCLOAK_SOCIAL_GOOGLE_CLIENT_SECRET}"; \
			fi; \
			if [ -n "$${PULSE_PLATFORM_DEV_SUBJECT:-}" ]; then \
				echo "using local noop subject override for pulse-platform public app"; \
				if $(LOCAL_HELM) upgrade --install $(PLATFORM_RELEASE) $(PLATFORM_CHART) \
					--namespace $(PLATFORM_NAMESPACE) --create-namespace \
					$(LOCAL_HELM_UPGRADE_FLAGS) \
					$(LOCAL_PLATFORM_HELM_VALUES_ARGS) \
					"$$@" 2>&1 | tee "$$helm_log"; then \
					rm -f "$$helm_log"; \
					return 0; \
				fi; \
			else \
				if $(PLATFORM_HELM_APPLY) "$$@" 2>&1 | tee "$$helm_log"; then \
					rm -f "$$helm_log"; \
					return 0; \
				fi; \
			fi; \
			if grep -q 'StatefulSet.apps ".*-valkey-node" is invalid: spec: Forbidden' "$$helm_log"; then \
				valkey_recreate_for_immutable_upgrade; \
				rm -f "$$helm_log"; \
				if [ -n "$${PULSE_PLATFORM_DEV_SUBJECT:-}" ]; then \
					echo "retrying pulse-platform helm apply after Valkey StatefulSet recreation"; \
					$(LOCAL_HELM) upgrade --install $(PLATFORM_RELEASE) $(PLATFORM_CHART) \
						--namespace $(PLATFORM_NAMESPACE) --create-namespace \
						$(LOCAL_HELM_UPGRADE_FLAGS) \
						$(LOCAL_PLATFORM_HELM_VALUES_ARGS) \
						"$$@"; \
				else \
					echo "retrying pulse-platform helm apply after Valkey StatefulSet recreation"; \
					$(PLATFORM_HELM_APPLY) "$$@"; \
				fi; \
				return 0; \
			fi; \
			rm -f "$$helm_log"; \
			return 1; \
		}; \
		wait_endpoints() { \
			name="$$1"; attempts="$$2"; label="$$3"; \
			if ! $(LOCAL_KUBECTL) -n "$$ns" get endpoints "$$name" >/dev/null 2>&1; then \
				echo "skipping endpoint wait for $$label ($$name not found yet)"; \
				return 0; \
			fi; \
			for _ in $$(seq 1 "$$attempts"); do \
				endpoint_ips="$$( $(LOCAL_KUBECTL) -n "$$ns" get endpoints "$$name" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true )"; \
				if [ -n "$$endpoint_ips" ]; then \
					echo "$$label endpoints ready: $$endpoint_ips"; \
					return 0; \
				fi; \
				sleep 5; \
			done; \
			echo "$$label endpoints did not become ready"; \
			exit 1; \
		}; \
		wait_secret() { \
			name="$$1"; attempts="$$2"; \
			for _ in $$(seq 1 "$$attempts"); do \
				if $(LOCAL_KUBECTL) -n "$$ns" get secret "$$name" >/dev/null 2>&1; then \
					echo "secret/$$name is ready"; \
					return 0; \
				fi; \
				sleep 5; \
			done; \
			echo "secret/$$name did not become ready"; \
			exit 1; \
		}; \
		wait_condition_obj() { \
			kind="$$1"; name="$$2"; condition="$$3"; timeout="$$4"; \
			if $(LOCAL_KUBECTL) -n "$$ns" get "$$kind" "$$name" >/dev/null 2>&1; then \
				echo "waiting for $$kind/$$name condition=$$condition"; \
				$(LOCAL_KUBECTL) -n "$$ns" wait --for=condition="$$condition" "$$kind/$$name" --timeout="$$timeout"; \
			fi; \
		}; \
		wait_crd_established() { \
			name="$$1"; timeout="$$2"; \
			if $(LOCAL_KUBECTL) get crd "$$name" >/dev/null 2>&1; then \
				echo "waiting for crd/$$name condition=Established"; \
				$(LOCAL_KUBECTL) wait --for=condition=Established "crd/$$name" --timeout="$$timeout"; \
			fi; \
		}; \
		ensure_prometheus_operator_crds() { \
			missing=0; \
			for crd in alertmanagerconfigs.monitoring.coreos.com alertmanagers.monitoring.coreos.com podmonitors.monitoring.coreos.com probes.monitoring.coreos.com prometheusagents.monitoring.coreos.com prometheuses.monitoring.coreos.com prometheusrules.monitoring.coreos.com scrapeconfigs.monitoring.coreos.com servicemonitors.monitoring.coreos.com thanosrulers.monitoring.coreos.com; do \
				if ! $(LOCAL_KUBECTL) get crd "$$crd" >/dev/null 2>&1; then \
					missing=1; \
				fi; \
			done; \
			if [ "$$missing" = "1" ]; then \
				echo "pre-installing Prometheus Operator CRDs before Helm apply"; \
				helm show crds $(PLATFORM_CHART)/charts/kube-prometheus-stack-82.2.0.tgz | $(LOCAL_KUBECTL) apply --server-side=true --force-conflicts -f -; \
			fi; \
			for crd in alertmanagerconfigs.monitoring.coreos.com alertmanagers.monitoring.coreos.com podmonitors.monitoring.coreos.com probes.monitoring.coreos.com prometheusagents.monitoring.coreos.com prometheuses.monitoring.coreos.com prometheusrules.monitoring.coreos.com scrapeconfigs.monitoring.coreos.com servicemonitors.monitoring.coreos.com thanosrulers.monitoring.coreos.com; do \
				wait_crd_established "$$crd" 180s; \
			done; \
		}; \
		keycloak_bootstrap_override="$$(mktemp /tmp/pulse-platform-keycloak-bootstrap.XXXXXX).yaml"; \
		cleanup() { rm -f "$$keycloak_bootstrap_override"; }; \
		trap cleanup EXIT INT TERM; \
		printf '%s\n' \
			'components:' \
			'  keycloak:' \
			'    enabled: false' \
			'keycloakRealm:' \
			'  enabled: false' > "$$keycloak_bootstrap_override"; \
		keycloak_first_pass_flags=""; \
		if ! $(LOCAL_KUBECTL) -n "$$ns" get statefulset $(PLATFORM_RELEASE)-keycloak >/dev/null 2>&1; then \
			echo "fresh Keycloak bootstrap detected; deferring Keycloak until CNPG and bootstrap prerequisites are ready"; \
			keycloak_first_pass_flags="-f $$keycloak_bootstrap_override"; \
		fi; \
		$(LOCAL_KUBECTL) create namespace $(PLATFORM_NAMESPACE) --dry-run=client -o yaml | $(LOCAL_KUBECTL) apply -f -; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-ingress-nginx-controller >/dev/null 2>&1; then \
			echo "waiting for existing ingress-nginx controller to become ready before Helm apply"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-ingress-nginx-controller --timeout=180s; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get svc $(PLATFORM_RELEASE)-ingress-nginx-controller-admission >/dev/null 2>&1; then \
			wait_endpoints $(PLATFORM_RELEASE)-ingress-nginx-controller-admission 36 "ingress-nginx admission"; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cert-manager >/dev/null 2>&1; then \
			echo "waiting for existing cert-manager controller to become ready before Helm apply"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cert-manager --timeout=180s; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cert-manager-webhook >/dev/null 2>&1; then \
			echo "waiting for existing cert-manager webhook to become ready before Helm apply"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cert-manager-webhook --timeout=180s; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cert-manager-cainjector >/dev/null 2>&1; then \
			echo "waiting for existing cert-manager cainjector to become ready before Helm apply"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cert-manager-cainjector --timeout=180s; \
		fi; \
		obs_enabled="$$(awk ' \
			/^[^[:space:]]/ { top=$$1; sub(":", "", top); section="" } \
			top == "components" && /^[[:space:]]+observabilityLite:/ { section="observabilityLite" } \
			top == "components" && section == "observabilityLite" && /^[[:space:]]+enabled:[[:space:]]*true[[:space:]]*$$/ { found=1 } \
			END { print found ? "True" : "False" } \
		' $(LOCAL_PLATFORM_VALUES))"; \
		if [ "$$obs_enabled" = "True" ]; then \
			ensure_prometheus_operator_crds; \
		fi; \
		echo "installing platform release via Helm"; \
		run_platform_helm $$keycloak_first_pass_flags; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cloudnative-pg >/dev/null 2>&1; then \
			echo "waiting for CloudNativePG operator to become ready"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cloudnative-pg --timeout=180s; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get svc cnpg-webhook-service >/dev/null 2>&1; then \
			wait_endpoints cnpg-webhook-service 36 "CNPG webhook"; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cert-manager >/dev/null 2>&1; then \
			echo "waiting for cert-manager controller to become ready"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cert-manager --timeout=180s; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cert-manager-webhook >/dev/null 2>&1; then \
			echo "waiting for cert-manager webhook to become ready"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cert-manager-webhook --timeout=180s; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cert-manager-cainjector >/dev/null 2>&1; then \
			echo "waiting for cert-manager cainjector to become ready"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cert-manager-cainjector --timeout=180s; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-ingress-nginx-controller >/dev/null 2>&1; then \
			echo "waiting for ingress-nginx controller to become ready"; \
			$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-ingress-nginx-controller --timeout=180s; \
		fi; \
		if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get svc $(PLATFORM_RELEASE)-ingress-nginx-controller-admission >/dev/null 2>&1; then \
			wait_endpoints $(PLATFORM_RELEASE)-ingress-nginx-controller-admission 36 "ingress-nginx admission"; \
		fi; \
		if [ -n "$$keycloak_first_pass_flags" ]; then \
			wait_condition_obj cluster.postgresql.cnpg.io $(PLATFORM_RELEASE)-core Ready $(WAIT_TIMEOUT); \
			wait_secret $(DB_MIGRATION_SECRET) 36; \
			wait_endpoints $(DB_MIGRATION_CLUSTER)-rw 36 "CNPG rw service"; \
		fi; \
		echo "running second platform Helm reconcile for CRD-backed resources"; \
		run_platform_helm; \
		if [ "$$(uname -s)" = "Darwin" ] && [ "$(LOCAL_PLATFORM_AUTO_TRUST_TLS)" = "1" ] && $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get secret pulse-platform-local-tls >/dev/null 2>&1; then \
			$(MAKE) --no-print-directory local-trust-platform-tls; \
		fi

platform-wait:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ns="$(PLATFORM_NAMESPACE)"; \
	describe_cluster() { \
		echo "current node state:"; \
		$(LOCAL_KUBECTL) get nodes -o wide || true; \
		echo "current platform pod state:"; \
		$(LOCAL_KUBECTL) -n "$$ns" get pods -o wide || true; \
	}; \
	wait_nodes_ready() { \
		echo "waiting for k3d nodes to become Ready"; \
		if ! $(LOCAL_KUBECTL) wait --for=condition=Ready node --all --timeout=$(WAIT_TIMEOUT); then \
			echo "k3d node readiness failed"; \
			describe_cluster; \
			exit 1; \
		fi; \
	}; \
	wait_endpoints_required() { \
		name="$$1"; attempts="$$2"; label="$$3"; \
		if ! $(LOCAL_KUBECTL) -n "$$ns" get endpoints "$$name" >/dev/null 2>&1; then \
			echo "$$label endpoint object ($$name) not found"; \
			describe_cluster; \
			exit 1; \
		fi; \
		for _ in $$(seq 1 "$$attempts"); do \
			endpoint_ips="$$( $(LOCAL_KUBECTL) -n "$$ns" get endpoints "$$name" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true )"; \
			if [ -n "$$endpoint_ips" ]; then \
				echo "$$label endpoints ready: $$endpoint_ips"; \
				return 0; \
			fi; \
			sleep 5; \
		done; \
		echo "$$label endpoints did not become ready"; \
		describe_cluster; \
		exit 1; \
	}; \
	wait_rollout() { \
		kind="$$1"; name="$$2"; timeout="$$3"; \
		if $(LOCAL_KUBECTL) -n "$$ns" get "$$kind" "$$name" >/dev/null 2>&1; then \
			echo "waiting for $$kind/$$name"; \
			$(LOCAL_KUBECTL) -n "$$ns" rollout status "$$kind/$$name" --timeout="$$timeout"; \
		fi; \
	}; \
	wait_condition() { \
		kind="$$1"; name="$$2"; condition="$$3"; timeout="$$4"; \
		if $(LOCAL_KUBECTL) -n "$$ns" get "$$kind" "$$name" >/dev/null 2>&1; then \
			echo "waiting for $$kind/$$name condition=$$condition"; \
			$(LOCAL_KUBECTL) -n "$$ns" wait --for=condition="$$condition" "$$kind/$$name" --timeout="$$timeout"; \
		fi; \
	}; \
	recover_cnpg_stuck_replica() { \
		cluster_name="$(PLATFORM_RELEASE)-core"; \
		if ! $(LOCAL_KUBECTL) -n "$$ns" get cluster.postgresql.cnpg.io "$$cluster_name" >/dev/null 2>&1; then \
			return 1; \
		fi; \
		current_primary="$$( $(LOCAL_KUBECTL) -n "$$ns" get cluster.postgresql.cnpg.io "$$cluster_name" -o jsonpath='{.status.currentPrimary}' 2>/dev/null || true )"; \
		ready_instances="$$( $(LOCAL_KUBECTL) -n "$$ns" get cluster.postgresql.cnpg.io "$$cluster_name" -o jsonpath='{.status.readyInstances}' 2>/dev/null || true )"; \
		spec_instances="$$( $(LOCAL_KUBECTL) -n "$$ns" get cluster.postgresql.cnpg.io "$$cluster_name" -o jsonpath='{.spec.instances}' 2>/dev/null || true )"; \
		cluster_phase_reason="$$( $(LOCAL_KUBECTL) -n "$$ns" get cluster.postgresql.cnpg.io "$$cluster_name" -o jsonpath='{.status.phaseReason}' 2>/dev/null || true )"; \
		if [ -z "$$current_primary" ] || [ -z "$$spec_instances" ] || [ -z "$$ready_instances" ]; then \
			return 1; \
		fi; \
		if [ "$$ready_instances" = "$$spec_instances" ]; then \
			return 1; \
		fi; \
		replica_pod="$$( $(LOCAL_KUBECTL) -n "$$ns" get pods -l cnpg.io/cluster="$$cluster_name" -o jsonpath='{range .items[*]}{.metadata.name} {.status.containerStatuses[0].ready}{"\n"}{end}' | awk '$$1 != "'"$$current_primary"'" && $$2 != "true" {print $$1; exit}' )"; \
		if [ -z "$$replica_pod" ]; then \
			return 1; \
		fi; \
		replica_phase="$$( $(LOCAL_KUBECTL) -n "$$ns" get pod "$$replica_pod" -o jsonpath='{.status.phase}' 2>/dev/null || true )"; \
		replica_pvc_exists=0; \
		if $(LOCAL_KUBECTL) -n "$$ns" get pvc "$$replica_pod" >/dev/null 2>&1; then \
			replica_pvc_exists=1; \
		fi; \
		replica_logs="$$( $(LOCAL_KUBECTL) -n "$$ns" logs "$$replica_pod" --all-containers --tail=80 2>/dev/null || true )"; \
		case "$$cluster_phase_reason $$replica_logs" in \
			*"Timeout: request did not complete within requested timeout"*|*"Failed to execute pg_rewind"*|*"could not restore file"*) \
				echo "detected stuck CNPG replica $$replica_pod; scaling CNPG to the healthy primary before recloning"; \
				$(LOCAL_KUBECTL) -n "$$ns" patch cluster.postgresql.cnpg.io "$$cluster_name" --type=merge -p '{"spec":{"instances":1}}'; \
				$(LOCAL_KUBECTL) -n "$$ns" delete pod "$$replica_pod" --ignore-not-found=true --wait=true; \
				if $(LOCAL_KUBECTL) -n "$$ns" get pvc "$$replica_pod" >/dev/null 2>&1; then \
					$(LOCAL_KUBECTL) -n "$$ns" delete pvc "$$replica_pod" --wait=true; \
				fi; \
				$(LOCAL_KUBECTL) -n "$$ns" wait --for=condition=Ready cluster.postgresql.cnpg.io/"$$cluster_name" --timeout=180s; \
				echo "restoring CNPG replica count=$$spec_instances"; \
				$(LOCAL_KUBECTL) -n "$$ns" patch cluster.postgresql.cnpg.io "$$cluster_name" --type=merge -p "{\"spec\":{\"instances\":$$spec_instances}}"; \
				return 0; \
				;; \
		esac; \
		if [ "$$replica_phase" = "Pending" ] && [ "$$replica_pvc_exists" = "0" ]; then \
			echo "detected CNPG replica $$replica_pod pending without PVC; recycling replica count to force PVC recreation"; \
			$(LOCAL_KUBECTL) -n "$$ns" patch cluster.postgresql.cnpg.io "$$cluster_name" --type=merge -p '{"spec":{"instances":1}}'; \
			$(LOCAL_KUBECTL) -n "$$ns" delete pod "$$replica_pod" --ignore-not-found=true --wait=true; \
			$(LOCAL_KUBECTL) -n "$$ns" wait --for=condition=Ready cluster.postgresql.cnpg.io/"$$cluster_name" --timeout=180s; \
			echo "restoring CNPG replica count=$$spec_instances"; \
			$(LOCAL_KUBECTL) -n "$$ns" patch cluster.postgresql.cnpg.io "$$cluster_name" --type=merge -p "{\"spec\":{\"instances\":$$spec_instances}}"; \
			return 0; \
		fi; \
		return 1; \
	}; \
	wait_job_complete() { \
		name="$$1"; timeout="$$2"; \
		if $(LOCAL_KUBECTL) -n "$$ns" get job "$$name" >/dev/null 2>&1; then \
			echo "waiting for job/$$name condition=complete"; \
			$(LOCAL_KUBECTL) -n "$$ns" wait --for=condition=complete job/"$$name" --timeout="$$timeout"; \
		fi; \
	}; \
	verify_minio_bucket() { \
		if ! command -v $(DOCKER) >/dev/null 2>&1; then \
			echo "$(DOCKER) not found. Install Docker first to verify MinIO bucket bootstrap."; \
			exit 1; \
		fi; \
		archive_secret="$(ARCHIVE_INTEGRATION_SECRET)"; \
		root_user="$$( $(LOCAL_KUBECTL) -n "$$ns" get secret "$$archive_secret" -o jsonpath='{.data.rootUser}' | base64 -d )"; \
		root_pass="$$( $(LOCAL_KUBECTL) -n "$$ns" get secret "$$archive_secret" -o jsonpath='{.data.rootPassword}' | base64 -d )"; \
		pf_log="$$(mktemp -t pulse-platform-minio-bucket-check.XXXXXX.log)"; \
		$(LOCAL_KUBECTL) -n "$$ns" port-forward "svc/$(ARCHIVE_INTEGRATION_SERVICE)" "$(DR_MINIO_LOCAL_PORT):9000" >"$$pf_log" 2>&1 & \
		pf_pid=$$!; \
		cleanup_pf() { \
			kill "$$pf_pid" >/dev/null 2>&1 || true; \
			wait "$$pf_pid" >/dev/null 2>&1 || true; \
		}; \
		ready=0; \
		for _ in {1..30}; do \
			if ! kill -0 "$$pf_pid" >/dev/null 2>&1; then \
				break; \
			fi; \
			if command -v nc >/dev/null 2>&1 && nc -z 127.0.0.1 "$(DR_MINIO_LOCAL_PORT)" >/dev/null 2>&1; then \
				ready=1; \
				break; \
			fi; \
			if command -v curl >/dev/null 2>&1 && curl --silent --fail --max-time 2 "http://127.0.0.1:$(DR_MINIO_LOCAL_PORT)/minio/health/live" >/dev/null 2>&1; then \
				ready=1; \
				break; \
			fi; \
			sleep 1; \
		done; \
		if [ "$$ready" -ne 1 ]; then \
			echo "minio port-forward did not become ready; see $$pf_log"; \
			cleanup_pf; \
			exit 1; \
		fi; \
		echo "verifying MinIO bucket $(DR_ARCHIVE_BUCKET)"; \
		$(DOCKER) run --rm \
			--entrypoint /bin/sh \
			-e DR_DOCKER_ENDPOINT="$(DR_MINIO_DOCKER_ENDPOINT)" \
			-e DR_ROOT_USER="$$root_user" \
			-e DR_ROOT_PASS="$$root_pass" \
			-e DR_BUCKET="$(DR_ARCHIVE_BUCKET)" \
			"$(DR_MINIO_MC_IMAGE)" \
			-c 'set -e; mc alias set local "http://$$DR_DOCKER_ENDPOINT" "$$DR_ROOT_USER" "$$DR_ROOT_PASS" >/dev/null; mc ls "local/$$DR_BUCKET" >/dev/null'; \
		cleanup_pf; \
		rm -f "$$pf_log"; \
		}; \
		wait_nodes_ready; \
		wait_rollout deployment $(PLATFORM_RELEASE)-cloudnative-pg 180s; \
		if ! wait_condition cluster.postgresql.cnpg.io $(PLATFORM_RELEASE)-core Ready 15s; then \
			if recover_cnpg_stuck_replica; then \
				echo "retrying CNPG cluster wait after local replica repair"; \
				wait_condition cluster.postgresql.cnpg.io $(PLATFORM_RELEASE)-core Ready $(WAIT_TIMEOUT); \
			else \
				wait_condition cluster.postgresql.cnpg.io $(PLATFORM_RELEASE)-core Ready $(WAIT_TIMEOUT); \
			fi; \
		fi; \
		wait_rollout statefulset $(PLATFORM_RELEASE)-nats $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(PLATFORM_RELEASE)-valkey-node $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(PLATFORM_RELEASE)-keycloak $(WAIT_TIMEOUT); \
	wait_job_complete $(PLATFORM_RELEASE)-keycloak-keycloak-config-cli 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-minio 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-ingress-nginx-controller 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-cert-manager 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-cert-manager-webhook 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-cert-manager-cainjector 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-external-secrets-cert-controller 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-external-secrets 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-external-secrets-webhook 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-kube-promet-operator 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-grafana 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-opentelemetry-collector 300s; \
	wait_endpoints_required $(PLATFORM_RELEASE)-core-rw 36 "CNPG rw service"; \
	wait_endpoints_required $(PLATFORM_RELEASE)-nats 36 "NATS service"; \
	wait_endpoints_required $(PLATFORM_RELEASE)-valkey 36 "Valkey service"; \
	wait_endpoints_required $(PLATFORM_RELEASE)-minio 36 "MinIO service"; \
	wait_endpoints_required $(PLATFORM_RELEASE)-keycloak-headless 36 "Keycloak service"; \
	verify_minio_bucket; \
	echo "platform dependencies are ready"
	@set -euo pipefail; \
	ns="$(PLATFORM_NAMESPACE)"; \
	wait_rollout() { \
		kind="$$1"; name="$$2"; timeout="$$3"; \
		if $(LOCAL_KUBECTL) -n "$$ns" get "$$kind" "$$name" >/dev/null 2>&1; then \
			echo "waiting for $$kind/$$name"; \
			$(LOCAL_KUBECTL) -n "$$ns" rollout status "$$kind/$$name" --timeout="$$timeout"; \
		fi; \
	}; \
	wait_rollout deployment $(PLATFORM_RELEASE)-public-app 300s; \
	wait_rollout deployment $(PLATFORM_RELEASE)-realtime-gateway 300s

local-trust-platform-tls:
	@set -euo pipefail; \
	if [ "$$(uname -s)" != "Darwin" ]; then \
		echo "local-trust-platform-tls is currently supported on macOS only"; \
		exit 1; \
	fi; \
	tmp_cert="$$(mktemp /tmp/pulse-platform-local-tls.XXXXXX.crt)"; \
	trap 'rm -f "$$tmp_cert"' EXIT INT TERM; \
	echo "exporting pulse-platform local CA certificate from cluster"; \
	kubectl --context "$(K3D_CONTEXT)" -n "$(PLATFORM_NAMESPACE)" get secret pulse-platform-local-ca -o jsonpath='{.data.tls\.crt}' | base64 -d > "$$tmp_cert"; \
	fingerprint="$$(openssl x509 -in "$$tmp_cert" -noout -fingerprint -sha256 | sed 's/^.*=//; s/://g')"; \
	if security find-certificate -a -Z "$$HOME/Library/Keychains/login.keychain-db" 2>/dev/null | tr '[:lower:]' '[:upper:]' | grep -q "$$fingerprint"; then \
		echo "localhost TLS CA already trusted in login keychain"; \
		exit 0; \
	fi; \
	echo "adding CA certificate to login keychain trust store"; \
	security add-trusted-cert -d -r trustRoot -k "$$HOME/Library/Keychains/login.keychain-db" "$$tmp_cert"; \
	echo "trusted localhost TLS CA for pulse-platform"

local-trust-platform-tls-system:
	@set -euo pipefail; \
	if [ "$$(uname -s)" != "Darwin" ]; then \
		echo "local-trust-platform-tls-system is currently supported on macOS only"; \
		exit 1; \
	fi; \
	tmp_cert="$$(mktemp /tmp/pulse-platform-local-ca.XXXXXX.crt)"; \
	trap 'rm -f "$$tmp_cert"' EXIT INT TERM; \
	echo "exporting pulse-platform local CA certificate from cluster"; \
	kubectl --context "$(K3D_CONTEXT)" -n "$(PLATFORM_NAMESPACE)" get secret pulse-platform-local-ca -o jsonpath='{.data.tls\.crt}' | base64 -d > "$$tmp_cert"; \
	echo "adding CA certificate to System keychain trust store (admin password may be required)"; \
	sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$$tmp_cert"; \
	echo "trusted localhost TLS CA for pulse-platform in System keychain"

edge-verify-http3-local:
	@set -euo pipefail; \
	if ! command -v curl >/dev/null 2>&1; then \
		echo "curl not found. Install a curl build with HTTP/3 support."; \
		exit 1; \
	fi; \
	if ! curl -V 2>/dev/null | grep -q 'Features:.*HTTP3'; then \
		echo "curl is installed, but the linked libcurl lacks HTTP/3 support; install an HTTP/3-capable curl before running this check."; \
		exit 1; \
	fi; \
	if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi; \
	url="$${HTTP3_VERIFY_URL:-https://localhost}"; \
	echo "verifying local HTTP/3 edge at $$url"; \
	$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get svc $(PLATFORM_RELEASE)-public-edge-http3 >/dev/null 2>&1 || { \
		echo "service/$(PLATFORM_RELEASE)-public-edge-http3 not found in $(PLATFORM_NAMESPACE); local HTTP/3 appears disabled"; \
		exit 1; \
	}; \
	headers="$$(curl --http2 -sS -I "$$url")"; \
	printf '%s\n' "$$headers" | grep -qi '^alt-svc: .*h3=' || { \
		echo "Alt-Svc h3 advertisement missing from $$url"; \
		exit 1; \
	}; \
	version="$$(curl --http3-only -sS -o /dev/null -w '%{http_version}' "$$url")"; \
	if [ "$$version" != "3" ]; then \
		echo "expected curl HTTP version 3, got $$version"; \
		exit 1; \
	fi; \
	echo "verified HTTP/3 via curl (--http3-only) and Alt-Svc on $$url"

local-cloud-db-env: gke-cloud-context
	@set -euo pipefail; \
	mkdir -p "$(dir $(LOCAL_CLOUD_DB_SERVICES_VALUES))"; \
	user="$$( $(KUBECTL) -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.username}' | base64 -d )"; \
	pass="$$( $(KUBECTL) -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.password}' | base64 -d )"; \
	dsn="host=$(LOCAL_CLOUD_DB_HOST) port=$(LOCAL_CLOUD_DB_PORT) user=$$user password=$$pass dbname=$(DB_MIGRATION_DB) sslmode=disable"; \
	quote_single() { printf "%s" "$$1" | sed "s/'/'\\\\''/g"; }; \
	qdsn="$$(quote_single "$$dsn")"; \
	umask 077; \
	{ \
		echo "# Generated by make local-cloud-db-env. Keep this file local."; \
		echo "# In another shell, keep the forward open: make cloud-db-forward"; \
		echo "runtime:"; \
		echo "  env:"; \
		printf "    controlPlaneDBDSN: '%s'\n" "$$qdsn"; \
		printf "    archiveManifestDBDSN: '%s'\n" "$$qdsn"; \
		echo "    pulseMqttEmulatorEnabled: 'false'"; \
		printf "    natsURLs: 'nats://%s:%s'\n" "$(LOCAL_CLOUD_REALTIME_HOST)" "$(LOCAL_CLOUD_NATS_PORT)"; \
		printf "    valkeyAddrs: '%s:%s'\n" "$(LOCAL_CLOUD_REALTIME_HOST)" "$(LOCAL_CLOUD_VALKEY_PORT)"; \
		echo "    valkeySentinelMasterSet: ''"; \
		echo "    projectionKeyPrefix: 'pulse:cloud-projection'"; \
		echo "    weatherKeyPrefix: 'pulse:cloud-weather'"; \
		echo "    inferenceKeyPrefix: 'pulse:cloud-inference'"; \
		echo "    providerMqttSessionCacheKeyPrefix: 'pulse:cloud-provider-mqtt-session'"; \
		echo "    energyCacheKeyPrefix: 'pulse:cloud-energy'"; \
		echo "  workers:"; \
		for worker in ingest inference projection rollup archive solarVerification scheduler pulseMqttEmulator; do \
			echo "    $$worker:"; \
			echo "      enabled: false"; \
		done; \
		echo "    grpcApi:"; \
		echo "      enabled: true"; \
		echo "    energyApi:"; \
		echo "      enabled: true"; \
	} > "$(LOCAL_CLOUD_DB_SERVICES_VALUES)"; \
	chmod 600 "$(LOCAL_CLOUD_DB_SERVICES_VALUES)"; \
	echo "wrote $(LOCAL_CLOUD_DB_SERVICES_VALUES) for $(LOCAL_CLOUD_DB_HOST):$(LOCAL_CLOUD_DB_PORT) without printing credentials"

local-cloud-realtime-env:
	@set -euo pipefail; \
	mkdir -p "$(dir $(LOCAL_CLOUD_REALTIME_PLATFORM_VALUES))"; \
	umask 077; \
	{ \
		echo "# Generated by make local-cloud-realtime-env. Keep this file local."; \
		echo "# In another shell, keep the forwards open: make cloud-realtime-forward-start CLOUD_REALTIME_FORWARD_ADDRESS=$(LOCAL_CLOUD_REALTIME_FORWARD_ADDRESS)"; \
		echo "runtime:"; \
		echo "  publicApp:"; \
		echo "    env:"; \
		echo "      dataPlane: 'cloud'"; \
		echo "  realtimeGateway:"; \
		echo "    env:"; \
		printf "      natsURLs: 'nats://%s:%s'\n" "$(LOCAL_CLOUD_REALTIME_HOST)" "$(LOCAL_CLOUD_NATS_PORT)"; \
		printf "      valkeyAddrs: '%s:%s'\n" "$(LOCAL_CLOUD_REALTIME_HOST)" "$(LOCAL_CLOUD_VALKEY_PORT)"; \
		echo "      valkeySentinelMasterSet: ''"; \
		echo "      projectionKeyPrefix: 'pulse:cloud-projection'"; \
		} > "$(LOCAL_CLOUD_REALTIME_PLATFORM_VALUES)"; \
	chmod 600 "$(LOCAL_CLOUD_REALTIME_PLATFORM_VALUES)"; \
	echo "wrote $(LOCAL_CLOUD_REALTIME_PLATFORM_VALUES) for cloud NATS/Valkey via $(LOCAL_CLOUD_REALTIME_HOST)"

services-up: helm-local-ready
	@if [ "$(SERVICES_AUTO_BUILD_IMAGE)" = "1" ]; then \
		$(MAKE) services-image-local-up; \
	fi
	@set -euo pipefail; \
		ns="$(PLATFORM_NAMESPACE)"; \
		wait_nodes_ready() { \
			echo "waiting for k3d nodes to become Ready"; \
			$(LOCAL_KUBECTL) wait --for=condition=Ready node --all --timeout=$(WAIT_TIMEOUT); \
		}; \
		wait_endpoints() { \
			name="$$1"; attempts="$$2"; label="$$3"; \
			if ! $(LOCAL_KUBECTL) -n "$$ns" get endpoints "$$name" >/dev/null 2>&1; then \
				echo "$$label endpoint object ($$name) not found"; \
				exit 1; \
			fi; \
			for _ in $$(seq 1 "$$attempts"); do \
				endpoint_ips="$$( $(LOCAL_KUBECTL) -n "$$ns" get endpoints "$$name" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true )"; \
				if [ -n "$$endpoint_ips" ]; then \
					echo "$$label endpoints ready: $$endpoint_ips"; \
					return 0; \
				fi; \
				sleep 5; \
			done; \
			echo "$$label endpoints did not become ready"; \
			exit 1; \
		}; \
		wait_nodes_ready; \
		echo "verifying platform dependency endpoints before services rollout"; \
		wait_endpoints $(PLATFORM_RELEASE)-core-rw 36 "CNPG rw service"; \
		wait_endpoints $(PLATFORM_RELEASE)-nats 36 "NATS service"; \
		wait_endpoints $(PLATFORM_RELEASE)-valkey 36 "Valkey service"; \
		wait_endpoints $(PLATFORM_RELEASE)-minio 36 "MinIO service"; \
		wait_endpoints $(PLATFORM_RELEASE)-keycloak-headless 36 "Keycloak service"
	@$(MAKE) --no-print-directory chart-deps-local CHART=$(SERVICES_CHART)
	@set -euo pipefail; \
	set --; \
	for values in $(LOCAL_SERVICES_VALUES); do \
		set -- "$$@" -f "$$values"; \
	done; \
	echo "applying $(SERVICES_RELEASE) with values: $(LOCAL_SERVICES_VALUES)"; \
	$(LOCAL_HELM) upgrade --install $(SERVICES_RELEASE) $(SERVICES_CHART) \
		--namespace $(SERVICES_NAMESPACE) --create-namespace \
		$(LOCAL_HELM_UPGRADE_FLAGS) \
		"$$@"
	@if [ "$(SERVICES_AUTO_BUILD_IMAGE)" = "1" ]; then \
		echo "restarting $(SERVICES_RELEASE) deployments to pick up imported :local image"; \
		$(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) rollout restart deploy -l app.kubernetes.io/instance=$(SERVICES_RELEASE); \
	fi

services-up-cloud-db:
	$(MAKE) --no-print-directory cloud-db-forward-start \
		CLOUD_DB_FORWARD_ADDRESS="$(LOCAL_CLOUD_DB_FORWARD_ADDRESS)"
	$(MAKE) --no-print-directory local-cloud-db-env
	$(MAKE) --no-print-directory services-up \
		LOCAL_SERVICES_VALUES="$(LOCAL_SERVICES_VALUES) $(LOCAL_CLOUD_DB_SERVICES_VALUES)"

services-wait:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ns="$(SERVICES_NAMESPACE)"; \
	describe_services() { \
		echo "current services deployment state:"; \
		$(LOCAL_KUBECTL) -n "$$ns" get deploy -o wide || true; \
		echo "current services pod state:"; \
		$(LOCAL_KUBECTL) -n "$$ns" get pods -o wide || true; \
	}; \
	echo "waiting for k3d nodes to become Ready"; \
	$(LOCAL_KUBECTL) wait --for=condition=Ready node --all --timeout=$(WAIT_TIMEOUT); \
	if ! $(LOCAL_KUBECTL) get ns "$$ns" >/dev/null 2>&1; then \
		echo "namespace $$ns does not exist yet, skipping services wait"; \
		exit 0; \
	fi; \
	if [ -z "$$( $(LOCAL_KUBECTL) -n "$$ns" get deploy -l app.kubernetes.io/instance=$(SERVICES_RELEASE) -o name 2>/dev/null )" ]; then \
		echo "no services workloads found for instance $(SERVICES_RELEASE) in $$ns"; \
		exit 0; \
	fi; \
	echo "waiting for services deployments to finish rolling out"; \
	for deploy in $$($(LOCAL_KUBECTL) -n "$$ns" get deploy -l app.kubernetes.io/instance=$(SERVICES_RELEASE) -o name); do \
		$(LOCAL_KUBECTL) -n "$$ns" rollout status "$$deploy" --timeout=$(WAIT_TIMEOUT); \
		desired="$$( $(LOCAL_KUBECTL) -n "$$ns" get "$$deploy" -o jsonpath='{.spec.replicas}' )"; \
		ready="$$( $(LOCAL_KUBECTL) -n "$$ns" get "$$deploy" -o jsonpath='{.status.readyReplicas}' )"; \
		available="$$( $(LOCAL_KUBECTL) -n "$$ns" get "$$deploy" -o jsonpath='{.status.availableReplicas}' )"; \
		desired="$${desired:-0}"; \
		ready="$${ready:-0}"; \
		available="$${available:-0}"; \
		if [ "$$ready" != "$$desired" ] || [ "$$available" != "$$desired" ]; then \
			echo "$$deploy is not fully healthy after rollout (desired=$$desired ready=$$ready available=$$available)"; \
			describe_services; \
			exit 1; \
		fi; \
	done; \
	echo "services dependencies are ready"

platform-recover-local:
	@set -euo pipefail; \
	echo "starting local cluster recovery flow"; \
	$(MAKE) --no-print-directory k3d-up; \
	if $(LOCAL_KUBECTL) get ns "$(PLATFORM_NAMESPACE)" >/dev/null 2>&1; then \
		echo "restarting critical local platform workloads to clear stranded cold-start state"; \
		for workload in \
			"statefulset/$(PLATFORM_RELEASE)-nats" \
			"statefulset/$(PLATFORM_RELEASE)-valkey-node" \
			"statefulset/$(PLATFORM_RELEASE)-keycloak" \
			"deployment/$(PLATFORM_RELEASE)-minio"; do \
			if $(LOCAL_KUBECTL) -n "$(PLATFORM_NAMESPACE)" get "$$workload" >/dev/null 2>&1; then \
				$(LOCAL_KUBECTL) -n "$(PLATFORM_NAMESPACE)" rollout restart "$$workload"; \
			fi; \
		done; \
	fi; \
	$(MAKE) --no-print-directory platform-up; \
	$(MAKE) --no-print-directory platform-wait; \
	$(MAKE) --no-print-directory services-up; \
	$(MAKE) --no-print-directory services-wait

dev-grafana:
	@$(MAKE) --no-print-directory platform-up
	@set -euo pipefail; \
	ns="$(PLATFORM_NAMESPACE)"; \
	deploy_name="$(PLATFORM_RELEASE)-grafana"; \
	if ! $(LOCAL_KUBECTL) -n "$$ns" get deploy "$$deploy_name" >/dev/null 2>&1; then \
		echo "deployment/$$deploy_name not found in namespace $$ns"; \
		exit 1; \
	fi; \
	echo "waiting for deployment/$$deploy_name"; \
	$(LOCAL_KUBECTL) -n "$$ns" rollout status deploy/"$$deploy_name" --timeout=$(WAIT_TIMEOUT)

dev-up: k3d-up public-images-local-up platform-up platform-wait services-up services-wait

dev-up-cloud-db:
	$(MAKE) --no-print-directory k3d-up
	$(MAKE) --no-print-directory public-images-local-up \
		EXPO_PUBLIC_LOCAL_DATA_PLANE=cloud
	$(MAKE) --no-print-directory cloud-realtime-forward-start \
		CLOUD_REALTIME_FORWARD_ADDRESS="$(LOCAL_CLOUD_REALTIME_FORWARD_ADDRESS)"
	$(MAKE) --no-print-directory local-cloud-realtime-env
	$(MAKE) --no-print-directory platform-up \
		LOCAL_PLATFORM_VALUES="$(LOCAL_PLATFORM_VALUES) $(LOCAL_CLOUD_REALTIME_PLATFORM_VALUES)"
	$(MAKE) --no-print-directory platform-wait
	$(MAKE) --no-print-directory public-deployments-restart-local
	$(MAKE) --no-print-directory services-up-cloud-db
	$(MAKE) --no-print-directory services-wait

local-up: dev-up

local-up-cloud-db: dev-up-cloud-db

local-deploy: dev-deploy

local-deploy-cloud-db: dev-deploy-cloud-db

local-down: dev-down

local-status:
	@echo "local cluster: $(K3D_CONTEXT)"
	@echo "platform namespace ($(PLATFORM_NAMESPACE))"
	@$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get pods
	@echo "services namespace ($(SERVICES_NAMESPACE))"
	@$(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get pods

dev-down:
	@if command -v $(HELM) >/dev/null 2>&1; then \
		$(LOCAL_HELM) uninstall $(SERVICES_RELEASE) --namespace $(SERVICES_NAMESPACE) >/dev/null 2>&1 || true; \
		$(LOCAL_HELM) uninstall $(PLATFORM_RELEASE) --namespace $(PLATFORM_NAMESPACE) >/dev/null 2>&1 || true; \
	else \
		echo "$(HELM) not found; skipping helm uninstall"; \
	fi
	@if command -v $(KUBECTL) >/dev/null 2>&1; then \
		$(LOCAL_KUBECTL) delete namespace $(SERVICES_NAMESPACE) --ignore-not-found >/dev/null 2>&1 || true; \
		$(LOCAL_KUBECTL) delete namespace $(PLATFORM_NAMESPACE) --ignore-not-found >/dev/null 2>&1 || true; \
	fi
	@if [ "$(DELETE_CLUSTER)" = "1" ]; then \
		if command -v $(K3D) >/dev/null 2>&1; then \
			$(K3D) cluster delete $(K3D_CLUSTER_NAME) || true; \
		else \
			echo "$(K3D) not found; cannot delete cluster"; \
		fi; \
	else \
		echo "cluster preserved (set DELETE_CLUSTER=1 to delete)"; \
	fi

# public-images-local-up rebuilds and imports the updated pulse-platform and pulse-realtime-gateway images.
# services-up re-applies Helm and, by default, auto-builds/imports the Go workers image.
# dev-web-deploy owns the public web app + realtime gateway local redeploy path for k3d.
# Set DEV_DEPLOY_SKIP_PUBLIC_RESTART=1 when another target needs platform prep/image import without
# immediately restarting the public deployments.
# dev-deploy defaults to a Helm fast path (`DEV_DEPLOY_HELM=auto`) that skips Helm re-apply unless the
# local chart/values files changed or the release is missing. Use `DEV_DEPLOY_HELM=always` to force full Helm apply.
# The rollout restart calls are important because the images use the same :local tag with IfNotPresent, so importing alone will not replace already-running pods.
public-deployments-restart-local:
	@set -euo pipefail; \
		restart_and_wait_if_exists() { \
			ns="$$1"; \
			name="$$2"; \
			if $(LOCAL_KUBECTL) -n "$$ns" get deploy/"$$name" >/dev/null 2>&1; then \
				echo "restarting $$ns/$$name"; \
				$(LOCAL_KUBECTL) -n "$$ns" rollout restart deploy/"$$name"; \
				echo "waiting for $$ns/$$name"; \
				$(LOCAL_ROLLOUT_STATUS) "$$ns" "$$name" 300s; \
			else \
				echo "skipping missing deployment $$ns/$$name"; \
			fi; \
		}; \
		echo "restarting updated public deployments"; \
		restart_and_wait_if_exists $(PLATFORM_NAMESPACE) pulse-platform-realtime-gateway; \
		restart_and_wait_if_exists $(PLATFORM_NAMESPACE) pulse-platform-public-app

dev-web-deploy:
	@set -euo pipefail; \
		if [ -f .env ]; then \
			set -a; source ./.env; set +a; \
		fi; \
		data_mode="$$(DEV_DEPLOY_DATA_MODE="$(DEV_DEPLOY_DATA_MODE)" \
			EXPO_PUBLIC_LOCAL_DATA_PLANE="$${EXPO_PUBLIC_LOCAL_DATA_PLANE:-}" \
			KUBECTL="$(KUBECTL)" \
			K3D_CONTEXT="$(K3D_CONTEXT)" \
			PLATFORM_NAMESPACE="$(PLATFORM_NAMESPACE)" \
			SERVICES_NAMESPACE="$(SERVICES_NAMESPACE)" \
			PLATFORM_RELEASE="$(PLATFORM_RELEASE)" \
			SERVICES_RELEASE="$(SERVICES_RELEASE)" \
			sh scripts/local-dev-data-mode.sh)"; \
		echo "dev-web-deploy data mode: $$data_mode"; \
		helm_mode="$(DEV_DEPLOY_HELM)"; \
		platform_apply=0; \
		case "$$helm_mode" in \
			always|1|true) \
				platform_apply=1; \
				;; \
			never|0|false) \
				;; \
			auto) \
				if ! $(LOCAL_HELM) status $(PLATFORM_RELEASE) --namespace $(PLATFORM_NAMESPACE) >/dev/null 2>&1; then \
					platform_apply=1; \
				fi; \
				if [ -n "$$(git status --porcelain --untracked-files=all -- $(PLATFORM_CHART) $(LOCAL_PLATFORM_VALUES))" ]; then \
					platform_apply=1; \
				fi; \
				if [ "$$platform_apply" = "0" ]; then \
					if ! $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy/pulse-platform-realtime-gateway >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy/pulse-platform-public-app >/dev/null 2>&1; then \
						platform_apply=1; \
					fi; \
				fi; \
				if [ "$$platform_apply" = "0" ] && [ -n "$${PULSE_PLATFORM_DEV_SUBJECT:-}" ]; then \
					current_subject="$$( $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy/pulse-platform-public-app -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="PULSE_PLATFORM_DEV_SUBJECT")].value}' 2>/dev/null || true )"; \
					if [ "$$current_subject" != "$${PULSE_PLATFORM_DEV_SUBJECT}" ]; then \
						echo "detected changed local noop subject override for pulse-platform public app"; \
						platform_apply=1; \
					fi; \
				fi; \
				if [ "$$platform_apply" = "0" ] && [ -n "$${KEYCLOAK_SOCIAL_GOOGLE_CLIENT_ID:-}" ]; then \
					current_google_client_id="$$( $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get secret pulse-platform-keycloak-social-providers -o jsonpath='{.data.KEYCLOAK_SOCIAL_GOOGLE_CLIENT_ID}' 2>/dev/null | base64 -d || true )"; \
					if [ "$$current_google_client_id" != "$${KEYCLOAK_SOCIAL_GOOGLE_CLIENT_ID}" ]; then \
						echo "detected changed local Keycloak Google client id"; \
						platform_apply=1; \
					fi; \
				fi; \
				;; \
			*) \
				echo "unsupported DEV_DEPLOY_HELM=$$helm_mode (expected auto, always, or never)"; \
				exit 1; \
				;; \
		esac; \
		if [ "$$data_mode" = "local" ] && [ "$$platform_apply" = "0" ]; then \
			current_data_plane="$$( $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy/pulse-platform-public-app -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="PULSE_PLATFORM_DATA_PLANE")].value}' 2>/dev/null || true )"; \
			if [ "$$current_data_plane" = "cloud" ]; then \
				echo "detected Local Edge platform release; applying full-local platform values"; \
				platform_apply=1; \
			fi; \
		fi; \
		if [ "$$data_mode" = "local-edge" ]; then \
			platform_apply=1; \
			$(MAKE) --no-print-directory cloud-realtime-forward-start \
				CLOUD_REALTIME_FORWARD_ADDRESS="$(LOCAL_CLOUD_REALTIME_FORWARD_ADDRESS)"; \
			$(MAKE) --no-print-directory local-cloud-realtime-env; \
		fi; \
		$(MAKE) --no-print-directory k3d-up; \
		if [ "$$data_mode" = "local-edge" ]; then \
			$(MAKE) --no-print-directory public-images-local-up \
				EXPO_PUBLIC_LOCAL_DATA_PLANE=cloud; \
		else \
			$(MAKE) --no-print-directory public-images-local-up \
				EXPO_PUBLIC_LOCAL_DATA_PLANE=local; \
		fi; \
		if [ "$$platform_apply" = "1" ]; then \
			echo "applying platform Helm release"; \
			if [ "$$data_mode" = "local-edge" ]; then \
				$(MAKE) --no-print-directory platform-up \
					LOCAL_PLATFORM_VALUES="$(LOCAL_PLATFORM_VALUES) $(LOCAL_CLOUD_REALTIME_PLATFORM_VALUES)"; \
			else \
				$(MAKE) --no-print-directory platform-up; \
			fi; \
		else \
			echo "skipping platform Helm apply (set DEV_DEPLOY_HELM=always to force)"; \
		fi; \
		$(MAKE) --no-print-directory platform-wait
	@set -euo pipefail; \
		if [ "$${DEV_DEPLOY_SKIP_PUBLIC_RESTART:-0}" = "1" ]; then \
			echo "skipping public deployment restart (DEV_DEPLOY_SKIP_PUBLIC_RESTART=1)"; \
		else \
			$(MAKE) --no-print-directory public-deployments-restart-local; \
		fi
	@echo "showing platform deployment state and recent realtime gateway logs"
	$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy
	$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) logs deploy/pulse-platform-realtime-gateway --since=5m

dev-web-deploy-cloud-realtime:
	$(MAKE) --no-print-directory dev-web-deploy \
		DEV_DEPLOY_DATA_MODE=local-edge \
		DEV_DEPLOY_HELM=always \
		EXPO_PUBLIC_LOCAL_DATA_PLANE=cloud

dev-deploy-cloud-db:
	$(MAKE) --no-print-directory dev-deploy \
		DEV_DEPLOY_DATA_MODE=local-edge \
		DEV_DEPLOY_HELM=always \
		EXPO_PUBLIC_LOCAL_DATA_PLANE=cloud

dev-deploy:
	@set -euo pipefail; \
		if [ -f .env ]; then \
			set -a; source ./.env; set +a; \
		fi; \
		data_mode="$$(DEV_DEPLOY_DATA_MODE="$(DEV_DEPLOY_DATA_MODE)" \
			EXPO_PUBLIC_LOCAL_DATA_PLANE="$${EXPO_PUBLIC_LOCAL_DATA_PLANE:-}" \
			KUBECTL="$(KUBECTL)" \
			K3D_CONTEXT="$(K3D_CONTEXT)" \
			PLATFORM_NAMESPACE="$(PLATFORM_NAMESPACE)" \
			SERVICES_NAMESPACE="$(SERVICES_NAMESPACE)" \
			PLATFORM_RELEASE="$(PLATFORM_RELEASE)" \
			SERVICES_RELEASE="$(SERVICES_RELEASE)" \
			sh scripts/local-dev-data-mode.sh)"; \
		echo "dev-deploy data mode: $$data_mode"; \
		helm_mode="$(DEV_DEPLOY_HELM)"; \
		services_apply=0; \
		case "$$helm_mode" in \
			always|1|true) \
				services_apply=1; \
				;; \
			never|0|false) \
				;; \
			auto) \
				if ! $(LOCAL_HELM) status $(SERVICES_RELEASE) --namespace $(SERVICES_NAMESPACE) >/dev/null 2>&1; then \
					services_apply=1; \
				fi; \
				if [ -n "$$(git status --porcelain --untracked-files=all -- $(SERVICES_CHART) $(LOCAL_SERVICES_VALUES))" ]; then \
					services_apply=1; \
				fi; \
				if [ "$$services_apply" = "0" ]; then \
					if ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-ingest >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-projection >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-archive >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-inference >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-grpc-api >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-energy-api >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-rollup >/dev/null 2>&1; then \
						services_apply=1; \
					fi; \
				fi; \
				;; \
			*) \
				echo "unsupported DEV_DEPLOY_HELM=$$helm_mode (expected auto, always, or never)"; \
				exit 1; \
				;; \
		esac; \
		if [ "$$data_mode" = "local-edge" ]; then \
			services_apply=1; \
		fi; \
		if [ "$$data_mode" = "local" ] && [ "$$services_apply" = "0" ]; then \
			current_projection_prefix="$$( $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get configmap/$(SERVICES_RELEASE)-runtime-env -o jsonpath='{.data.PROJECTION_KEY_PREFIX}' 2>/dev/null || true )"; \
			case "$$current_projection_prefix" in \
				*cloud*) \
					echo "detected Local Edge services release; applying full-local services values"; \
					services_apply=1; \
					;; \
			esac; \
		fi; \
		$(MAKE) --no-print-directory DEV_DEPLOY_HELM="$$helm_mode" DEV_DEPLOY_SKIP_PUBLIC_RESTART=1 DEV_DEPLOY_DATA_MODE="$$data_mode" dev-web-deploy; \
		if [ "$(SERVICES_AUTO_BUILD_IMAGE)" = "1" ]; then \
			$(MAKE) --no-print-directory services-image-local-up; \
		fi; \
		if [ "$$services_apply" = "1" ]; then \
			echo "applying services Helm release"; \
			if [ "$$data_mode" = "local-edge" ]; then \
				$(MAKE) --no-print-directory SERVICES_AUTO_BUILD_IMAGE=0 services-up-cloud-db; \
			else \
				$(MAKE) --no-print-directory SERVICES_AUTO_BUILD_IMAGE=0 services-up; \
			fi; \
		else \
			echo "skipping services Helm apply (set DEV_DEPLOY_HELM=always to force)"; \
		fi; \
		if [ "$$data_mode" = "local-edge" ]; then \
			echo "skipping local database migrations in local-edge mode; API pods use the cloud DB forward"; \
		else \
			echo "applying local database migrations before service restarts"; \
			$(MAKE) --no-print-directory db-migrate-up-local; \
		fi
	@set -euo pipefail; \
		restart_and_wait_if_exists() { \
			ns="$$1"; \
			name="$$2"; \
			if $(LOCAL_KUBECTL) -n "$$ns" get deploy/"$$name" >/dev/null 2>&1; then \
				echo "restarting $$ns/$$name"; \
				$(LOCAL_KUBECTL) -n "$$ns" rollout restart deploy/"$$name"; \
				echo "waiting for $$ns/$$name"; \
				$(LOCAL_ROLLOUT_STATUS) "$$ns" "$$name" 300s; \
			else \
				echo "skipping missing deployment $$ns/$$name"; \
			fi; \
		}; \
		recreate_and_wait_if_exists() { \
			ns="$$1"; \
			name="$$2"; \
			if $(LOCAL_KUBECTL) -n "$$ns" get deploy/"$$name" >/dev/null 2>&1; then \
				replicas="$$( $(LOCAL_KUBECTL) -n "$$ns" get deploy/"$$name" -o jsonpath='{.spec.replicas}' )"; \
				if [ -z "$$replicas" ]; then \
					replicas=1; \
				fi; \
				echo "recycling $$ns/$$name"; \
				$(LOCAL_KUBECTL) -n "$$ns" scale deploy/"$$name" --replicas=0; \
				$(LOCAL_ROLLOUT_STATUS_STRICT) "$$ns" "$$name" 180s; \
				echo "restoring $$ns/$$name replicas=$$replicas"; \
				$(LOCAL_KUBECTL) -n "$$ns" scale deploy/"$$name" --replicas="$$replicas"; \
				echo "waiting for $$ns/$$name"; \
				$(LOCAL_ROLLOUT_STATUS) "$$ns" "$$name" 300s; \
			else \
				echo "skipping missing deployment $$ns/$$name"; \
			fi; \
		}; \
		echo "restarting updated local deployments in dependency order"; \
		echo "phase: verification"; \
		recreate_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-solar-verification; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-scheduler; \
		echo "phase: ingest"; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-ingest; \
		echo "phase: transform"; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-projection; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-rollup; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-archive; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-inference; \
		echo "phase: go services"; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-grpc-api; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-energy-api; \
		echo "phase: rest services"; \
		restart_and_wait_if_exists $(PLATFORM_NAMESPACE) pulse-platform-realtime-gateway; \
		echo "phase: frontend"; \
		restart_and_wait_if_exists $(PLATFORM_NAMESPACE) pulse-platform-public-app
	@echo "showing deployment state and recent realtime gateway logs"
	$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy
	$(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy
	$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) logs deploy/pulse-platform-realtime-gateway --since=5m

dev-archive-audit:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	db_log=/tmp/pulse-archive-audit-db-portforward.log; \
	minio_log=/tmp/pulse-archive-audit-minio-portforward.log; \
	echo "starting postgres port-forward on 127.0.0.1:$(REGEN_DB_LOCAL_PORT) (log: $$db_log)"; \
	$(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) port-forward svc/$(DB_MIGRATION_CLUSTER)-rw $(REGEN_DB_LOCAL_PORT):5432 >$$db_log 2>&1 & \
	db_pid=$$!; \
	echo "starting minio port-forward on 127.0.0.1:$(REGEN_MINIO_LOCAL_PORT) (log: $$minio_log)"; \
	$(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) port-forward svc/$(ARCHIVE_INTEGRATION_SERVICE) $(REGEN_MINIO_LOCAL_PORT):9000 >$$minio_log 2>&1 & \
	minio_pid=$$!; \
	cleanup() { \
		kill $$db_pid >/dev/null 2>&1 || true; \
		kill $$minio_pid >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	sleep 2; \
	if [ -n "$(REGEN_FROM)" ]; then \
		from="$(REGEN_FROM)"; \
	else \
		from="$$(date -u -v-48H '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '-48 hours' '+%Y-%m-%dT%H:%M:%SZ')"; \
	fi; \
	if [ -n "$(REGEN_TO)" ]; then \
		to="$(REGEN_TO)"; \
	else \
		to="$$(date -u '+%Y-%m-%dT%H:%M:%SZ')"; \
	fi; \
	db_user="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.username}' | base64 -d )"; \
	db_pass="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.password}' | base64 -d )"; \
	access_key="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(ARCHIVE_INTEGRATION_SECRET) -o jsonpath='{.data.rootUser}' | base64 -d )"; \
	secret_key="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(ARCHIVE_INTEGRATION_SECRET) -o jsonpath='{.data.rootPassword}' | base64 -d )"; \
	echo "auditing MinIO raw archive vs archive_object_manifest from $$from to $$to"; \
	CONTROL_PLANE_DB_DSN="postgresql://$$db_user:$$db_pass@127.0.0.1:$(REGEN_DB_LOCAL_PORT)/$(DB_MIGRATION_DB)" \
	ARCHIVE_OBJECT_ENDPOINT="127.0.0.1:$(REGEN_MINIO_LOCAL_PORT)" \
	ARCHIVE_OBJECT_ACCESS_KEY="$$access_key" \
	ARCHIVE_OBJECT_SECRET_KEY="$$secret_key" \
	ARCHIVE_OBJECT_REGION="us-east-1" \
	ARCHIVE_OBJECT_SECURE=false \
	$(GO) run ./cmd/ecoflow-archive-audit -archive-bucket pulse-telemetry-raw -archive-prefix raw -from "$$from" -to "$$to" -max-objects $(REGEN_MAX_OBJECTS)

dev-archive-reconcile:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	db_log=/tmp/pulse-archive-reconcile-db-portforward.log; \
	minio_log=/tmp/pulse-archive-reconcile-minio-portforward.log; \
	echo "starting postgres port-forward on 127.0.0.1:$(REGEN_DB_LOCAL_PORT) (log: $$db_log)"; \
	$(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) port-forward svc/$(DB_MIGRATION_CLUSTER)-rw $(REGEN_DB_LOCAL_PORT):5432 >$$db_log 2>&1 & \
	db_pid=$$!; \
	echo "starting minio port-forward on 127.0.0.1:$(REGEN_MINIO_LOCAL_PORT) (log: $$minio_log)"; \
	$(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) port-forward svc/$(ARCHIVE_INTEGRATION_SERVICE) $(REGEN_MINIO_LOCAL_PORT):9000 >$$minio_log 2>&1 & \
	minio_pid=$$!; \
	cleanup() { \
		kill $$db_pid >/dev/null 2>&1 || true; \
		kill $$minio_pid >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	sleep 2; \
	if [ -n "$(REGEN_FROM)" ]; then \
		from="$(REGEN_FROM)"; \
	else \
		from="$$(date -u -v-48H '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '-48 hours' '+%Y-%m-%dT%H:%M:%SZ')"; \
	fi; \
	if [ -n "$(REGEN_TO)" ]; then \
		to="$(REGEN_TO)"; \
	else \
		to="$$(date -u '+%Y-%m-%dT%H:%M:%SZ')"; \
	fi; \
	db_user="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.username}' | base64 -d )"; \
	db_pass="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.password}' | base64 -d )"; \
	access_key="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(ARCHIVE_INTEGRATION_SECRET) -o jsonpath='{.data.rootUser}' | base64 -d )"; \
	secret_key="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(ARCHIVE_INTEGRATION_SECRET) -o jsonpath='{.data.rootPassword}' | base64 -d )"; \
	echo "reconciling archive_object_manifest against MinIO raw archive from $$from to $$to"; \
	CONTROL_PLANE_DB_DSN="postgresql://$$db_user:$$db_pass@127.0.0.1:$(REGEN_DB_LOCAL_PORT)/$(DB_MIGRATION_DB)" \
	ARCHIVE_OBJECT_ENDPOINT="127.0.0.1:$(REGEN_MINIO_LOCAL_PORT)" \
	ARCHIVE_OBJECT_ACCESS_KEY="$$access_key" \
	ARCHIVE_OBJECT_SECRET_KEY="$$secret_key" \
	ARCHIVE_OBJECT_REGION="us-east-1" \
	ARCHIVE_OBJECT_SECURE=false \
	$(GO) run ./cmd/ecoflow-archive-reconcile -apply -archive-bucket pulse-telemetry-raw -archive-prefix raw -from "$$from" -to "$$to" -max-objects $(REGEN_MAX_OBJECTS)

dev-regen-data:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	db_log=/tmp/pulse-regen-db-portforward.log; \
	nats_log=/tmp/pulse-regen-nats-portforward.log; \
	minio_log=/tmp/pulse-regen-minio-portforward.log; \
	echo "starting postgres port-forward on 127.0.0.1:$(REGEN_DB_LOCAL_PORT) (log: $$db_log)"; \
	$(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) port-forward svc/$(DB_MIGRATION_CLUSTER)-rw $(REGEN_DB_LOCAL_PORT):5432 >$$db_log 2>&1 & \
	db_pid=$$!; \
	echo "starting minio port-forward on 127.0.0.1:$(REGEN_MINIO_LOCAL_PORT) (log: $$minio_log)"; \
	$(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) port-forward svc/$(ARCHIVE_INTEGRATION_SERVICE) $(REGEN_MINIO_LOCAL_PORT):9000 >$$minio_log 2>&1 & \
	minio_pid=$$!; \
	cleanup() { \
		kill $$db_pid >/dev/null 2>&1 || true; \
		kill $$minio_pid >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	sleep 2; \
	if [ -n "$(REGEN_FROM)" ]; then \
		from="$(REGEN_FROM)"; \
	else \
		from="$$(date -u -v-48H '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '-48 hours' '+%Y-%m-%dT%H:%M:%SZ')"; \
	fi; \
	if [ -n "$(REGEN_TO)" ]; then \
		to="$(REGEN_TO)"; \
	else \
		to="$$(date -u '+%Y-%m-%dT%H:%M:%SZ')"; \
	fi; \
	replay_started_at="$$(date -u '+%Y-%m-%dT%H:%M:%SZ')"; \
	db_user="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.username}' | base64 -d )"; \
	db_pass="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.password}' | base64 -d )"; \
	access_key="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(ARCHIVE_INTEGRATION_SECRET) -o jsonpath='{.data.rootUser}' | base64 -d )"; \
	secret_key="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get secret $(ARCHIVE_INTEGRATION_SECRET) -o jsonpath='{.data.rootPassword}' | base64 -d )"; \
	primary="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) get pods -l cnpg.io/cluster=$(DB_MIGRATION_CLUSTER),cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}' )"; \
	if [ -z "$$primary" ]; then \
		echo "no CNPG primary pod found for cluster=$(DB_MIGRATION_CLUSTER) in namespace=$(PLATFORM_NAMESPACE)"; \
		exit 1; \
	fi; \
	echo "rebuilding archive-backed rollups safely for all devices from $$from to $$to"; \
	device_args=""; \
	if [ -n "$(REGEN_PROVIDER)" ]; then device_args="$$device_args -provider '$(REGEN_PROVIDER)'"; fi; \
	if [ -n "$(REGEN_DEVICE_IDS)" ]; then device_args="$$device_args -device-ids '$(REGEN_DEVICE_IDS)'"; fi; \
	if [ -n "$(REGEN_PROVIDER_DEVICE_IDS)" ]; then device_args="$$device_args -provider-device-ids '$(REGEN_PROVIDER_DEVICE_IDS)'"; fi; \
	if [ -n "$(REGEN_PARALLELISM)" ]; then device_args="$$device_args -parallelism $(REGEN_PARALLELISM)"; fi; \
	CONTROL_PLANE_DB_DSN="postgresql://$$db_user:$$db_pass@127.0.0.1:$(REGEN_DB_LOCAL_PORT)/$(DB_MIGRATION_DB)" \
	ARCHIVE_OBJECT_ENDPOINT="127.0.0.1:$(REGEN_MINIO_LOCAL_PORT)" \
	ARCHIVE_OBJECT_ACCESS_KEY="$$access_key" \
	ARCHIVE_OBJECT_SECRET_KEY="$$secret_key" \
	ARCHIVE_OBJECT_REGION="us-east-1" \
	ARCHIVE_OBJECT_SECURE=false \
	sh -c "$(GO) run ./cmd/ecoflow-rollup-rebuild -direct-archive -archive-bucket pulse-telemetry-raw -archive-prefix raw -from '$$from' -to '$$to' -max-objects $(REGEN_MAX_OBJECTS) $$device_args"; \
	proof_sql="WITH bounds AS (SELECT '$$from'::timestamptz AS current_from, '$$to'::timestamptz AS current_to, ('$$to'::timestamptz - '$$from'::timestamptz) AS window_size), current_rows AS (SELECT provider_device_id, bucket_start, updated_at, COALESCE(solar_generated_wh, CASE WHEN COALESCE(pv_avg_w, 0) > 0 THEN pv_avg_w / 60.0 ELSE 0 END) AS derived_solar_generated_wh FROM telemetry_rollup_minute, bounds WHERE bucket_start >= bounds.current_from AND bucket_start < bounds.current_to), previous_rows AS (SELECT provider_device_id, COALESCE(solar_generated_wh, CASE WHEN COALESCE(pv_avg_w, 0) > 0 THEN pv_avg_w / 60.0 ELSE 0 END) AS derived_solar_generated_wh FROM telemetry_rollup_minute, bounds WHERE bucket_start >= bounds.current_from - bounds.window_size AND bucket_start < bounds.current_from), devices AS (SELECT provider_device_id FROM current_rows UNION SELECT provider_device_id FROM previous_rows) SELECT devices.provider_device_id || '|' || COALESCE(curr.touched_buckets, 0) || '|' || COALESCE(curr.total_buckets, 0) || '|' || COALESCE(curr.latest_bucket_utc, 'n/a') || '|' || ROUND(COALESCE(curr.current_wh, 0)::numeric, 2) || '|' || ROUND(COALESCE(prev.previous_wh, 0)::numeric, 2) || '|' || CASE WHEN COALESCE(prev.previous_wh, 0) > 0 THEN ROUND((((COALESCE(curr.current_wh, 0) - prev.previous_wh) / prev.previous_wh) * 100)::numeric, 2)::text ELSE 'n/a' END FROM devices LEFT JOIN (SELECT provider_device_id, COUNT(*) FILTER (WHERE updated_at >= '$$replay_started_at'::timestamptz) AS touched_buckets, COUNT(*) AS total_buckets, COALESCE(to_char(MAX(bucket_start) AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"'), 'n/a') AS latest_bucket_utc, SUM(derived_solar_generated_wh) AS current_wh FROM current_rows GROUP BY provider_device_id) curr ON curr.provider_device_id = devices.provider_device_id LEFT JOIN (SELECT provider_device_id, SUM(derived_solar_generated_wh) AS previous_wh FROM previous_rows GROUP BY provider_device_id) prev ON prev.provider_device_id = devices.provider_device_id ORDER BY devices.provider_device_id;"; \
	for attempt in $$(seq 1 30); do \
		proof_rows="$$( $(KUBECTL) --context "$$ctx" -n $(PLATFORM_NAMESPACE) exec "$$primary" -- env PGPASSWORD="$$db_pass" psql -h "$(DB_MIGRATION_CLUSTER)-rw" -U "$$db_user" -d "$(DB_MIGRATION_DB)" -v ON_ERROR_STOP=1 -Atc "$$proof_sql" )"; \
		touched="$$( printf '%s\n' "$$proof_rows" | awk -F'|' 'NF >= 5 { sum += $$2 } END { print sum + 0 }' )"; \
		if [ "$$touched" -gt 0 ]; then \
			echo "replay proof (provider_device_id|touched_buckets|total_buckets|latest_bucket_utc|current_window_derived_solar_generated_wh|previous_window_derived_solar_generated_wh|delta_pct)"; \
			printf '%s\n' "$$proof_rows"; \
			break; \
		fi; \
		if [ $$attempt -eq 30 ]; then \
			echo "no rollup buckets updated after replay window $$from -> $$to"; \
			echo "proof query returned:"; \
			printf '%s\n' "$$proof_rows"; \
			exit 1; \
		fi; \
		sleep 2; \
	done

db-migrate-up-local:
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	if ! find $(DB_MIGRATIONS_DIR) -maxdepth 1 -type f -name '*.up.sql' | grep -q .; then \
		echo "no .up.sql files found in $(DB_MIGRATIONS_DIR)"; \
		exit 1; \
	fi; \
	primary="$$(kubectl --context "$$ctx" -n "$$ns" get pods -l cnpg.io/cluster="$$cluster",cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')"; \
	if [ -z "$$primary" ]; then \
		echo "no CNPG primary pod found for cluster=$$cluster in namespace=$$ns"; \
		exit 1; \
	fi; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	db_log="$$(mktemp)"; \
	cleanup() { \
		rc="$$?"; \
		if [ -n "$${pf_pid:-}" ] && kill -0 "$$pf_pid" 2>/dev/null; then kill "$$pf_pid" 2>/dev/null || true; fi; \
		rm -f "$$db_log"; \
		exit "$$rc"; \
	}; \
	trap cleanup EXIT INT TERM; \
	echo "starting postgres port-forward on 127.0.0.1:$(DB_MIGRATION_LOCAL_PORT) (log: $$db_log)"; \
	$(KUBECTL) --context "$$ctx" -n "$$ns" port-forward svc/"$$cluster-rw" $(DB_MIGRATION_LOCAL_PORT):5432 >"$$db_log" 2>&1 & \
	pf_pid="$$!"; \
	for attempt in $$(seq 1 40); do \
		if nc -z 127.0.0.1 $(DB_MIGRATION_LOCAL_PORT) >/dev/null 2>&1; then break; fi; \
		sleep 1; \
	done; \
	if ! nc -z 127.0.0.1 $(DB_MIGRATION_LOCAL_PORT) >/dev/null 2>&1; then \
		echo "postgres port-forward failed"; \
		cat "$$db_log"; \
		exit 1; \
	fi; \
	echo "running idempotent migration runner against $$cluster-rw (db=$$db user=$$user)"; \
	CONTROL_PLANE_DB_DSN="postgresql://$$user:$$pass@127.0.0.1:$(DB_MIGRATION_LOCAL_PORT)/$$db?sslmode=disable" \
	DB_MIGRATIONS_DIR="$(DB_MIGRATIONS_DIR)" \
	DB_MIGRATION_ENVIRONMENT=local \
	DB_MIGRATION_REQUIRE_BACKUP=false \
	DB_MIGRATION_FORWARD_ONLY=true \
	$(GO) run ./cmd/ecoflow-db-migrate-job

db-migrate-down-local:
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	files="$$(find $(DB_MIGRATIONS_DIR) -maxdepth 1 -type f -name '*.down.sql' | sort -r)"; \
	if [ -z "$$files" ]; then \
		echo "no .down.sql files found in $(DB_MIGRATIONS_DIR)"; \
		exit 1; \
	fi; \
	primary="$$(kubectl --context "$$ctx" -n "$$ns" get pods -l cnpg.io/cluster="$$cluster",cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')"; \
	if [ -z "$$primary" ]; then \
		echo "no CNPG primary pod found for cluster=$$cluster in namespace=$$ns"; \
		exit 1; \
	fi; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	while IFS= read -r f; do \
		[ -n "$$f" ] || continue; \
		echo "reverting $$f"; \
		cat "$$f" | kubectl --context "$$ctx" -n "$$ns" exec -i "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -f -; \
	done <<< "$$files"

db-migrate-verify-local:
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	primary="$$(kubectl --context "$$ctx" -n "$$ns" get pods -l cnpg.io/cluster="$$cluster",cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')"; \
	if [ -z "$$primary" ]; then \
		echo "no CNPG primary pod found for cluster=$$cluster in namespace=$$ns"; \
		exit 1; \
	fi; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	echo "verifying control-plane schema on $$cluster-rw (db=$$db user=$$user)"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('users','devices','user_devices','provider_credentials','provider_devices','archive_object_manifest','telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day') ORDER BY table_name;"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef d JOIN pg_class c ON c.oid=d.adrelid JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=d.adnum WHERE c.relname='users' AND a.attname='id';"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "SELECT hypertable_name FROM timescaledb_information.hypertables WHERE hypertable_schema='public' AND hypertable_name IN ('telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day') ORDER BY hypertable_name;"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "SELECT hypertable_name || '|' || (config->>'drop_after') || '|' || schedule_interval::text FROM timescaledb_information.jobs WHERE proc_name='policy_retention' AND hypertable_schema='public' AND hypertable_name IN ('telemetry_rollup_minute','telemetry_rollup_hour','telemetry_rollup_day') ORDER BY hypertable_name;"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "SELECT a.attname, pg_get_expr(d.adbin,d.adrelid) FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum JOIN pg_class c ON c.oid=a.attrelid WHERE c.relname='users' AND a.attname IN ('created_at','updated_at') ORDER BY a.attname;"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "SELECT conname FROM pg_constraint WHERE conname IN ('chk_user_devices_role','chk_devices_ecoflow_sn_nonempty','chk_users_keycloak_subject_nonempty','uq_archive_manifest_bucket_key','chk_archive_manifest_ts_order','pk_rollup_minute','pk_rollup_hour','pk_rollup_day') ORDER BY conname;"

db-migrate-cycle-local:
	@$(MAKE) db-migrate-up-local
	@$(MAKE) db-migrate-verify-local
	@$(MAKE) db-migrate-down-local
	@$(MAKE) db-migrate-up-local
	@$(MAKE) db-migrate-verify-local

db-migrate-e2e-local: db-migrate-up-local
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	primary="$$(kubectl --context "$$ctx" -n "$$ns" get pods -l cnpg.io/cluster="$$cluster",cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')"; \
	if [ -z "$$primary" ]; then \
		echo "no CNPG primary pod found for cluster=$$cluster in namespace=$$ns"; \
		exit 1; \
	fi; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	echo "running migration e2e checks on $$cluster-rw (db=$$db user=$$user)"; \
	sql="TRUNCATE archive_object_manifest, user_devices, provider_devices, provider_credentials, users, devices RESTART IDENTITY CASCADE; WITH u AS (INSERT INTO users (keycloak_subject, email, display_name, created_at, updated_at) VALUES ('kc-sub-e2e-1','e2e1@example.com','E2E User 1', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC') RETURNING id), d AS (INSERT INTO devices (ecoflow_sn, product_name, model, created_at, updated_at) VALUES ('SN-E2E-0001','DELTA Pro Ultra','dpu', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC') RETURNING id) INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at) SELECT u.id, d.id, 'viewer', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC' FROM u CROSS JOIN d; SELECT u.keycloak_subject, d.ecoflow_sn, ud.role FROM users u JOIN user_devices ud ON ud.user_id = u.id JOIN devices d ON d.id = ud.device_id WHERE u.keycloak_subject = 'kc-sub-e2e-1';"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "$$sql"; \
	set +e; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "INSERT INTO users (keycloak_subject, created_at, updated_at) VALUES ('kc-sub-e2e-1', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC');" >/tmp/m1_dup_user.out 2>&1; \
	rc_user=$$?; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "INSERT INTO devices (ecoflow_sn, created_at, updated_at) VALUES ('SN-E2E-0001', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC');" >/tmp/m1_dup_sn.out 2>&1; \
	rc_sn=$$?; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at) SELECT user_id, device_id, 'owner', NOW() AT TIME ZONE 'UTC', NOW() AT TIME ZONE 'UTC' FROM user_devices LIMIT 1;" >/tmp/m1_bad_role.out 2>&1; \
	rc_role=$$?; \
	set -e; \
	if [ $$rc_user -eq 0 ] || [ $$rc_sn -eq 0 ] || [ $$rc_role -eq 0 ]; then \
		echo "e2e constraint check failed: expected uniqueness/role failures"; \
		echo "rc_user=$$rc_user rc_sn=$$rc_sn rc_role=$$rc_role"; \
		exit 1; \
	fi; \
	echo "e2e checks passed: ownership join + uniqueness + role guard"

db-seed-dev-local: db-migrate-up-local
	@set -euo pipefail; \
	incoming_provider="$${ECOFLOW_DEV_PROVIDER:-}"; \
	incoming_access_key="$${ECOFLOW_DEV_ACCESS_KEY:-}"; \
	incoming_secret_key="$${ECOFLOW_DEV_SECRET_KEY:-}"; \
	incoming_user_subject="$${ECOFLOW_DEV_USER_SUBJECT:-}"; \
	incoming_user_email="$${ECOFLOW_DEV_USER_EMAIL:-}"; \
	incoming_seed_sns="$${ECOFLOW_DEV_SEED_SNS:-}"; \
	if [ -f .env ]; then \
		set -a; source ./.env; set +a; \
	fi; \
	if [ -n "$$incoming_provider" ]; then export ECOFLOW_DEV_PROVIDER="$$incoming_provider"; fi; \
	if [ -n "$$incoming_access_key" ]; then export ECOFLOW_DEV_ACCESS_KEY="$$incoming_access_key"; fi; \
	if [ -n "$$incoming_secret_key" ]; then export ECOFLOW_DEV_SECRET_KEY="$$incoming_secret_key"; fi; \
	if [ -n "$$incoming_user_subject" ]; then export ECOFLOW_DEV_USER_SUBJECT="$$incoming_user_subject"; fi; \
	if [ -n "$$incoming_user_email" ]; then export ECOFLOW_DEV_USER_EMAIL="$$incoming_user_email"; fi; \
	if [ -n "$$incoming_seed_sns" ]; then export ECOFLOW_DEV_SEED_SNS="$$incoming_seed_sns"; fi; \
	seed_user_subject="$${DB_SEED_USER_SUBJECT:-$${ECOFLOW_DEV_USER_SUBJECT:-$${PULSE_PLATFORM_DEV_SUBJECT:-$(DB_SEED_USER_SUBJECT)}}}"; \
	seed_user_email="$${DB_SEED_USER_EMAIL:-$${ECOFLOW_DEV_USER_EMAIL:-$$seed_user_subject}}"; \
	seed_serials="$${DB_SEED_SERIALS:-$${ECOFLOW_DEV_SEED_SNS:-$(DB_SEED_SERIALS)}}"; \
	if [ -z "$${ECOFLOW_DEV_ACCESS_KEY:-}" ]; then \
		echo "ECOFLOW_DEV_ACCESS_KEY is required (export it or set it in .env)"; \
		exit 1; \
	fi; \
	if [ -z "$${ECOFLOW_DEV_SECRET_KEY:-}" ]; then \
		echo "ECOFLOW_DEV_SECRET_KEY is required (export it or set it in .env)"; \
		exit 1; \
	fi; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	port="$(DB_SEED_LOCAL_PORT)"; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	pf_log="$$(mktemp -t ecoflow-dev-seed-port-forward.XXXXXX.log)"; \
	$(LOCAL_KUBECTL) -n "$$ns" port-forward "svc/$$cluster-rw" "$$port:5432" >"$$pf_log" 2>&1 & \
	pf_pid=$$!; \
	cleanup() { \
		kill "$$pf_pid" >/dev/null 2>&1 || true; \
		wait "$$pf_pid" >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	ready=0; \
	for _ in {1..30}; do \
		if grep -q "Forwarding from" "$$pf_log" 2>/dev/null; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -ne 1 ]; then \
		echo "port-forward did not become ready; see $$pf_log"; \
		exit 1; \
	fi; \
	CONTROL_PLANE_DB_DSN="host=127.0.0.1 port=$$port user=$$user password=$$pass dbname=$$db sslmode=disable" \
	ECOFLOW_DEV_USER_SUBJECT="$$seed_user_subject" \
	ECOFLOW_DEV_USER_EMAIL="$$seed_user_email" \
	ECOFLOW_DEV_SEED_SNS="$$seed_serials" \
	$(GO) run ./cmd/ecoflow-dev-seed

_pgroll-local:
	@set -euo pipefail; \
	action="$(PGROLL_ACTION)"; \
	if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi; \
	if ! command -v $(PGROLL) >/dev/null 2>&1; then \
		echo "$(PGROLL) not found. Install pgroll first."; \
		exit 1; \
	fi; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	port="$(PGROLL_LOCAL_PORT)"; \
	log_file="$$(mktemp -t pulse-pgroll-port-forward.XXXXXX.log)"; \
	$(LOCAL_KUBECTL) -n "$$ns" port-forward "svc/$$cluster-rw" "$$port:5432" >"$$log_file" 2>&1 & \
	pf_pid=$$!; \
	cleanup() { \
		kill "$$pf_pid" >/dev/null 2>&1 || true; \
		wait "$$pf_pid" >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	ready=0; \
	for _ in {1..30}; do \
		if grep -q "Forwarding from" "$$log_file" 2>/dev/null; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -ne 1 ]; then \
		echo "port-forward did not become ready; see $$log_file"; \
		exit 1; \
	fi; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	url="postgres://$$user:$$pass@127.0.0.1:$$port/$$db?sslmode=disable"; \
	case "$$action" in \
		init) \
			$(PGROLL) init --postgres-url "$$url"; \
			;; \
		status) \
			$(PGROLL) --postgres-url "$$url" status; \
			;; \
		start) \
			if [ -z "$(PGROLL_PLAN)" ]; then \
				echo "PGROLL_PLAN is required for pgroll-start-local"; \
				exit 1; \
			fi; \
			$(PGROLL) --postgres-url "$$url" start "$(PGROLL_PLAN)"; \
			;; \
		complete) \
			$(PGROLL) --postgres-url "$$url" complete; \
			;; \
		rollback) \
			$(PGROLL) --postgres-url "$$url" rollback; \
			;; \
		*) \
			echo "unsupported PGROLL_ACTION=$$action"; \
			exit 1; \
			;; \
	esac

pgroll-init-local:
	@$(MAKE) --no-print-directory PGROLL_ACTION=init _pgroll-local

pgroll-status-local:
	@$(MAKE) --no-print-directory PGROLL_ACTION=status _pgroll-local

pgroll-start-local:
	@$(MAKE) --no-print-directory PGROLL_ACTION=start _pgroll-local

pgroll-complete-local:
	@$(MAKE) --no-print-directory PGROLL_ACTION=complete _pgroll-local

pgroll-rollback-local:
	@$(MAKE) --no-print-directory PGROLL_ACTION=rollback _pgroll-local

dr-backup-local:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@if ! command -v $(DOCKER) >/dev/null 2>&1; then \
		echo "$(DOCKER) not found. Install Docker first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	archive_ns="$(ARCHIVE_INTEGRATION_NAMESPACE)"; \
	archive_secret="$(ARCHIVE_INTEGRATION_SECRET)"; \
	backup_dir="$(DR_BACKUP_DIR)"; \
	report_file="$(DR_REPORT_FILE)"; \
	bucket="$(DR_ARCHIVE_BUCKET)"; \
	primary="$$(kubectl --context "$$ctx" -n "$$ns" get pods -l cnpg.io/cluster="$$cluster",cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')"; \
	if [ -z "$$primary" ]; then \
		echo "no CNPG primary pod found for cluster=$$cluster in namespace=$$ns"; \
		exit 1; \
	fi; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	root_user="$$(kubectl --context "$$ctx" -n "$$archive_ns" get secret "$$archive_secret" -o jsonpath='{.data.rootUser}' | base64 -d)"; \
	root_pass="$$(kubectl --context "$$ctx" -n "$$archive_ns" get secret "$$archive_secret" -o jsonpath='{.data.rootPassword}' | base64 -d)"; \
	mkdir -p "$$backup_dir/minio"; \
	db_dump="$$backup_dir/postgres.data.sql"; \
	echo "writing Postgres backup to $$db_dump"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" pg_dump -h "$$cluster-rw" -U "$$user" -d "$$db" \
		--data-only --column-inserts \
		--table=users \
		--table=devices \
		--table=user_devices \
		--table=provider_credentials \
		--table=provider_devices \
		--table=archive_object_manifest > "$$db_dump"; \
	pf_log="$$(mktemp -t ecoflow-dr-minio-backup-port-forward.XXXXXX.log)"; \
	kubectl --context "$$ctx" -n "$$archive_ns" port-forward "svc/$(ARCHIVE_INTEGRATION_SERVICE)" "$(DR_MINIO_LOCAL_PORT):9000" >"$$pf_log" 2>&1 & \
	pf_pid=$$!; \
	cleanup() { \
		kill "$$pf_pid" >/dev/null 2>&1 || true; \
		wait "$$pf_pid" >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	ready=0; \
	for _ in {1..30}; do \
		if grep -q "Forwarding from" "$$pf_log" 2>/dev/null; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -ne 1 ]; then \
		echo "minio port-forward did not become ready; see $$pf_log"; \
		exit 1; \
	fi; \
	echo "syncing MinIO bucket $$bucket to $$backup_dir/minio/$$bucket"; \
	object_count="$$( $(DOCKER) run --rm \
		--entrypoint /bin/sh \
		-e DR_DOCKER_ENDPOINT="$(DR_MINIO_DOCKER_ENDPOINT)" \
		-e DR_ROOT_USER="$$root_user" \
		-e DR_ROOT_PASS="$$root_pass" \
		-e DR_BUCKET="$$bucket" \
		-v "$$backup_dir/minio:/backup" \
		"$(DR_MINIO_MC_IMAGE)" \
		-c 'set -e; mc alias set local "http://$$DR_DOCKER_ENDPOINT" "$$DR_ROOT_USER" "$$DR_ROOT_PASS" >/dev/null; mc mb --ignore-existing "local/$$DR_BUCKET" >/dev/null; mkdir -p "/backup/$$DR_BUCKET"; mc mirror --overwrite "local/$$DR_BUCKET" "/backup/$$DR_BUCKET" >/dev/null; mc ls --recursive "/backup/$$DR_BUCKET" | wc -l | tr -d " "' )"; \
	counts="$$(kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "SELECT (SELECT COUNT(*) FROM users)::text || '|' || (SELECT COUNT(*) FROM devices)::text || '|' || (SELECT COUNT(*) FROM user_devices)::text || '|' || (SELECT COUNT(*) FROM provider_credentials)::text || '|' || (SELECT COUNT(*) FROM provider_devices)::text || '|' || (SELECT COUNT(*) FROM archive_object_manifest)::text;")"; \
	IFS='|' read -r users_count devices_count user_devices_count provider_credentials_count provider_devices_count archive_manifest_count <<< "$$counts"; \
	{ \
		printf "backup_created_at_utc=%s\n" "$$(date -u '+%Y-%m-%dT%H:%M:%SZ')"; \
		printf "backup_name=%s\n" "$(DR_BACKUP_NAME)"; \
		printf "archive_bucket=%s\n" "$$bucket"; \
		printf "users_count=%s\n" "$$users_count"; \
		printf "devices_count=%s\n" "$$devices_count"; \
		printf "user_devices_count=%s\n" "$$user_devices_count"; \
		printf "provider_credentials_count=%s\n" "$$provider_credentials_count"; \
		printf "provider_devices_count=%s\n" "$$provider_devices_count"; \
		printf "archive_manifest_count=%s\n" "$$archive_manifest_count"; \
		printf "archive_object_count=%s\n" "$$object_count"; \
	} > "$$report_file"; \
	echo "backup report written to $$report_file"

dr-restore-local:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@if ! command -v $(DOCKER) >/dev/null 2>&1; then \
		echo "$(DOCKER) not found. Install Docker first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	archive_ns="$(ARCHIVE_INTEGRATION_NAMESPACE)"; \
	archive_secret="$(ARCHIVE_INTEGRATION_SECRET)"; \
	backup_dir="$(DR_BACKUP_DIR)"; \
	report_file="$(DR_REPORT_FILE)"; \
	if [ ! -f "$$report_file" ]; then \
		echo "backup report not found at $$report_file"; \
		exit 1; \
	fi; \
	source "$$report_file"; \
	bucket="$${archive_bucket:-$(DR_ARCHIVE_BUCKET)}"; \
	db_dump="$$backup_dir/postgres.data.sql"; \
	if [ ! -f "$$db_dump" ]; then \
		echo "postgres backup file not found at $$db_dump"; \
		exit 1; \
	fi; \
	if [ ! -d "$$backup_dir/minio/$$bucket" ]; then \
		echo "minio backup directory not found at $$backup_dir/minio/$$bucket"; \
		exit 1; \
	fi; \
	primary="$$(kubectl --context "$$ctx" -n "$$ns" get pods -l cnpg.io/cluster="$$cluster",cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')"; \
	if [ -z "$$primary" ]; then \
		echo "no CNPG primary pod found for cluster=$$cluster in namespace=$$ns"; \
		exit 1; \
	fi; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	echo "restoring Postgres from $$db_dump"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "TRUNCATE archive_object_manifest, user_devices, provider_devices, provider_credentials, users, devices RESTART IDENTITY CASCADE;"; \
	cat "$$db_dump" | kubectl --context "$$ctx" -n "$$ns" exec -i "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -f -; \
	root_user="$$(kubectl --context "$$ctx" -n "$$archive_ns" get secret "$$archive_secret" -o jsonpath='{.data.rootUser}' | base64 -d)"; \
	root_pass="$$(kubectl --context "$$ctx" -n "$$archive_ns" get secret "$$archive_secret" -o jsonpath='{.data.rootPassword}' | base64 -d)"; \
	pf_log="$$(mktemp -t ecoflow-dr-minio-restore-port-forward.XXXXXX.log)"; \
	kubectl --context "$$ctx" -n "$$archive_ns" port-forward "svc/$(ARCHIVE_INTEGRATION_SERVICE)" "$(DR_MINIO_LOCAL_PORT):9000" >"$$pf_log" 2>&1 & \
	pf_pid=$$!; \
	cleanup() { \
		kill "$$pf_pid" >/dev/null 2>&1 || true; \
		wait "$$pf_pid" >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	ready=0; \
	for _ in {1..30}; do \
		if grep -q "Forwarding from" "$$pf_log" 2>/dev/null; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -ne 1 ]; then \
		echo "minio port-forward did not become ready; see $$pf_log"; \
		exit 1; \
	fi; \
	echo "restoring MinIO bucket $$bucket from $$backup_dir/minio/$$bucket"; \
	$(DOCKER) run --rm \
		--entrypoint /bin/sh \
		-e DR_DOCKER_ENDPOINT="$(DR_MINIO_DOCKER_ENDPOINT)" \
		-e DR_ROOT_USER="$$root_user" \
		-e DR_ROOT_PASS="$$root_pass" \
		-e DR_BUCKET="$$bucket" \
		-v "$$backup_dir/minio:/backup" \
		"$(DR_MINIO_MC_IMAGE)" \
		-c 'set -e; if [ ! -d "/backup/$$DR_BUCKET" ]; then echo "missing backup directory: /backup/$$DR_BUCKET"; exit 1; fi; mc alias set local "http://$$DR_DOCKER_ENDPOINT" "$$DR_ROOT_USER" "$$DR_ROOT_PASS" >/dev/null; mc mb --ignore-existing "local/$$DR_BUCKET" >/dev/null; mc rm --recursive --force "local/$$DR_BUCKET" >/dev/null 2>&1 || true; mc mirror --overwrite "/backup/$$DR_BUCKET" "local/$$DR_BUCKET" >/dev/null'; \
	echo "restore completed"

dr-drill-local: dr-backup-local
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@if ! command -v $(DOCKER) >/dev/null 2>&1; then \
		echo "$(DOCKER) not found. Install Docker first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ctx="$(K3D_CONTEXT)"; \
	ns="$(DB_MIGRATION_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	secret="$(DB_MIGRATION_SECRET)"; \
	db="$(DB_MIGRATION_DB)"; \
	archive_ns="$(ARCHIVE_INTEGRATION_NAMESPACE)"; \
	archive_secret="$(ARCHIVE_INTEGRATION_SECRET)"; \
	report_file="$(DR_REPORT_FILE)"; \
	if [ ! -f "$$report_file" ]; then \
		echo "backup report not found at $$report_file"; \
		exit 1; \
	fi; \
	source "$$report_file"; \
	bucket="$${archive_bucket:-$(DR_ARCHIVE_BUCKET)}"; \
	primary="$$(kubectl --context "$$ctx" -n "$$ns" get pods -l cnpg.io/cluster="$$cluster",cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')"; \
	if [ -z "$$primary" ]; then \
		echo "no CNPG primary pod found for cluster=$$cluster in namespace=$$ns"; \
		exit 1; \
	fi; \
	user="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.username}' | base64 -d)"; \
	pass="$$(kubectl --context "$$ctx" -n "$$ns" get secret "$$secret" -o jsonpath='{.data.password}' | base64 -d)"; \
	echo "simulating Postgres data loss"; \
	kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "TRUNCATE archive_object_manifest, user_devices, provider_devices, provider_credentials, users, devices RESTART IDENTITY CASCADE;"; \
	root_user="$$(kubectl --context "$$ctx" -n "$$archive_ns" get secret "$$archive_secret" -o jsonpath='{.data.rootUser}' | base64 -d)"; \
	root_pass="$$(kubectl --context "$$ctx" -n "$$archive_ns" get secret "$$archive_secret" -o jsonpath='{.data.rootPassword}' | base64 -d)"; \
	pf_log="$$(mktemp -t ecoflow-dr-minio-drill-port-forward.XXXXXX.log)"; \
	kubectl --context "$$ctx" -n "$$archive_ns" port-forward "svc/$(ARCHIVE_INTEGRATION_SERVICE)" "$(DR_MINIO_LOCAL_PORT):9000" >"$$pf_log" 2>&1 & \
	pf_pid=$$!; \
	cleanup() { \
		kill "$$pf_pid" >/dev/null 2>&1 || true; \
		wait "$$pf_pid" >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	ready=0; \
	for _ in {1..30}; do \
		if grep -q "Forwarding from" "$$pf_log" 2>/dev/null; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -ne 1 ]; then \
		echo "minio port-forward did not become ready; see $$pf_log"; \
		exit 1; \
	fi; \
	echo "simulating MinIO object loss for bucket $$bucket"; \
	$(DOCKER) run --rm \
		--entrypoint /bin/sh \
		-e DR_DOCKER_ENDPOINT="$(DR_MINIO_DOCKER_ENDPOINT)" \
		-e DR_ROOT_USER="$$root_user" \
		-e DR_ROOT_PASS="$$root_pass" \
		-e DR_BUCKET="$$bucket" \
		"$(DR_MINIO_MC_IMAGE)" \
		-c 'set -e; mc alias set local "http://$$DR_DOCKER_ENDPOINT" "$$DR_ROOT_USER" "$$DR_ROOT_PASS" >/dev/null; mc mb --ignore-existing "local/$$DR_BUCKET" >/dev/null; mc rm --recursive --force "local/$$DR_BUCKET" >/dev/null 2>&1 || true'; \
	kill "$$pf_pid" >/dev/null 2>&1 || true; \
	wait "$$pf_pid" >/dev/null 2>&1 || true; \
	trap - EXIT INT TERM; \
	"$${MAKE:-make}" dr-restore-local DR_BACKUP_NAME="$(DR_BACKUP_NAME)"; \
	"$${MAKE:-make}" db-migrate-verify-local; \
	exp_users="$${users_count:-0}"; \
	exp_devices="$${devices_count:-0}"; \
	exp_user_devices="$${user_devices_count:-0}"; \
	exp_provider_credentials="$${provider_credentials_count:-0}"; \
	exp_provider_devices="$${provider_devices_count:-0}"; \
	exp_archive_manifest="$${archive_manifest_count:-0}"; \
	exp_archive_objects="$${archive_object_count:-0}"; \
	actual_counts="$$(kubectl --context "$$ctx" -n "$$ns" exec "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -Atc "SELECT (SELECT COUNT(*) FROM users)::text || '|' || (SELECT COUNT(*) FROM devices)::text || '|' || (SELECT COUNT(*) FROM user_devices)::text || '|' || (SELECT COUNT(*) FROM provider_credentials)::text || '|' || (SELECT COUNT(*) FROM provider_devices)::text || '|' || (SELECT COUNT(*) FROM archive_object_manifest)::text;")"; \
	IFS='|' read -r act_users act_devices act_user_devices act_provider_credentials act_provider_devices act_archive_manifest <<< "$$actual_counts"; \
	pf_log="$$(mktemp -t ecoflow-dr-minio-validate-port-forward.XXXXXX.log)"; \
	kubectl --context "$$ctx" -n "$$archive_ns" port-forward "svc/$(ARCHIVE_INTEGRATION_SERVICE)" "$(DR_MINIO_LOCAL_PORT):9000" >"$$pf_log" 2>&1 & \
	pf_pid=$$!; \
	cleanup() { \
		kill "$$pf_pid" >/dev/null 2>&1 || true; \
		wait "$$pf_pid" >/dev/null 2>&1 || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	ready=0; \
	for _ in {1..30}; do \
		if grep -q "Forwarding from" "$$pf_log" 2>/dev/null; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -ne 1 ]; then \
		echo "minio port-forward did not become ready; see $$pf_log"; \
		exit 1; \
	fi; \
	act_archive_objects="$$( $(DOCKER) run --rm \
		--entrypoint /bin/sh \
		-e DR_DOCKER_ENDPOINT="$(DR_MINIO_DOCKER_ENDPOINT)" \
		-e DR_ROOT_USER="$$root_user" \
		-e DR_ROOT_PASS="$$root_pass" \
		-e DR_BUCKET="$$bucket" \
		"$(DR_MINIO_MC_IMAGE)" \
		-c 'set -e; mc alias set local "http://$$DR_DOCKER_ENDPOINT" "$$DR_ROOT_USER" "$$DR_ROOT_PASS" >/dev/null; mc ls --recursive "local/$$DR_BUCKET" | wc -l | tr -d " "' )"; \
	kill "$$pf_pid" >/dev/null 2>&1 || true; \
	wait "$$pf_pid" >/dev/null 2>&1 || true; \
	trap - EXIT INT TERM; \
	if [ "$$act_users" -lt "$$exp_users" ] || [ "$$act_devices" -lt "$$exp_devices" ] || [ "$$act_user_devices" -lt "$$exp_user_devices" ] || [ "$$act_provider_credentials" -lt "$$exp_provider_credentials" ] || [ "$$act_provider_devices" -lt "$$exp_provider_devices" ] || [ "$$act_archive_manifest" -lt "$$exp_archive_manifest" ] || [ "$$act_archive_objects" -lt "$$exp_archive_objects" ]; then \
		echo "restore validation failed (actual counts dropped below backup baseline)"; \
		echo "expected db counts: users=$$exp_users devices=$$exp_devices user_devices=$$exp_user_devices provider_credentials=$$exp_provider_credentials provider_devices=$$exp_provider_devices archive_manifest=$$exp_archive_manifest"; \
		echo "actual db counts:   users=$$act_users devices=$$act_devices user_devices=$$act_user_devices provider_credentials=$$act_provider_credentials provider_devices=$$act_provider_devices archive_manifest=$$act_archive_manifest"; \
		echo "expected archive objects=$$exp_archive_objects actual archive objects=$$act_archive_objects"; \
		exit 1; \
	fi; \
	if [ "$$act_users" -gt "$$exp_users" ] || [ "$$act_devices" -gt "$$exp_devices" ] || [ "$$act_user_devices" -gt "$$exp_user_devices" ] || [ "$$act_provider_credentials" -gt "$$exp_provider_credentials" ] || [ "$$act_provider_devices" -gt "$$exp_provider_devices" ] || [ "$$act_archive_manifest" -gt "$$exp_archive_manifest" ] || [ "$$act_archive_objects" -gt "$$exp_archive_objects" ]; then \
		echo "restore validation note: observed post-restore growth above backup baseline (likely live ingest during drill)"; \
		echo "expected db counts: users=$$exp_users devices=$$exp_devices user_devices=$$exp_user_devices provider_credentials=$$exp_provider_credentials provider_devices=$$exp_provider_devices archive_manifest=$$exp_archive_manifest"; \
		echo "actual db counts:   users=$$act_users devices=$$act_devices user_devices=$$act_user_devices provider_credentials=$$act_provider_credentials provider_devices=$$act_provider_devices archive_manifest=$$act_archive_manifest"; \
		echo "expected archive objects=$$exp_archive_objects actual archive objects=$$act_archive_objects"; \
	fi; \
	echo "drill validation passed (db + archive object counts restored; actual >= backup baseline)"

auth-keycloak-verify-local:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ns="$(PLATFORM_NAMESPACE)"; \
	secret_name="$(PLATFORM_RELEASE)-keycloak"; \
	admin_password="$$( $(LOCAL_KUBECTL) -n "$$ns" get secret "$$secret_name" -o jsonpath='{.data.admin-password}' | base64 --decode )"; \
	pod_name="$$( $(LOCAL_KUBECTL) -n "$$ns" get pods -l app.kubernetes.io/instance=$(PLATFORM_RELEASE),app.kubernetes.io/component=keycloak -o jsonpath='{.items[0].metadata.name}' )"; \
	if [ -z "$$pod_name" ]; then \
		echo "no Keycloak pod found in namespace $$ns"; \
		exit 1; \
	fi; \
	$(LOCAL_KUBECTL) -n "$$ns" exec "$$pod_name" -- env HOME=/tmp /opt/bitnami/keycloak/bin/kcadm.sh config credentials --server http://127.0.0.1:8080 --realm master --user "$(KEYCLOAK_ADMIN_USER)" --password "$$admin_password" >/dev/null; \
	$(LOCAL_KUBECTL) -n "$$ns" exec "$$pod_name" -- env HOME=/tmp /opt/bitnami/keycloak/bin/kcadm.sh get "realms/$(KEYCLOAK_REALM_NAME)" --fields realm >/dev/null; \
	for alias in google facebook; do \
		$(LOCAL_KUBECTL) -n "$$ns" exec "$$pod_name" -- env HOME=/tmp /opt/bitnami/keycloak/bin/kcadm.sh get "identity-provider/instances/$$alias" -r "$(KEYCLOAK_REALM_NAME)" --fields alias >/dev/null; \
	done; \
	echo "keycloak realm verification passed: realm=$(KEYCLOAK_REALM_NAME), providers=google,facebook"

gke-context:
	@if ! command -v $(GCLOUD) >/dev/null 2>&1; then \
		echo "$(GCLOUD) not found. Install Google Cloud SDK first."; \
		exit 1; \
	fi
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@if [ -z "$(GKE_PROJECT_ID)" ]; then \
		echo "GKE_PROJECT_ID is required. Example: make gke-context GKE_PROJECT_ID=my-project"; \
		exit 1; \
	fi
	@echo "fetching kube credentials for $(GKE_CLUSTER_NAME) in $(GKE_CLUSTER_ZONE) (project: $(GKE_PROJECT_ID))"
	$(GCLOUD) container clusters get-credentials $(GKE_CLUSTER_NAME) \
		--zone $(GKE_CLUSTER_ZONE) \
		--project $(GKE_PROJECT_ID)

gke-cloud-context:
	@if ! command -v $(GCLOUD) >/dev/null 2>&1; then \
		echo "$(GCLOUD) not found. Install Google Cloud SDK first."; \
		exit 1; \
	fi
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@echo "fetching kube credentials for regional cluster $(GKE_CLOUD_CLUSTER_NAME) in $(GKE_CLOUD_CLUSTER_REGION) (project: $(GKE_CLOUD_PROJECT_ID))"
	$(GCLOUD) container clusters get-credentials $(GKE_CLOUD_CLUSTER_NAME) \
		--region $(GKE_CLOUD_CLUSTER_REGION) \
		--project $(GKE_CLOUD_PROJECT_ID)

gke-dev-guardrails: gke-context
	@echo "ensuring namespace $(GKE_DEV_NAMESPACE) exists"
	@$(KUBECTL) get ns $(GKE_DEV_NAMESPACE) >/dev/null 2>&1 || $(KUBECTL) create ns $(GKE_DEV_NAMESPACE)
	@echo "applying dev guardrails in $(GKE_DEV_NAMESPACE)"
	$(KUBECTL) apply -f $(GKE_GUARDRAILS_DIR)/pulse-dev-resourcequota.yaml
	$(KUBECTL) apply -f $(GKE_GUARDRAILS_DIR)/pulse-dev-limitrange.yaml

gke-park: gke-context
	@set -euo pipefail; \
	ns="$(GKE_DEV_NAMESPACE)"; \
	echo "parking stateless workloads in $$ns"; \
	for dep in $(GKE_STATELESS_DEPLOYMENTS); do \
		if $(KUBECTL) -n "$$ns" get deploy "$$dep" >/dev/null 2>&1; then \
			echo "scaling deploy/$$dep -> $(GKE_PARK_REPLICAS)"; \
			$(KUBECTL) -n "$$ns" scale deploy/"$$dep" --replicas=$(GKE_PARK_REPLICAS); \
		else \
			echo "skipping missing deploy/$$dep in $$ns"; \
		fi; \
	done; \
	if $(GCLOUD) container node-pools describe $(GKE_BASELINE_NODEPOOL) --cluster $(GKE_CLUSTER_NAME) --zone $(GKE_CLUSTER_ZONE) --project $(GKE_PROJECT_ID) >/dev/null 2>&1; then \
		echo "setting baseline node pool min/max to $(GKE_BASELINE_MIN_PARKED)/$(GKE_BASELINE_MAX)"; \
		$(GCLOUD) container node-pools update $(GKE_BASELINE_NODEPOOL) \
			--cluster $(GKE_CLUSTER_NAME) \
			--zone $(GKE_CLUSTER_ZONE) \
			--project $(GKE_PROJECT_ID) \
			--enable-autoscaling \
			--min-nodes $(GKE_BASELINE_MIN_PARKED) \
			--max-nodes $(GKE_BASELINE_MAX); \
	else \
		echo "skipping missing node pool $(GKE_BASELINE_NODEPOOL)"; \
	fi; \
	if $(GCLOUD) container node-pools describe $(GKE_SPOT_NODEPOOL) --cluster $(GKE_CLUSTER_NAME) --zone $(GKE_CLUSTER_ZONE) --project $(GKE_PROJECT_ID) >/dev/null 2>&1; then \
		echo "setting spot node pool min/max to $(GKE_SPOT_MIN)/$(GKE_SPOT_MAX)"; \
		$(GCLOUD) container node-pools update $(GKE_SPOT_NODEPOOL) \
			--cluster $(GKE_CLUSTER_NAME) \
			--zone $(GKE_CLUSTER_ZONE) \
			--project $(GKE_PROJECT_ID) \
			--enable-autoscaling \
			--min-nodes $(GKE_SPOT_MIN) \
			--max-nodes $(GKE_SPOT_MAX); \
	else \
		echo "skipping missing node pool $(GKE_SPOT_NODEPOOL)"; \
	fi

gke-wake: gke-context gke-dev-guardrails
	@set -euo pipefail; \
	ns="$(GKE_DEV_NAMESPACE)"; \
	if $(GCLOUD) container node-pools describe $(GKE_BASELINE_NODEPOOL) --cluster $(GKE_CLUSTER_NAME) --zone $(GKE_CLUSTER_ZONE) --project $(GKE_PROJECT_ID) >/dev/null 2>&1; then \
		echo "setting baseline node pool min/max to $(GKE_BASELINE_MIN_ACTIVE)/$(GKE_BASELINE_MAX)"; \
		$(GCLOUD) container node-pools update $(GKE_BASELINE_NODEPOOL) \
			--cluster $(GKE_CLUSTER_NAME) \
			--zone $(GKE_CLUSTER_ZONE) \
			--project $(GKE_PROJECT_ID) \
			--enable-autoscaling \
			--min-nodes $(GKE_BASELINE_MIN_ACTIVE) \
			--max-nodes $(GKE_BASELINE_MAX); \
	else \
		echo "skipping missing node pool $(GKE_BASELINE_NODEPOOL)"; \
	fi; \
	if $(GCLOUD) container node-pools describe $(GKE_SPOT_NODEPOOL) --cluster $(GKE_CLUSTER_NAME) --zone $(GKE_CLUSTER_ZONE) --project $(GKE_PROJECT_ID) >/dev/null 2>&1; then \
		echo "setting spot node pool min/max to $(GKE_SPOT_MIN)/$(GKE_SPOT_MAX)"; \
		$(GCLOUD) container node-pools update $(GKE_SPOT_NODEPOOL) \
			--cluster $(GKE_CLUSTER_NAME) \
			--zone $(GKE_CLUSTER_ZONE) \
			--project $(GKE_PROJECT_ID) \
			--enable-autoscaling \
			--min-nodes $(GKE_SPOT_MIN) \
			--max-nodes $(GKE_SPOT_MAX); \
	else \
		echo "skipping missing node pool $(GKE_SPOT_NODEPOOL)"; \
	fi; \
	echo "waking stateless workloads in $$ns"; \
	for dep in $(GKE_STATELESS_DEPLOYMENTS); do \
		if $(KUBECTL) -n "$$ns" get deploy "$$dep" >/dev/null 2>&1; then \
			echo "scaling deploy/$$dep -> $(GKE_WAKE_REPLICAS)"; \
			$(KUBECTL) -n "$$ns" scale deploy/"$$dep" --replicas=$(GKE_WAKE_REPLICAS); \
		else \
			echo "skipping missing deploy/$$dep in $$ns"; \
		fi; \
	done

scale-down: gke-park

scale-up: gke-wake

cloud-context: gke-cloud-context

argocd-bootstrap-dev: gke-context
	@if ! command -v $(HELM) >/dev/null 2>&1; then \
		echo "$(HELM) not found. Install helm first."; \
		exit 1; \
	fi
	@echo "ensuring namespace $(ARGOCD_NAMESPACE) exists"
	@$(KUBECTL) get ns $(ARGOCD_NAMESPACE) >/dev/null 2>&1 || $(KUBECTL) create ns $(ARGOCD_NAMESPACE)
	@echo "installing/upgrading Argo CD chart $(ARGOCD_HELM_CHART) ($(ARGOCD_CHART_VERSION))"
	@$(HELM) repo add argo $(ARGOCD_HELM_REPO) >/dev/null 2>&1 || true
	@$(HELM) repo update >/dev/null 2>&1
	$(HELM) upgrade --install $(ARGOCD_RELEASE) $(ARGOCD_HELM_CHART) \
		--version $(ARGOCD_CHART_VERSION) \
		--namespace $(ARGOCD_NAMESPACE) \
		--create-namespace \
		-f $(ARGOCD_VALUES_DEV)
	@echo "waiting for Argo CD CRDs and core workloads"
	$(KUBECTL) wait --for=condition=Established --timeout=$(WAIT_TIMEOUT) crd/applications.argoproj.io
	$(KUBECTL) -n $(ARGOCD_NAMESPACE) rollout status deploy/argocd-server --timeout=$(WAIT_TIMEOUT)
	$(KUBECTL) -n $(ARGOCD_NAMESPACE) rollout status deploy/argocd-repo-server --timeout=$(WAIT_TIMEOUT)
	$(KUBECTL) -n $(ARGOCD_NAMESPACE) rollout status sts/argocd-application-controller --timeout=$(WAIT_TIMEOUT)

argocd-apps-dev: gke-context
	@set -euo pipefail; \
	for app in $(ARGOCD_APPS); do \
		manifest="$(ARGOCD_APPS_DIR)/$$app.yaml"; \
		if [ ! -f "$$manifest" ]; then \
			echo "missing Argo application manifest: $$manifest"; \
			exit 1; \
		fi; \
		echo "applying $$manifest"; \
		$(KUBECTL) apply -f "$$manifest"; \
	done

argocd-wait-apps: gke-context
	@set -euo pipefail; \
	for app in $(ARGOCD_APPS); do \
		echo "waiting for application/$$app (Synced + Healthy)"; \
		ok=0; \
		for attempt in $$(seq 1 $(ARGOCD_APP_WAIT_ATTEMPTS)); do \
			sync="$$( $(KUBECTL) -n $(ARGOCD_NAMESPACE) get application "$$app" -o jsonpath='{.status.sync.status}' 2>/dev/null || true )"; \
			health="$$( $(KUBECTL) -n $(ARGOCD_NAMESPACE) get application "$$app" -o jsonpath='{.status.health.status}' 2>/dev/null || true )"; \
			echo "  attempt $$attempt/$(ARGOCD_APP_WAIT_ATTEMPTS): sync=$${sync:-n/a} health=$${health:-n/a}"; \
			if [ "$$sync" = "Synced" ] && [ "$$health" = "Healthy" ]; then \
				ok=1; \
				break; \
			fi; \
			sleep $(ARGOCD_APP_WAIT_SLEEP_SEC); \
		done; \
		if [ $$ok -ne 1 ]; then \
			echo "application/$$app did not reach Synced+Healthy"; \
			$(KUBECTL) -n $(ARGOCD_NAMESPACE) get application "$$app" -o yaml || true; \
			exit 1; \
		fi; \
	done

argocd-dev-up: argocd-bootstrap-dev argocd-apps-dev argocd-wait-apps

argocd-bootstrap-cloud: gke-cloud-context
	@if ! command -v $(HELM) >/dev/null 2>&1; then \
		echo "$(HELM) not found. Install helm first."; \
		exit 1; \
	fi
	@echo "ensuring namespace $(ARGOCD_NAMESPACE) exists"
	@$(KUBECTL) get ns $(ARGOCD_NAMESPACE) >/dev/null 2>&1 || $(KUBECTL) create ns $(ARGOCD_NAMESPACE)
	@echo "installing/upgrading Argo CD chart $(ARGOCD_HELM_CHART) ($(ARGOCD_CHART_VERSION))"
	@$(HELM) repo add argo $(ARGOCD_HELM_REPO) >/dev/null 2>&1 || true
	@$(HELM) repo update >/dev/null 2>&1
	$(HELM) upgrade --install $(ARGOCD_RELEASE) $(ARGOCD_HELM_CHART) \
		--version $(ARGOCD_CHART_VERSION) \
		--namespace $(ARGOCD_NAMESPACE) \
		--create-namespace \
		-f $(ARGOCD_VALUES_CLOUD)
	@echo "waiting for Argo CD CRDs and core workloads"
	$(KUBECTL) wait --for=condition=Established --timeout=$(WAIT_TIMEOUT) crd/applications.argoproj.io
	$(KUBECTL) -n $(ARGOCD_NAMESPACE) rollout status deploy/argocd-server --timeout=$(WAIT_TIMEOUT)
	$(KUBECTL) -n $(ARGOCD_NAMESPACE) rollout status deploy/argocd-repo-server --timeout=$(WAIT_TIMEOUT)
	$(KUBECTL) -n $(ARGOCD_NAMESPACE) rollout status sts/argocd-application-controller --timeout=$(WAIT_TIMEOUT)

argocd-apps-cloud: gke-cloud-context
	@set -euo pipefail; \
	for app in $(ARGOCD_CLOUD_APPS); do \
		manifest="$(ARGOCD_APPS_DIR)/$$app.yaml"; \
		if [ ! -f "$$manifest" ]; then \
			echo "missing Argo application manifest: $$manifest"; \
			exit 1; \
		fi; \
		echo "applying $$manifest"; \
		$(KUBECTL) apply -f "$$manifest"; \
	done

argocd-wait-apps-cloud: gke-cloud-context
	@set -euo pipefail; \
	for app in $(ARGOCD_CLOUD_APPS); do \
		echo "waiting for application/$$app (Synced + Healthy)"; \
		ok=0; \
		for attempt in $$(seq 1 $(ARGOCD_APP_WAIT_ATTEMPTS)); do \
			sync="$$( $(KUBECTL) -n $(ARGOCD_NAMESPACE) get application "$$app" -o jsonpath='{.status.sync.status}' 2>/dev/null || true )"; \
			health="$$( $(KUBECTL) -n $(ARGOCD_NAMESPACE) get application "$$app" -o jsonpath='{.status.health.status}' 2>/dev/null || true )"; \
			echo "  attempt $$attempt/$(ARGOCD_APP_WAIT_ATTEMPTS): sync=$${sync:-n/a} health=$${health:-n/a}"; \
			if [ "$$sync" = "Synced" ] && [ "$$health" = "Healthy" ]; then \
				ok=1; \
				break; \
			fi; \
			sleep $(ARGOCD_APP_WAIT_SLEEP_SEC); \
		done; \
		if [ $$ok -ne 1 ]; then \
			echo "application/$$app did not reach Synced+Healthy"; \
			$(KUBECTL) -n $(ARGOCD_NAMESPACE) get application "$$app" -o yaml || true; \
			exit 1; \
		fi; \
	done

argocd-cloud-up: argocd-bootstrap-cloud argocd-apps-cloud argocd-wait-apps-cloud

cloud-up: argocd-cloud-up

cloud-refresh: argocd-apps-cloud argocd-wait-apps-cloud

cloud-health-gate: gke-cloud-context
	@set -euo pipefail; \
	platform_ns="$(PLATFORM_NAMESPACE)"; \
	services_ns="$(SERVICES_NAMESPACE)"; \
	cluster="$(DB_MIGRATION_CLUSTER)"; \
	echo "checking CNPG cluster $$cluster"; \
	ready="$$( $(KUBECTL) -n "$$platform_ns" get cluster "$$cluster" -o jsonpath='{.status.readyInstances}' 2>/dev/null || true )"; \
	ready="$${ready:-0}"; \
	current_primary="$$( $(KUBECTL) -n "$$platform_ns" get cluster "$$cluster" -o jsonpath='{.status.currentPrimary}' 2>/dev/null || true )"; \
	if [ "$$ready" -lt 2 ] || [ -z "$$current_primary" ]; then \
		echo "CNPG is not ready enough for rollout (readyInstances=$$ready currentPrimary=$${current_primary:-n/a})"; \
		$(KUBECTL) -n "$$platform_ns" get cluster "$$cluster" -o yaml || true; \
		exit 1; \
	fi; \
	rw_ips="$$( $(KUBECTL) -n "$$platform_ns" get endpoints "$$cluster-rw" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true )"; \
	if [ -z "$$rw_ips" ]; then \
		echo "CNPG rw endpoint $$cluster-rw has no ready addresses"; \
		exit 1; \
	fi; \
	for sts in "$(CLOUD_PLATFORM_RELEASE)-nats:3" "$(CLOUD_PLATFORM_RELEASE)-valkey-node:3"; do \
		name="$${sts%%:*}"; \
		want="$${sts##*:}"; \
		got="$$( $(KUBECTL) -n "$$platform_ns" get statefulset "$$name" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || true )"; \
		got="$${got:-0}"; \
		if [ "$$got" -lt "$$want" ]; then \
			echo "statefulset/$$name is not ready enough (ready=$$got want=$$want)"; \
			$(KUBECTL) -n "$$platform_ns" get statefulset "$$name" -o wide || true; \
			exit 1; \
		fi; \
	done; \
	bad_pods="$$( $(KUBECTL) get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded --no-headers 2>/dev/null || true )"; \
	if [ -n "$$bad_pods" ]; then \
		echo "non-running pods block rollout:"; \
		printf '%s\n' "$$bad_pods"; \
		exit 1; \
	fi; \
	for deploy in "$(CLOUD_SERVICES_RELEASE)-go-ingest" "$(CLOUD_SERVICES_RELEASE)-go-archive" "$(CLOUD_SERVICES_RELEASE)-go-rollup"; do \
		if $(KUBECTL) -n "$$services_ns" get deploy "$$deploy" >/dev/null 2>&1; then \
			if $(KUBECTL) -n "$$services_ns" logs deploy/"$$deploy" --since=10m 2>/dev/null | grep -Eiq 'dropping envelope|drain failed|publish failed'; then \
				echo "recent failure/drop log found in deployment/$$deploy; aborting rollout gate"; \
				exit 1; \
			fi; \
		fi; \
	done; \
	echo "cloud health gate passed: CNPG ready=$$ready primary=$$current_primary rw=$$rw_ips"

cloud-platform-apply: gke-cloud-context cloud-health-gate
	@$(MAKE) --no-print-directory chart-deps-local CHART=$(PLATFORM_CHART)
	@set -euo pipefail; \
	set --; \
	for values in $(CLOUD_PLATFORM_VALUES); do \
		set -- "$$@" -f "$$values"; \
	done; \
	ownership_args=""; \
	case "$(CLOUD_HELM_TAKE_OWNERSHIP)" in 1|true|TRUE|yes|YES) ownership_args="--take-ownership" ;; esac; \
	server_side_args=""; \
	if [ -n "$(CLOUD_HELM_SERVER_SIDE)" ]; then \
		server_side_args="--server-side=$(CLOUD_HELM_SERVER_SIDE)"; \
	fi; \
	echo "applying $(CLOUD_PLATFORM_RELEASE) with values: $(CLOUD_PLATFORM_VALUES)"; \
	$(HELM) upgrade --install $(CLOUD_PLATFORM_RELEASE) $(PLATFORM_CHART) \
		--namespace $(PLATFORM_NAMESPACE) \
		--create-namespace \
		"$$@" \
		$$ownership_args \
		$$server_side_args \
		$(CLOUD_HELM_FORCE_CONFLICTS_ARGS) \
		$(CLOUD_PLATFORM_HELM_SET_ARGS) \
		$(CLOUD_HELM_EXTRA_ARGS)
	@set -euo pipefail; \
	ns="$(PLATFORM_NAMESPACE)"; \
	wait_rollout() { \
		kind="$$1"; name="$$2"; timeout="$$3"; \
		if $(KUBECTL) -n "$$ns" get "$$kind" "$$name" >/dev/null 2>&1; then \
			echo "waiting for $$kind/$$name"; \
			$(KUBECTL) -n "$$ns" rollout status "$$kind/$$name" --timeout="$$timeout"; \
		fi; \
	}; \
	wait_rollout deployment $(CLOUD_PLATFORM_RELEASE)-external-secrets 300s; \
	wait_rollout deployment $(CLOUD_PLATFORM_RELEASE)-external-secrets-webhook 300s; \
	wait_rollout deployment $(CLOUD_PLATFORM_RELEASE)-public-app 300s; \
	wait_rollout deployment $(CLOUD_PLATFORM_RELEASE)-realtime-gateway 300s; \
	wait_rollout statefulset $(CLOUD_PLATFORM_RELEASE)-nats $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(CLOUD_PLATFORM_RELEASE)-valkey-node $(WAIT_TIMEOUT); \
	$(KUBECTL) -n "$$ns" wait --for=condition=Ready cluster/$(DB_MIGRATION_CLUSTER) --timeout=$(WAIT_TIMEOUT)
	@$(MAKE) --no-print-directory cloud-health-gate

cloud-services-apply: gke-cloud-context cloud-health-gate
	@$(MAKE) --no-print-directory chart-deps-local CHART=$(SERVICES_CHART)
	@set -euo pipefail; \
	set --; \
	for values in $(CLOUD_SERVICES_VALUES); do \
		set -- "$$@" -f "$$values"; \
	done; \
	ownership_args=""; \
	case "$(CLOUD_HELM_TAKE_OWNERSHIP)" in 1|true|TRUE|yes|YES) ownership_args="--take-ownership" ;; esac; \
	server_side_args=""; \
	if [ -n "$(CLOUD_HELM_SERVER_SIDE)" ]; then \
		server_side_args="--server-side=$(CLOUD_HELM_SERVER_SIDE)"; \
	fi; \
	echo "applying $(CLOUD_SERVICES_RELEASE) with values: $(CLOUD_SERVICES_VALUES)"; \
	$(HELM) upgrade --install $(CLOUD_SERVICES_RELEASE) $(SERVICES_CHART) \
		--namespace $(SERVICES_NAMESPACE) \
		--create-namespace \
		"$$@" \
		$$ownership_args \
		$$server_side_args \
		$(CLOUD_HELM_FORCE_CONFLICTS_ARGS) \
		$(CLOUD_SERVICES_HELM_SET_ARGS) \
		$(CLOUD_HELM_EXTRA_ARGS)
	@set -euo pipefail; \
	ns="$(SERVICES_NAMESPACE)"; \
	for component in go-ingest go-archive go-rollup go-projection go-inference go-grpc-api go-energy-api go-solar-verification go-scheduler; do \
		deploy="$(CLOUD_SERVICES_RELEASE)-$$component"; \
		if $(KUBECTL) -n "$$ns" get deploy "$$deploy" >/dev/null 2>&1; then \
			echo "waiting for deployment/$$deploy"; \
			$(KUBECTL) -n "$$ns" rollout status deploy/"$$deploy" --timeout=$(WAIT_TIMEOUT); \
		fi; \
	done
	@$(MAKE) --no-print-directory cloud-health-gate

cloud-deploy: cloud-platform-apply cloud-services-apply

cloud-cost-min-deploy:
	$(MAKE) cloud-deploy \
		CLOUD_PLATFORM_VALUES="$(CLOUD_PLATFORM_VALUES) $(CLOUD_COST_MIN_PLATFORM_VALUES)" \
		CLOUD_SERVICES_VALUES="$(CLOUD_SERVICES_VALUES) $(CLOUD_COST_MIN_SERVICES_VALUES)"

cloud-forward-image-build:
	docker build \
		--platform "$(CLOUD_FORWARD_DOCKER_PLATFORM)" \
		-f "$(CLOUD_FORWARD_DOCKERFILE)" \
		-t "$(CLOUD_FORWARD_IMAGE)" \
		.

cloud-db-forward: gke-cloud-context
	@$(MAKE) --no-print-directory cloud-db-forward-start
	@echo "following $(CLOUD_DB_FORWARD_CONTAINER) logs; stop with: make cloud-db-forward-stop"
	docker logs -f "$(CLOUD_DB_FORWARD_CONTAINER)"

cloud-db-forward-start: gke-cloud-context
	@set -euo pipefail; \
	mkdir -p "$(dir $(CLOUD_DB_FORWARD_PID_FILE))"; \
	if ! docker image inspect "$(CLOUD_FORWARD_IMAGE)" >/dev/null 2>&1 || \
		! docker run --rm --platform "$(CLOUD_FORWARD_DOCKER_PLATFORM)" --entrypoint test "$(CLOUD_FORWARD_IMAGE)" -x /usr/local/bin/cloud-forward-supervisor.sh >/dev/null 2>&1; then \
		$(MAKE) --no-print-directory cloud-forward-image-build; \
	fi; \
	container="$(CLOUD_DB_FORWARD_CONTAINER)"; \
	if docker inspect "$$container" >/dev/null 2>&1; then \
		if [ "$$(docker inspect -f '{{.State.Running}}' "$$container")" = "true" ]; then \
			container_cmd="$$(docker inspect -f '{{json .Config.Cmd}}' "$$container")"; \
			if ! printf '%s\n' "$$container_cmd" | grep -q 'cloud-forward-supervisor.sh'; then \
				echo "cloud DB forward container is running without supervisor; restarting $$container"; \
				docker rm -f "$$container" >/dev/null; \
			elif scripts/cloud-forward-probe.sh postgres $(CLOUD_DB_LOCAL_PORT); then \
				echo "$$(docker inspect -f '{{.State.Pid}}' "$$container")" > "$(CLOUD_DB_FORWARD_PID_FILE)"; \
				echo "cloud DB forward container is running ($$container)"; \
				exit 0; \
			else \
				echo "cloud DB forward container is running but 127.0.0.1:$(CLOUD_DB_LOCAL_PORT) failed Postgres protocol probe"; \
				docker logs --tail 80 "$$container" || true; \
				echo "restarting stale cloud DB forward container $$container"; \
				docker rm -f "$$container" >/dev/null; \
			fi; \
		else \
			docker rm "$$container" >/dev/null; \
		fi; \
	fi; \
	if [ "$$(uname -s)" = "Darwin" ] && command -v launchctl >/dev/null 2>&1; then \
		domain="gui/$$(id -u)"; \
		launchctl bootout "$$domain/$(CLOUD_DB_FORWARD_LABEL)" >/dev/null 2>&1 || true; \
		rm -f "$$HOME/Library/LaunchAgents/$(CLOUD_DB_FORWARD_LABEL).plist"; \
	fi; \
	pids="$$(pgrep -f 'port-forward --address .* svc/$(DB_MIGRATION_CLUSTER)-rw $(CLOUD_DB_LOCAL_PORT):5432' || true)"; \
	if [ -n "$$pids" ]; then \
		echo "$$pids" | xargs kill >/dev/null 2>&1 || true; \
	fi; \
	rm -f "$(CLOUD_DB_FORWARD_PID_FILE)"; \
	if scripts/cloud-forward-probe.sh postgres $(CLOUD_DB_LOCAL_PORT); then \
		echo "cloud DB forward port $(CLOUD_DB_LOCAL_PORT) is already reachable but not managed by Docker"; \
		exit 0; \
	elif nc -z 127.0.0.1 $(CLOUD_DB_LOCAL_PORT) >/dev/null 2>&1; then \
		echo "cloud DB forward port $(CLOUD_DB_LOCAL_PORT) is reachable but failed Postgres protocol probe"; \
		exit 1; \
	fi; \
	echo "starting cloud DB forward container $$container on $(CLOUD_DB_FORWARD_ADDRESS):$(CLOUD_DB_LOCAL_PORT)"; \
	docker run -d \
		--name "$$container" \
		--restart "$(CLOUD_FORWARD_RESTART)" \
		--platform "$(CLOUD_FORWARD_DOCKER_PLATFORM)" \
		-p "$(CLOUD_DB_FORWARD_ADDRESS):$(CLOUD_DB_LOCAL_PORT):$(CLOUD_DB_LOCAL_PORT)" \
		-v "$(CLOUD_FORWARD_KUBECONFIG_DIR):/root/.kube:ro" \
		-v "$(CLOUD_FORWARD_GCLOUD_CONFIG_DIR):/root/.config/gcloud" \
		-e KUBECONFIG=/root/.kube/config \
		-e CLOUDSDK_CONFIG=/root/.config/gcloud \
		-e USE_GKE_GCLOUD_AUTH_PLUGIN=True \
		-e CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC="$(CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC)" \
		-e CLOUD_FORWARD_SUPERVISOR_RESTART_DELAY_SEC="$(CLOUD_FORWARD_SUPERVISOR_RESTART_DELAY_SEC)" \
		-e CLOUD_FORWARD_SUPERVISOR_STARTUP_GRACE_SEC="$(CLOUD_FORWARD_SUPERVISOR_STARTUP_GRACE_SEC)" \
		-e CLOUD_FORWARD_SUPERVISOR_FAILURE_THRESHOLD="$(CLOUD_FORWARD_SUPERVISOR_FAILURE_THRESHOLD)" \
		"$(CLOUD_FORWARD_IMAGE)" \
		cloud-forward-supervisor.sh "cloud DB" postgres "$(CLOUD_DB_LOCAL_PORT)" \
		kubectl --context "$(CLOUD_KUBE_CONTEXT)" -n "$(PLATFORM_NAMESPACE)" port-forward --address 0.0.0.0 svc/$(DB_MIGRATION_CLUSTER)-rw $(CLOUD_DB_LOCAL_PORT):5432; \
	for _ in $$(seq 1 30); do \
		if scripts/cloud-forward-probe.sh postgres $(CLOUD_DB_LOCAL_PORT); then \
			echo "$$(docker inspect -f '{{.State.Pid}}' "$$container")" > "$(CLOUD_DB_FORWARD_PID_FILE)"; \
			echo "cloud DB forward ready on $(CLOUD_DB_FORWARD_ADDRESS):$(CLOUD_DB_LOCAL_PORT) ($$container)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "cloud DB forward did not become ready"; \
	docker logs --tail 80 "$$container" || true; \
	rm -f "$(CLOUD_DB_FORWARD_PID_FILE)"; \
	exit 1

cloud-db-forward-stop:
	@set -euo pipefail; \
	container="$(CLOUD_DB_FORWARD_CONTAINER)"; \
	if docker inspect "$$container" >/dev/null 2>&1; then \
		echo "stopping cloud DB forward container $$container"; \
		docker rm -f "$$container" >/dev/null; \
	else \
		echo "cloud DB forward container is not running"; \
	fi; \
	if [ "$$(uname -s)" = "Darwin" ] && command -v launchctl >/dev/null 2>&1; then \
		domain="gui/$$(id -u)"; \
		launchctl bootout "$$domain/$(CLOUD_DB_FORWARD_LABEL)" >/dev/null 2>&1 || true; \
		rm -f "$$HOME/Library/LaunchAgents/$(CLOUD_DB_FORWARD_LABEL).plist"; \
	fi; \
	pids="$$(pgrep -f 'port-forward --address .* svc/$(DB_MIGRATION_CLUSTER)-rw $(CLOUD_DB_LOCAL_PORT):5432' || true)"; \
	if [ -n "$$pids" ]; then \
		echo "$$pids" | xargs kill >/dev/null 2>&1 || true; \
		echo "stopped legacy cloud DB forward process(es)"; \
	fi; \
	rm -f "$(CLOUD_DB_FORWARD_PID_FILE)"

cloud-db-forward-status:
	@set -euo pipefail; \
	container="$(CLOUD_DB_FORWARD_CONTAINER)"; \
	if docker inspect "$$container" >/dev/null 2>&1 && [ "$$(docker inspect -f '{{.State.Running}}' "$$container")" = "true" ]; then \
		echo "cloud DB forward container is running ($$container)"; \
	else \
		echo "cloud DB forward container is not running"; \
	fi; \
	if scripts/cloud-forward-probe.sh postgres $(CLOUD_DB_LOCAL_PORT); then \
		echo "127.0.0.1:$(CLOUD_DB_LOCAL_PORT) passed Postgres protocol probe"; \
	else \
		if nc -z 127.0.0.1 $(CLOUD_DB_LOCAL_PORT) >/dev/null 2>&1; then \
			echo "127.0.0.1:$(CLOUD_DB_LOCAL_PORT) is reachable but failed Postgres protocol probe"; \
		else \
			echo "127.0.0.1:$(CLOUD_DB_LOCAL_PORT) is not reachable"; \
		fi; \
		exit 1; \
	fi

cloud-realtime-forward: gke-cloud-context
	@echo "forwarding cloud NATS to $(CLOUD_REALTIME_FORWARD_ADDRESS):$(CLOUD_NATS_LOCAL_PORT)"
	@$(KUBECTL) --context $(CLOUD_KUBE_CONTEXT) -n $(PLATFORM_NAMESPACE) port-forward --address $(CLOUD_REALTIME_FORWARD_ADDRESS) svc/$(CLOUD_NATS_SERVICE) $(CLOUD_NATS_LOCAL_PORT):4222 & \
	nats_pid="$$!"; \
	echo "forwarding cloud Valkey to $(CLOUD_REALTIME_FORWARD_ADDRESS):$(CLOUD_VALKEY_LOCAL_PORT)"; \
	trap 'kill "$$nats_pid" >/dev/null 2>&1 || true' INT TERM EXIT; \
	$(KUBECTL) --context $(CLOUD_KUBE_CONTEXT) -n $(PLATFORM_NAMESPACE) port-forward --address $(CLOUD_REALTIME_FORWARD_ADDRESS) svc/$(CLOUD_VALKEY_SERVICE) $(CLOUD_VALKEY_LOCAL_PORT):6379

cloud-realtime-forward-start: gke-cloud-context
	@set -euo pipefail; \
	mkdir -p "$(dir $(CLOUD_NATS_FORWARD_PID_FILE))" "$(dir $(CLOUD_VALKEY_FORWARD_PID_FILE))"; \
	if ! docker image inspect "$(CLOUD_FORWARD_IMAGE)" >/dev/null 2>&1 || \
		! docker run --rm --platform "$(CLOUD_FORWARD_DOCKER_PLATFORM)" --entrypoint test "$(CLOUD_FORWARD_IMAGE)" -x /usr/local/bin/cloud-forward-supervisor.sh >/dev/null 2>&1; then \
		$(MAKE) --no-print-directory cloud-forward-image-build; \
	fi; \
	start_forward() { \
		label="$$1"; \
		service="$$2"; \
		local_port="$$3"; \
		remote_port="$$4"; \
		pid_file="$$5"; \
		container="$$6"; \
		launch_label="$$7"; \
		probe="$$8"; \
		if docker inspect "$$container" >/dev/null 2>&1; then \
			if [ "$$(docker inspect -f '{{.State.Running}}' "$$container")" = "true" ]; then \
				container_cmd="$$(docker inspect -f '{{json .Config.Cmd}}' "$$container")"; \
				if ! printf '%s\n' "$$container_cmd" | grep -q 'cloud-forward-supervisor.sh'; then \
					echo "$$label forward container is running without supervisor; restarting $$container"; \
					docker rm -f "$$container" >/dev/null; \
				elif scripts/cloud-forward-probe.sh "$$probe" "$$local_port"; then \
					echo "$$(docker inspect -f '{{.State.Pid}}' "$$container")" > "$$pid_file"; \
					echo "$$label forward container is running ($$container)"; \
					return 0; \
				else \
					echo "$$label forward container is running but 127.0.0.1:$$local_port failed protocol probe"; \
					docker logs --tail 80 "$$container" || true; \
					echo "restarting stale $$label forward container $$container"; \
					docker rm -f "$$container" >/dev/null; \
				fi; \
			else \
				docker rm "$$container" >/dev/null; \
			fi; \
		fi; \
		rm -f "$$pid_file"; \
		if [ "$$(uname -s)" = "Darwin" ] && command -v launchctl >/dev/null 2>&1; then \
			domain="gui/$$(id -u)"; \
			launchctl bootout "$$domain/$$launch_label" >/dev/null 2>&1 || true; \
			rm -f "$$HOME/Library/LaunchAgents/$$launch_label.plist"; \
		fi; \
		pids="$$(pgrep -f "port-forward --address .* svc/$$service $$local_port:$$remote_port" || true)"; \
		if [ -n "$$pids" ]; then \
			echo "$$pids" | xargs kill >/dev/null 2>&1 || true; \
		fi; \
		if scripts/cloud-forward-probe.sh "$$probe" "$$local_port"; then \
			echo "$$label forward port $$local_port is already reachable but not managed by Docker"; \
			return 0; \
		elif nc -z 127.0.0.1 "$$local_port" >/dev/null 2>&1; then \
			echo "$$label forward port $$local_port is reachable but failed protocol probe"; \
			return 1; \
		fi; \
		echo "starting $$label forward container $$container on $(CLOUD_REALTIME_FORWARD_ADDRESS):$$local_port"; \
		docker run -d \
			--name "$$container" \
			--restart "$(CLOUD_FORWARD_RESTART)" \
			--platform "$(CLOUD_FORWARD_DOCKER_PLATFORM)" \
			-p "$(CLOUD_REALTIME_FORWARD_ADDRESS):$$local_port:$$local_port" \
			-v "$(CLOUD_FORWARD_KUBECONFIG_DIR):/root/.kube:ro" \
			-v "$(CLOUD_FORWARD_GCLOUD_CONFIG_DIR):/root/.config/gcloud" \
			-e KUBECONFIG=/root/.kube/config \
			-e CLOUDSDK_CONFIG=/root/.config/gcloud \
			-e USE_GKE_GCLOUD_AUTH_PLUGIN=True \
			-e CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC="$(CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC)" \
			-e CLOUD_FORWARD_SUPERVISOR_RESTART_DELAY_SEC="$(CLOUD_FORWARD_SUPERVISOR_RESTART_DELAY_SEC)" \
			-e CLOUD_FORWARD_SUPERVISOR_STARTUP_GRACE_SEC="$(CLOUD_FORWARD_SUPERVISOR_STARTUP_GRACE_SEC)" \
			-e CLOUD_FORWARD_SUPERVISOR_FAILURE_THRESHOLD="$(CLOUD_FORWARD_SUPERVISOR_FAILURE_THRESHOLD)" \
			"$(CLOUD_FORWARD_IMAGE)" \
			cloud-forward-supervisor.sh "$$label" "$$probe" "$$local_port" \
			kubectl --context "$(CLOUD_KUBE_CONTEXT)" -n "$(PLATFORM_NAMESPACE)" port-forward --address 0.0.0.0 "svc/$$service" "$$local_port:$$remote_port"; \
		for _ in $$(seq 1 30); do \
			if scripts/cloud-forward-probe.sh "$$probe" "$$local_port"; then \
				echo "$$(docker inspect -f '{{.State.Pid}}' "$$container")" > "$$pid_file"; \
				echo "$$label forward ready on $(CLOUD_REALTIME_FORWARD_ADDRESS):$$local_port ($$container)"; \
				return 0; \
			fi; \
			sleep 1; \
		done; \
		echo "$$label forward did not become ready"; \
		docker logs --tail 80 "$$container" || true; \
		rm -f "$$pid_file"; \
		return 1; \
	}; \
	start_forward "cloud NATS" "$(CLOUD_NATS_SERVICE)" "$(CLOUD_NATS_LOCAL_PORT)" 4222 "$(CLOUD_NATS_FORWARD_PID_FILE)" "$(CLOUD_NATS_FORWARD_CONTAINER)" "$(CLOUD_NATS_FORWARD_LABEL)" nats; \
	start_forward "cloud Valkey" "$(CLOUD_VALKEY_SERVICE)" "$(CLOUD_VALKEY_LOCAL_PORT)" 6379 "$(CLOUD_VALKEY_FORWARD_PID_FILE)" "$(CLOUD_VALKEY_FORWARD_CONTAINER)" "$(CLOUD_VALKEY_FORWARD_LABEL)" valkey

cloud-realtime-forward-stop:
	@set -euo pipefail; \
	stop_forward() { \
		label="$$1"; \
		pid_file="$$2"; \
		pattern="$$3"; \
		container="$$4"; \
		launch_label="$$5"; \
		if docker inspect "$$container" >/dev/null 2>&1; then \
			echo "stopping $$label forward container $$container"; \
			docker rm -f "$$container" >/dev/null; \
		else \
			echo "$$label forward container is not running"; \
		fi; \
		if [ "$$(uname -s)" = "Darwin" ] && command -v launchctl >/dev/null 2>&1; then \
			domain="gui/$$(id -u)"; \
			launchctl bootout "$$domain/$$launch_label" >/dev/null 2>&1 || true; \
			rm -f "$$HOME/Library/LaunchAgents/$$launch_label.plist"; \
		fi; \
		pids="$$(pgrep -f "$$pattern" || true)"; \
		if [ -n "$$pids" ]; then \
			echo "$$pids" | xargs kill >/dev/null 2>&1 || true; \
			echo "stopped legacy $$label forward process(es)"; \
		fi; \
		rm -f "$$pid_file"; \
	}; \
	stop_forward "cloud NATS" "$(CLOUD_NATS_FORWARD_PID_FILE)" "port-forward --address .* svc/$(CLOUD_NATS_SERVICE) $(CLOUD_NATS_LOCAL_PORT):4222" "$(CLOUD_NATS_FORWARD_CONTAINER)" "$(CLOUD_NATS_FORWARD_LABEL)"; \
	stop_forward "cloud Valkey" "$(CLOUD_VALKEY_FORWARD_PID_FILE)" "port-forward --address .* svc/$(CLOUD_VALKEY_SERVICE) $(CLOUD_VALKEY_LOCAL_PORT):6379" "$(CLOUD_VALKEY_FORWARD_CONTAINER)" "$(CLOUD_VALKEY_FORWARD_LABEL)"

cloud-realtime-forward-status:
	@set -euo pipefail; \
	status_forward() { \
		label="$$1"; \
		container="$$2"; \
		local_port="$$3"; \
		probe="$$4"; \
		if docker inspect "$$container" >/dev/null 2>&1 && [ "$$(docker inspect -f '{{.State.Running}}' "$$container")" = "true" ]; then \
			echo "$$label forward container is running ($$container)"; \
		else \
			echo "$$label forward container is not running"; \
		fi; \
		if scripts/cloud-forward-probe.sh "$$probe" "$$local_port"; then \
			echo "127.0.0.1:$$local_port passed $$probe protocol probe"; \
		else \
			echo "127.0.0.1:$$local_port failed $$probe protocol probe"; \
			return 1; \
		fi; \
	}; \
	status_forward "cloud NATS" "$(CLOUD_NATS_FORWARD_CONTAINER)" "$(CLOUD_NATS_LOCAL_PORT)" nats; \
	status_forward "cloud Valkey" "$(CLOUD_VALKEY_FORWARD_CONTAINER)" "$(CLOUD_VALKEY_LOCAL_PORT)" valkey

cloud-db-env: gke-cloud-context
	@set -euo pipefail; \
	mkdir -p "$(dir $(CLOUD_DB_ENV_FILE))"; \
	user="$$( $(KUBECTL) -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.username}' | base64 -d )"; \
	pass="$$( $(KUBECTL) -n $(PLATFORM_NAMESPACE) get secret $(DB_MIGRATION_SECRET) -o jsonpath='{.data.password}' | base64 -d )"; \
	dsn="host=127.0.0.1 port=$(CLOUD_DB_LOCAL_PORT) user=$$user password=$$pass dbname=$(DB_MIGRATION_DB) sslmode=disable"; \
	quote_single() { printf "%s" "$$1" | sed "s/'/'\\\\''/g"; }; \
	qdsn="$$(quote_single "$$dsn")"; \
	umask 077; \
	{ \
		echo "# Generated by make cloud-db-env. Keep this file local."; \
		echo "# In another shell, run: make cloud-db-forward"; \
		printf "export CONTROL_PLANE_DB_DSN='%s'\n" "$$qdsn"; \
		printf "export ARCHIVE_MANIFEST_DB_DSN='%s'\n" "$$qdsn"; \
		printf "export ROLLUP_DB_DSN='%s'\n" "$$qdsn"; \
	} > "$(CLOUD_DB_ENV_FILE)"; \
	chmod 600 "$(CLOUD_DB_ENV_FILE)"; \
	echo "wrote $(CLOUD_DB_ENV_FILE) for 127.0.0.1:$(CLOUD_DB_LOCAL_PORT) without printing credentials"

cloud-status: gke-cloud-context
	@echo "cloud cluster: $(GKE_CLOUD_CLUSTER_NAME) ($(GKE_CLOUD_CLUSTER_REGION))"
	@echo "argocd applications (if installed)"
	@$(KUBECTL) -n $(ARGOCD_NAMESPACE) get applications.argoproj.io 2>/dev/null || echo "argocd applications not found"
	@echo "platform namespace ($(PLATFORM_NAMESPACE))"
	@$(KUBECTL) -n $(PLATFORM_NAMESPACE) get pods
	@$(KUBECTL) -n $(PLATFORM_NAMESPACE) get cluster,statefulset,endpoints 2>/dev/null || true
	@echo "services namespace ($(SERVICES_NAMESPACE))"
	@$(KUBECTL) -n $(SERVICES_NAMESPACE) get deploy,pdb,pods
	@echo "nodes"
	@$(KUBECTL) get nodes -L cloud.google.com/gke-nodepool,node.kubernetes.io/instance-type,topology.kubernetes.io/zone

web-stop:
	@pids="$$(lsof -tiTCP:$(WEB_PORT) -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then \
		echo "stopping web process(es) on port $(WEB_PORT): $$pids"; \
		kill $$pids 2>/dev/null || true; \
		sleep 1; \
		for pid in $$pids; do \
			if kill -0 $$pid 2>/dev/null; then \
				echo "force stopping pid $$pid"; \
				kill -9 $$pid 2>/dev/null || true; \
			fi; \
		done; \
	else \
		echo "no web process found on port $(WEB_PORT)"; \
	fi

web: web-stop
	@echo "starting web app on port $(WEB_PORT)"
	$(NPM) run -w apps/universal web -- --port $(WEB_PORT) --clear

clean:
	rm -rf bin .cache/go-build
