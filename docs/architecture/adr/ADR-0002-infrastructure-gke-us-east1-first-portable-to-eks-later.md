# ADR-0002: Infrastructure — GKE us-east1 First, Portable to EKS Later

**Status:** Accepted  
**Date:** 2026-02-20

## Context
We need Kubernetes deployment to support HA patterns and scale. The initial operator team is one person, so managed Kubernetes with lower operational burden is preferred. Region choice should be reasonable for US data residency while being closer to EU traffic than central US.

## Decision
- Deploy initially to **Google Kubernetes Engine (GKE)**.
- Choose region **us-east1**.
- Keep ingress and platform choices portable (e.g., ingress-nginx, cert-manager, Helm/Kustomize, Argo CD) so EKS migration remains feasible.

## Consequences
### Positive
- Lower ops overhead for a solo team
- Strong default integrations with GCP storage and secret manager
- Regional clusters provide zone resilience

### Tradeoffs
- Some cloud-specific operational details (IAM, storage classes)
- Need to ensure portability by avoiding heavy GKE-only features

### Follow-ups
- Use GitOps (Argo CD) and environment overlays
- Define a minimal portability checklist (ingress, storage abstractions, secrets)
