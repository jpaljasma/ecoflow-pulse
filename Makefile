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
DOCKER_BUILDKIT ?= 1
DOCKER_CONFIG_LOCAL ?= $(CURDIR)/.tmp/docker-noauth
GCLOUD ?= gcloud
LOCAL_PLATFORM_AUTO_TRUST_TLS ?= 1
K3D_CLUSTER_NAME ?= pulse-local
K3D_CONTEXT ?= k3d-$(K3D_CLUSTER_NAME)
K3D_CONFIG ?= deploy/tilt/k3d-config.yaml
PLATFORM_CHART ?= deploy/charts/pulse-platform
SERVICES_CHART ?= deploy/charts/pulse-services
LOCAL_PLATFORM_VALUES ?= deploy/env/local/values.platform.yaml
LOCAL_SERVICES_VALUES ?= deploy/env/local/values.services.yaml
PLATFORM_RELEASE ?= pulse-platform
SERVICES_RELEASE ?= pulse-services
PLATFORM_NAMESPACE ?= pulse-platform
SERVICES_NAMESPACE ?= pulse-services
DELETE_CLUSTER ?= 0
WAIT_TIMEOUT ?= 600s
HELM_RETRY_MAX ?= 6
HELM_RETRY_DELAY_SEC ?= 5
GKE_PROJECT_ID ?=
GKE_CLUSTER_NAME ?= pulse-dev
GKE_CLUSTER_ZONE ?= us-east1-b
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
ARGOCD_APPS_DIR ?= deploy/argocd/apps
ARGOCD_APPS ?= pulse-platform pulse-services
ARGOCD_APP_WAIT_ATTEMPTS ?= 60
ARGOCD_APP_WAIT_SLEEP_SEC ?= 10
DB_MIGRATIONS_DIR ?= deploy/db/migrations
DB_MIGRATION_NAMESPACE ?= pulse-platform
DB_MIGRATION_CLUSTER ?= pulse-platform-core
DB_MIGRATION_SECRET ?= pulse-platform-core-app
DB_MIGRATION_DB ?= pulse
PGROLL_LOCAL_PORT ?= 15433
PGROLL_PLAN ?=
DB_SEED_LOCAL_PORT ?= 15432
DB_SEED_USER_SUBJECT ?= jpaljasma@gmail.com
DB_SEED_USER_EMAIL ?= jpaljasma@gmail.com
DB_SEED_SERIALS ?= R351ZABAPH331057,Y711ZABA9H2P0294
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
SERVICES_IMAGE_DOCKERFILE ?= deploy/docker/pulse-services.Dockerfile
PLATFORM_APP_IMAGE_REPO ?= ecoflow-pulse/pulse-platform
PLATFORM_APP_IMAGE_TAG ?= local
PLATFORM_APP_IMAGE ?= $(PLATFORM_APP_IMAGE_REPO):$(PLATFORM_APP_IMAGE_TAG)
PLATFORM_APP_IMAGE_DOCKERFILE ?= deploy/docker/pulse-platform.Dockerfile
REALTIME_GATEWAY_IMAGE_REPO ?= ecoflow-pulse/pulse-realtime-gateway
REALTIME_GATEWAY_IMAGE_TAG ?= local
REALTIME_GATEWAY_IMAGE ?= $(REALTIME_GATEWAY_IMAGE_REPO):$(REALTIME_GATEWAY_IMAGE_TAG)
REALTIME_GATEWAY_IMAGE_DOCKERFILE ?= deploy/docker/pulse-realtime-gateway.Dockerfile
SERVICES_AUTO_BUILD_IMAGE ?= 1
DEV_DEPLOY_HELM ?= auto
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOFLAGS ?= -tags=moderncompress -mod=mod
LDFLAGS ?=
RACE_CRITICAL_PKGS ?= ./internal/ingestworker ./internal/ingestlease ./internal/projectionworker ./internal/archiveworker ./internal/telemetrybus ./cmd/ecoflow-grpc-api
RACE_STRESS_COUNT ?= 5
LOCAL_KUBECTL = $(KUBECTL) --context $(K3D_CONTEXT)
LOCAL_HELM = $(HELM) --kube-context $(K3D_CONTEXT)
LOCAL_HELM_UPGRADE_FLAGS ?= --server-side=true --force-conflicts
PLATFORM_HELM_APPLY = $(LOCAL_HELM) upgrade --install $(PLATFORM_RELEASE) $(PLATFORM_CHART) --namespace $(PLATFORM_NAMESPACE) --create-namespace $(LOCAL_HELM_UPGRADE_FLAGS) -f $(LOCAL_PLATFORM_VALUES)
LOCAL_PLATFORM_MANIFEST ?= $(CURDIR)/.tmp/pulse-platform.rendered.yaml
K6_SCRIPT ?= load/k6/main.js
K6_API_BASE_URL ?= http://127.0.0.1
K6_WS_URL ?= ws://127.0.0.1/ws
K6_USER_SUBJECT ?= jpaljasma@gmail.com
K6_DURATION ?= 1m
K6_INGEST_RATE ?= 20
K6_INGEST_PRE_ALLOCATED_VUS ?= 8
K6_INGEST_MAX_VUS ?= 32
K6_QUERY_RATE ?= 1
K6_QUERY_PRE_ALLOCATED_VUS ?= 2
K6_QUERY_MAX_VUS ?= 8
K6_WS_VUS ?= 20
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

