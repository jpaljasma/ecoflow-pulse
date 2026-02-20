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

## Recently Observed Missing Requirements

| Date | Missing tool | Context |
|---|---|---|
| 2026-02-20 | `k3d` | `make dev-up` failed at `k3d-up` before cluster creation |
| 2026-02-20 | Docker daemon not running | `make dev-up` failed to create k3d cluster (`Cannot connect to the Docker daemon`) |
| 2026-02-20 | k3d worker node not Ready after Docker restart | `pulse-platform` pods stuck `Terminating/Pending`; recovered by restarting affected node container (for example `docker restart k3d-pulse-local-agent-0`) |
