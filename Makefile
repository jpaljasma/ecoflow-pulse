SHELL := /bin/zsh

GO ?= go
NPM ?= npm
WEB_PORT ?= 8081
K3D ?= k3d
HELM ?= helm
KUBECTL ?= kubectl
DOCKER ?= docker
GCLOUD ?= gcloud
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
SERVICES_IMAGE_REPO ?= ecoflow-pulse/services
SERVICES_IMAGE_TAG ?= local
SERVICES_IMAGE ?= $(SERVICES_IMAGE_REPO):$(SERVICES_IMAGE_TAG)
SERVICES_IMAGE_DOCKERFILE ?= deploy/docker/pulse-services.Dockerfile
SERVICES_AUTO_BUILD_IMAGE ?= 1
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOFLAGS ?= -tags=moderncompress -mod=mod
LDFLAGS ?=
RACE_CRITICAL_PKGS ?= ./internal/ingestworker ./internal/ingestlease ./internal/projectionworker ./internal/archiveworker ./internal/telemetrybus ./cmd/ecoflow-grpc-api
RACE_STRESS_COUNT ?= 5
LOCAL_KUBECTL = $(KUBECTL) --context $(K3D_CONTEXT)
LOCAL_HELM = $(HELM) --kube-context $(K3D_CONTEXT)
PLATFORM_HELM_APPLY = $(LOCAL_HELM) upgrade --install $(PLATFORM_RELEASE) $(PLATFORM_CHART) --namespace $(PLATFORM_NAMESPACE) --create-namespace -f $(LOCAL_PLATFORM_VALUES)

export GOCACHE
export GOMODCACHE
export GOFLAGS