.PHONY: lint test test-race test-race-stress bench bench-ingestlease-integration test-archive-integration test-pipeline-integration test-proto-contract test-db-migrations-ci test-web-e2e test-mobile-e2e test-load-k6 build smoke mqtt ingest-worker inference-worker rollup-worker projection-worker archive-worker replay-cli gap-detector gap-repair-worker docker-local-ready k3d-local-ready helm-local-ready chart-deps-local services-image-build-local services-image-import-local services-image-local-up platform-app-image-build-local realtime-gateway-image-build-local public-images-build-local public-images-import-local public-images-local-up k3d-up platform-up platform-wait local-trust-platform-tls local-trust-platform-tls-system services-up services-wait dev-up dev-deploy dev-regen-data dev-down db-migrate-up-local db-migrate-down-local db-migrate-verify-local db-migrate-cycle-local db-migrate-e2e-local db-seed-dev-local pgroll-init-local pgroll-status-local pgroll-start-local pgroll-complete-local pgroll-rollback-local dr-backup-local dr-restore-local dr-drill-local auth-keycloak-verify-local gke-context gke-dev-guardrails gke-park gke-wake scale-down scale-up argocd-bootstrap-dev argocd-apps-dev argocd-wait-apps argocd-dev-up web web-stop clean

lint:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) fmt ./...
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
	@echo "running migration cycle + e2e validation suite with Testcontainers"
	$(GO) test ./internal/integrationtest -run TestMigrationsCycleAndE2E -count=1 -v

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
	@if [ ! -f "$(DOCKER_CONFIG_LOCAL)/config.json" ]; then \
		printf '{\n  "auths": {}\n}\n' > "$(DOCKER_CONFIG_LOCAL)/config.json"; \
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
		vendored_count="$$(find "$$charts_dir" -mindepth 1 -maxdepth 1 -name '*.tgz' 2>/dev/null | wc -l | tr -d '[:space:]')"; \
		if [ "$$vendored_count" != "$$dep_count" ]; then \
			echo "vendored chart packages missing for $$chart; running helm dependency build --skip-refresh"; \
			$(HELM) dependency build --skip-refresh "$$chart"; \
			exit 0; \
		fi; \
		echo "chart dependencies already vendored for $$chart; skipping helm dependency build"

