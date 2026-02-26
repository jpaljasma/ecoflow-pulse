# Reference: Local Dev Prerequisites

This page tracks required local tools for development workflows.

## Required Tooling

| Tool | Why it is needed | Used by |
|---|---|---|
| Go | Build/test backend services and CLIs | `go test`, `make test`, `make mqtt` |
| Node.js + npm | Build/test universal app | `npm run -w apps/universal ...`, `make web` |
| Docker Desktop (daemon running) | Runtime backend for k3d clusters | `make k3d-up`, `make dev-up` |
| Helm | Kubernetes chart lint/install and dependency resolution | `make platform-up`, `make services-up` |
| k3d | Local Kubernetes cluster bringup | `make k3d-up`, `make dev-up` |
| kubectl | Kubernetes cluster inspection and validation | `make k3d-up`, local debugging |
| buf | Protobuf + gRPC code generation and linting | `buf generate`, `make lint` |
| markdownlint-cli | Markdown quality checks | `make lint`, docs updates |
| Google Cloud SDK (`gcloud`) | GKE dev kubecontext and node-pool scaling | `make gke-context`, `make gke-park`, `make gke-wake`, `make argocd-dev-up` |

Install markdownlint-cli (non-Node) with Homebrew:

```bash
brew install markdownlint-cli
```

**golangci-lint** is a fast linters runner for Go

It runs linters in parallel, uses caching, supports YAML configuration, integrates with all major IDEs, and includes over a hundred linters.

```bash
brew install golangci-cli
```

## Recently Observed Missing Requirements

| Date | Missing tool | Context |
|---|---|---|
| 2026-02-20 | `k3d` | `make dev-up` failed at `k3d-up` before cluster creation |
| 2026-02-20 | Docker daemon not running | `make dev-up` failed to create k3d cluster (`Cannot connect to the Docker daemon`) |
| 2026-02-20 | k3d worker node not Ready after Docker restart | `pulse-platform` pods stuck `Terminating/Pending`; recovered by restarting affected node container (for example `docker restart k3d-pulse-local-agent-0`) |