CMDS := $(patsubst cmd/%,%,$(wildcard cmd/*))

.PHONY: lint test test-race test-race-stress bench bench-ingestlease-integration test-archive-integration build smoke mqtt ingest-worker rollup-worker projection-worker archive-worker replay-cli gap-detector gap-repair-worker services-image-build-local services-image-import-local services-image-local-up k3d-up platform-up platform-wait services-up services-wait dev-up dev-down db-migrate-up-local db-migrate-down-local db-migrate-verify-local db-migrate-cycle-local db-migrate-e2e-local db-seed-dev-local auth-keycloak-verify-local gke-context gke-dev-guardrails gke-park gke-wake scale-down scale-up argocd-bootstrap-dev argocd-apps-dev argocd-wait-apps argocd-dev-up web web-stop clean

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
	@echo "running buf lint"
	@buf lint
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

build:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" bin
	@for cmd in $(CMDS); do \
		echo "building $$cmd"; \
		$(GO) build -ldflags "$(LDFLAGS)" -o "bin/$$cmd" "./cmd/$$cmd"; \
	done

services-image-build-local:
	@if ! command -v $(DOCKER) >/dev/null 2>&1; then \
		echo "$(DOCKER) not found. Install Docker Desktop first."; \
		exit 1; \
	fi
	@if ! $(DOCKER) info >/dev/null 2>&1; then \
		echo "Docker daemon is not running. Start Docker Desktop and retry."; \
		exit 1; \
	fi
	@echo "building services image $(SERVICES_IMAGE) from $(SERVICES_IMAGE_DOCKERFILE)"
	$(DOCKER) build -f $(SERVICES_IMAGE_DOCKERFILE) -t $(SERVICES_IMAGE) .

services-image-import-local:
	@if ! command -v $(K3D) >/dev/null 2>&1; then \
		echo "$(K3D) not found. Install k3d first."; \
		exit 1; \
	fi
	@echo "importing services image $(SERVICES_IMAGE) into k3d cluster $(K3D_CLUSTER_NAME)"
	$(K3D) image import $(SERVICES_IMAGE) -c $(K3D_CLUSTER_NAME)

services-image-local-up: services-image-build-local services-image-import-local

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

platform-up:
	@if ! command -v $(HELM) >/dev/null 2>&1; then \
		echo "$(HELM) not found. Install helm first."; \
		exit 1; \
	fi
	$(HELM) dependency update $(PLATFORM_CHART)
	@set -euo pipefail; \
	attempt=1; \
	max_attempts=$(HELM_RETRY_MAX); \
	delay=$(HELM_RETRY_DELAY_SEC); \
	while true; do \
		echo "applying platform chart (attempt $$attempt/$$max_attempts)"; \
		if $(PLATFORM_HELM_APPLY); then \
			break; \
		fi; \
		if [ $$attempt -ge $$max_attempts ]; then \
			echo "platform apply failed after $$max_attempts attempts"; \
			exit 1; \
		fi; \
		echo "platform apply failed, waiting for CNPG webhook/operator before retry"; \
		if command -v $(KUBECTL) >/dev/null 2>&1; then \
			if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cloudnative-pg >/dev/null 2>&1; then \
				$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cloudnative-pg --timeout=180s || true; \
			fi; \
			if $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get svc cnpg-webhook-service >/dev/null 2>&1; then \
				for _ in {1..36}; do \
					webhook_eps="$$( $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get endpoints cnpg-webhook-service -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true )"; \
					if [ -n "$$webhook_eps" ]; then \
						echo "CNPG webhook endpoints ready: $$webhook_eps"; \
						break; \
					fi; \
					sleep 5; \
				done; \
			fi; \
		fi; \
		sleep $$delay; \
		attempt=$$((attempt + 1)); \
		delay=$$((delay * 2)); \
		if [ $$delay -gt 30 ]; then delay=30; fi; \
	done
	@if command -v $(KUBECTL) >/dev/null 2>&1 && $(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cloudnative-pg >/dev/null 2>&1; then \
		echo "waiting for CloudNativePG operator to become ready"; \
		$(LOCAL_KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cloudnative-pg --timeout=180s; \
	fi
	@echo "running platform reconcile pass for CRD-backed resources"
	@set -euo pipefail; \
	attempt=1; \
	max_attempts=$(HELM_RETRY_MAX); \
	while true; do \
		echo "reconciling platform chart (attempt $$attempt/$$max_attempts)"; \
		if $(PLATFORM_HELM_APPLY); then \
			break; \
		fi; \
		if [ $$attempt -ge $$max_attempts ]; then \
			echo "platform reconcile failed after $$max_attempts attempts"; \
			exit 1; \
		fi; \
		sleep 5; \
		attempt=$$((attempt + 1)); \
	done

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
	wait_rollout deployment $(PLATFORM_RELEASE)-cloudnative-pg 180s; \
	wait_condition cluster.postgresql.cnpg.io $(PLATFORM_RELEASE)-core Ready $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(PLATFORM_RELEASE)-nats $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(PLATFORM_RELEASE)-valkey-node $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(PLATFORM_RELEASE)-keycloak $(WAIT_TIMEOUT); \
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

services-up:
	@if ! command -v $(HELM) >/dev/null 2>&1; then \
		echo "$(HELM) not found. Install helm first."; \
		exit 1; \
	fi
	@if [ "$(SERVICES_AUTO_BUILD_IMAGE)" = "1" ]; then \
		$(MAKE) services-image-local-up; \
	fi
	$(HELM) dependency update $(SERVICES_CHART)
	$(LOCAL_HELM) upgrade --install $(SERVICES_RELEASE) $(SERVICES_CHART) \
		--namespace $(SERVICES_NAMESPACE) --create-namespace \
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

dev-up: k3d-up platform-up platform-wait services-up services-wait

dev-down:
	@if command -v $(HELM) >/dev/null 2>&1; then \
		$(LOCAL_HELM) uninstall $(SERVICES_RELEASE) --namespace $(SERVICES_NAMESPACE) >/dev/null 2>&1 || true; \
		$(LOCAL_HELM) uninstall $(PLATFORM_RELEASE) --namespace $(PLATFORM_NAMESPACE) >/dev/null 2>&1 || true; \
	else \
		echo "$(HELM) not found; skipping helm uninstall"; \
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