services-image-build-local: docker-local-ready
	@echo "building services image $(SERVICES_IMAGE) from $(SERVICES_IMAGE_DOCKERFILE)"
	@if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
		DOCKER_BUILDKIT=1 $(DOCKER) build -f $(SERVICES_IMAGE_DOCKERFILE) -t $(SERVICES_IMAGE) .; \
	else \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build -f $(SERVICES_IMAGE_DOCKERFILE) -t $(SERVICES_IMAGE) .; \
	fi

services-image-import-local: k3d-local-ready
	@echo "importing services image $(SERVICES_IMAGE) into k3d cluster $(K3D_CLUSTER_NAME)"
	$(K3D) image import $(SERVICES_IMAGE) -c $(K3D_CLUSTER_NAME)

services-image-local-up:
	@$(MAKE) --no-print-directory services-image-build-local
	@$(MAKE) --no-print-directory services-image-import-local

platform-app-image-build-local: docker-local-ready
	@echo "building public app image $(PLATFORM_APP_IMAGE) from $(PLATFORM_APP_IMAGE_DOCKERFILE)"
	@if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
		DOCKER_BUILDKIT=1 $(DOCKER) build -f $(PLATFORM_APP_IMAGE_DOCKERFILE) -t $(PLATFORM_APP_IMAGE) .; \
	else \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build -f $(PLATFORM_APP_IMAGE_DOCKERFILE) -t $(PLATFORM_APP_IMAGE) .; \
	fi

realtime-gateway-image-build-local: docker-local-ready
	@echo "building realtime gateway image $(REALTIME_GATEWAY_IMAGE) from $(REALTIME_GATEWAY_IMAGE_DOCKERFILE)"
	@if [ "$(DOCKER_BUILDKIT)" = "1" ]; then \
		DOCKER_BUILDKIT=1 $(DOCKER) build -f $(REALTIME_GATEWAY_IMAGE_DOCKERFILE) -t $(REALTIME_GATEWAY_IMAGE) .; \
	else \
		DOCKER_CONFIG="$(DOCKER_CONFIG_LOCAL)" $(DOCKER) build -f $(REALTIME_GATEWAY_IMAGE_DOCKERFILE) -t $(REALTIME_GATEWAY_IMAGE) .; \
	fi

public-images-build-local:
	@$(MAKE) --no-print-directory -j2 platform-app-image-build-local realtime-gateway-image-build-local

public-images-import-local: k3d-local-ready
	@echo "importing public app and realtime gateway images into k3d cluster $(K3D_CLUSTER_NAME)"
	$(K3D) image import $(PLATFORM_APP_IMAGE) $(REALTIME_GATEWAY_IMAGE) -c $(K3D_CLUSTER_NAME)

public-images-local-up:
	@$(MAKE) --no-print-directory public-images-build-local
	@$(MAKE) --no-print-directory public-images-import-local

smoke:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) run ./cmd/ecoflow-smoke

mqtt:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	@set +e; \
	interrupted=0; \
	trap 'interrupted=1' INT TERM; \
	$(GO) run ./cmd/ecoflow-mqtt-sub; \
	code=$$?; \
	if [ $$interrupted -eq 1 ] || [ $$code -eq 130 ]; then \
		echo "mqtt subscriber stopped"; \
		exit 0; \
	fi; \
	exit $$code

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
	else \
		echo "creating k3d cluster '$(K3D_CLUSTER_NAME)' from $(K3D_CONFIG)"; \
		$(K3D) cluster create --config $(K3D_CONFIG); \
	fi
	$(KUBECTL) config use-context $(K3D_CONTEXT)
	$(LOCAL_KUBECTL) get nodes

