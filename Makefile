SHELL := /bin/zsh

GO ?= go
NPM ?= npm
WEB_PORT ?= 8081
K3D ?= k3d
HELM ?= helm
KUBECTL ?= kubectl
DOCKER ?= docker
K3D_CLUSTER_NAME ?= pulse-local
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
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOFLAGS ?= -tags=moderncompress -mod=mod
LDFLAGS ?=
PLATFORM_HELM_APPLY = $(HELM) upgrade --install $(PLATFORM_RELEASE) $(PLATFORM_CHART) --namespace $(PLATFORM_NAMESPACE) --create-namespace -f $(LOCAL_PLATFORM_VALUES)

export GOCACHE
export GOMODCACHE
export GOFLAGS

CMDS := $(patsubst cmd/%,%,$(wildcard cmd/*))

.PHONY: lint test bench build smoke mqtt k3d-up platform-up platform-wait services-up services-wait dev-up dev-down web web-stop clean

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

test:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) test ./...

bench:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	$(GO) test ./... -run '^$$' -bench . -benchmem -count=1

build:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" bin
	@for cmd in $(CMDS); do \
		echo "building $$cmd"; \
		$(GO) build -ldflags "$(LDFLAGS)" -o "bin/$$cmd" "./cmd/$$cmd"; \
	done

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
	$(KUBECTL) get nodes

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
			if $(KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cloudnative-pg >/dev/null 2>&1; then \
				$(KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cloudnative-pg --timeout=180s || true; \
			fi; \
			if $(KUBECTL) -n $(PLATFORM_NAMESPACE) get svc cnpg-webhook-service >/dev/null 2>&1; then \
				for _ in {1..36}; do \
					webhook_eps="$$( $(KUBECTL) -n $(PLATFORM_NAMESPACE) get endpoints cnpg-webhook-service -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true )"; \
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
	@if command -v $(KUBECTL) >/dev/null 2>&1 && $(KUBECTL) -n $(PLATFORM_NAMESPACE) get deploy $(PLATFORM_RELEASE)-cloudnative-pg >/dev/null 2>&1; then \
		echo "waiting for CloudNativePG operator to become ready"; \
		$(KUBECTL) -n $(PLATFORM_NAMESPACE) rollout status deploy/$(PLATFORM_RELEASE)-cloudnative-pg --timeout=180s; \
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
		if $(KUBECTL) -n "$$ns" get "$$kind" "$$name" >/dev/null 2>&1; then \
			echo "waiting for $$kind/$$name"; \
			$(KUBECTL) -n "$$ns" rollout status "$$kind/$$name" --timeout="$$timeout"; \
		fi; \
	}; \
	wait_condition() { \
		kind="$$1"; name="$$2"; condition="$$3"; timeout="$$4"; \
		if $(KUBECTL) -n "$$ns" get "$$kind" "$$name" >/dev/null 2>&1; then \
			echo "waiting for $$kind/$$name condition=$$condition"; \
			$(KUBECTL) -n "$$ns" wait --for=condition="$$condition" "$$kind/$$name" --timeout="$$timeout"; \
		fi; \
	}; \
	wait_rollout deployment $(PLATFORM_RELEASE)-cloudnative-pg 180s; \
	wait_condition cluster.postgresql.cnpg.io $(PLATFORM_RELEASE)-core Ready $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(PLATFORM_RELEASE)-nats $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(PLATFORM_RELEASE)-valkey-node $(WAIT_TIMEOUT); \
	wait_rollout statefulset $(PLATFORM_RELEASE)-keycloak $(WAIT_TIMEOUT); \
	wait_rollout deployment $(PLATFORM_RELEASE)-minio 300s; \
	echo "platform dependencies are ready"

services-up:
	@if ! command -v $(HELM) >/dev/null 2>&1; then \
		echo "$(HELM) not found. Install helm first."; \
		exit 1; \
	fi
	$(HELM) dependency update $(SERVICES_CHART)
	$(HELM) upgrade --install $(SERVICES_RELEASE) $(SERVICES_CHART) \
		--namespace $(SERVICES_NAMESPACE) --create-namespace \
		-f $(LOCAL_SERVICES_VALUES)

services-wait:
	@if ! command -v $(KUBECTL) >/dev/null 2>&1; then \
		echo "$(KUBECTL) not found. Install kubectl first."; \
		exit 1; \
	fi
	@set -euo pipefail; \
	ns="$(SERVICES_NAMESPACE)"; \
	if ! $(KUBECTL) get ns "$$ns" >/dev/null 2>&1; then \
		echo "namespace $$ns does not exist yet, skipping services wait"; \
		exit 0; \
	fi; \
	if [ -z "$$( $(KUBECTL) -n "$$ns" get pods -l app.kubernetes.io/instance=$(SERVICES_RELEASE) -o name 2>/dev/null )" ]; then \
		echo "no services workloads found for instance $(SERVICES_RELEASE) in $$ns"; \
		exit 0; \
	fi; \
	echo "waiting for services pods to become Ready"; \
	$(KUBECTL) -n "$$ns" wait --for=condition=Ready pod -l app.kubernetes.io/instance=$(SERVICES_RELEASE) --timeout=$(WAIT_TIMEOUT); \
	echo "services dependencies are ready"

dev-up: k3d-up platform-up platform-wait services-up services-wait

dev-down:
	@if command -v $(HELM) >/dev/null 2>&1; then \
		$(HELM) uninstall $(SERVICES_RELEASE) --namespace $(SERVICES_NAMESPACE) >/dev/null 2>&1 || true; \
		$(HELM) uninstall $(PLATFORM_RELEASE) --namespace $(PLATFORM_NAMESPACE) >/dev/null 2>&1 || true; \
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