platform-up: helm-local-ready
	@$(MAKE) --no-print-directory chart-deps-local CHART=$(PLATFORM_CHART)
	@set -euo pipefail; \
		ns="$(PLATFORM_NAMESPACE)"; \
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
		echo "installing platform release via Helm"; \
		$(PLATFORM_HELM_APPLY) $$keycloak_first_pass_flags; \
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
		$(PLATFORM_HELM_APPLY); \
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
	wait_job_complete() { \
		name="$$1"; timeout="$$2"; \
		if $(LOCAL_KUBECTL) -n "$$ns" get job "$$name" >/dev/null 2>&1; then \
			echo "waiting for job/$$name condition=complete"; \
			$(LOCAL_KUBECTL) -n "$$ns" wait --for=condition=complete job/"$$name" --timeout="$$timeout"; \
		fi; \
	}; \
	wait_rollout deployment $(PLATFORM_RELEASE)-cloudnative-pg 180s; \
	wait_condition cluster.postgresql.cnpg.io $(PLATFORM_RELEASE)-core Ready $(WAIT_TIMEOUT); \
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

services-up: helm-local-ready
	@if [ "$(SERVICES_AUTO_BUILD_IMAGE)" = "1" ]; then \
		$(MAKE) services-image-local-up; \
	fi
	@$(MAKE) --no-print-directory chart-deps-local CHART=$(SERVICES_CHART)
	$(LOCAL_HELM) upgrade --install $(SERVICES_RELEASE) $(SERVICES_CHART) \
		--namespace $(SERVICES_NAMESPACE) --create-namespace \
		$(LOCAL_HELM_UPGRADE_FLAGS) \
		-f $(LOCAL_SERVICES_VALUES)

services-wait:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ns="$(SERVICES_NAMESPACE)"; \
	if ! $(LOCAL_KUBECTL) get ns "$$ns" >/dev/null 2>&1; then \
		echo "namespace $$ns does not exist yet, skipping services wait"; \
		exit 0; \
	fi; \
	if [ -z "$$( $(LOCAL_KUBECTL) -n "$$ns" get pods -l app.kubernetes.io/instance=$(SERVICES_RELEASE) -o name 2>/dev/null )" ]; then \
		echo "no services workloads found for instance $(SERVICES_RELEASE) in $$ns"; \
		exit 0; \
	fi; \
	echo "waiting for services pods to become Ready"; \
	$(LOCAL_KUBECTL) -n "$$ns" wait --for=condition=Ready pod -l app.kubernetes.io/instance=$(SERVICES_RELEASE) --timeout=$(WAIT_TIMEOUT); \
	echo "services dependencies are ready"

dev-up: k3d-up public-images-local-up platform-up platform-wait services-up services-wait

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
# dev-deploy defaults to a Helm fast path (`DEV_DEPLOY_HELM=auto`) that skips Helm re-apply unless the
# local chart/values files changed or the release is missing. Use `DEV_DEPLOY_HELM=always` to force full Helm apply.
# The rollout restart calls are important because the images use the same :local tag with IfNotPresent, so importing alone will not replace already-running pods.
dev-deploy:
	@set -euo pipefail; \
		helm_mode="$(DEV_DEPLOY_HELM)"; \
		platform_apply=0; \
		services_apply=0; \
		case "$$helm_mode" in \
			always|1|true) \
				platform_apply=1; \
				services_apply=1; \
				;; \
			never|0|false) \
				;; \
			auto) \
				if ! $(LOCAL_HELM) status $(PLATFORM_RELEASE) --namespace $(PLATFORM_NAMESPACE) >/dev/null 2>&1; then \
					platform_apply=1; \
				fi; \
				if ! $(LOCAL_HELM) status $(SERVICES_RELEASE) --namespace $(SERVICES_NAMESPACE) >/dev/null 2>&1; then \
					services_apply=1; \
				fi; \
				if [ -n "$$(git status --porcelain --untracked-files=all -- $(PLATFORM_CHART) $(LOCAL_PLATFORM_VALUES))" ]; then \
					platform_apply=1; \
				fi; \
				if [ -n "$$(git status --porcelain --untracked-files=all -- $(SERVICES_CHART) $(LOCAL_SERVICES_VALUES))" ]; then \
					services_apply=1; \
				fi; \
				if [ "$$platform_apply" = "0" ]; then \
					if ! $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy/pulse-platform-realtime-gateway >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy/pulse-platform-public-app >/dev/null 2>&1; then \
						platform_apply=1; \
					fi; \
				fi; \
				if [ "$$services_apply" = "0" ]; then \
					if ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-inference >/dev/null 2>&1 || \
					   ! $(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy/pulse-services-go-grpc-api >/dev/null 2>&1 || \
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
		$(MAKE) --no-print-directory public-images-local-up; \
		if [ "$$platform_apply" = "1" ]; then \
			echo "applying platform Helm release"; \
			$(MAKE) --no-print-directory platform-up; \
		else \
			echo "skipping platform Helm apply (set DEV_DEPLOY_HELM=always to force)"; \
		fi; \
		if [ "$(SERVICES_AUTO_BUILD_IMAGE)" = "1" ]; then \
			$(MAKE) --no-print-directory services-image-local-up; \
		fi; \
		if [ "$$services_apply" = "1" ]; then \
			echo "applying services Helm release"; \
			$(MAKE) --no-print-directory SERVICES_AUTO_BUILD_IMAGE=0 services-up; \
		else \
			echo "skipping services Helm apply (set DEV_DEPLOY_HELM=always to force)"; \
		fi
	@set -euo pipefail; \
		restart_and_wait_if_exists() { \
			ns="$$1"; \
			name="$$2"; \
			if $(LOCAL_KUBECTL) -n "$$ns" get deploy/"$$name" >/dev/null 2>&1; then \
				echo "restarting $$ns/$$name"; \
				$(LOCAL_KUBECTL) -n "$$ns" rollout restart deploy/"$$name"; \
				echo "waiting for $$ns/$$name"; \
				$(LOCAL_KUBECTL) -n "$$ns" rollout status deploy/"$$name" --timeout=300s; \
			else \
				echo "skipping missing deployment $$ns/$$name"; \
			fi; \
		}; \
		echo "restarting updated local deployments"; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-inference; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-grpc-api; \
		restart_and_wait_if_exists $(SERVICES_NAMESPACE) pulse-services-go-rollup; \
		restart_and_wait_if_exists $(PLATFORM_NAMESPACE) pulse-platform-realtime-gateway; \
		restart_and_wait_if_exists $(PLATFORM_NAMESPACE) pulse-platform-public-app
	@echo "showing deployment state and recent realtime gateway logs"
	$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy
	$(LOCAL_KUBECTL) -n $(SERVICES_NAMESPACE) get deploy
	$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) logs deploy/pulse-platform-realtime-gateway --since=5m

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
	sh -c "$(GO) run ./cmd/ecoflow-rollup-rebuild -from '$$from' -to '$$to' -max-objects $(REGEN_MAX_OBJECTS) $$device_args"; \
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
	files="$$(find $(DB_MIGRATIONS_DIR) -maxdepth 1 -type f -name '*.up.sql' | sort)"; \
	if [ -z "$$files" ]; then \
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
	while IFS= read -r f; do \
		[ -n "$$f" ] || continue; \
		echo "applying $$f"; \
		cat "$$f" | kubectl --context "$$ctx" -n "$$ns" exec -i "$$primary" -- env PGPASSWORD="$$pass" psql -h "$$cluster-rw" -U "$$user" -d "$$db" -v ON_ERROR_STOP=1 -f -; \
	done <<< "$$files"

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
	if [ -f .env ]; then \
		set -a; source ./.env; set +a; \
	fi; \
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
	ECOFLOW_DEV_USER_SUBJECT="$(DB_SEED_USER_SUBJECT)" \
	ECOFLOW_DEV_USER_EMAIL="$(DB_SEED_USER_EMAIL)" \
	ECOFLOW_DEV_SEED_SNS="$(DB_SEED_SERIALS)" \
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
