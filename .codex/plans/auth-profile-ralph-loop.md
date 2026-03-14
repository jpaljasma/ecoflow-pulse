# Auth Profile Ralph-Loop Plan

Status: Active implementation plan  
Last updated: 2026-03-13

## Goal

Implement ADR-0020 end to end:

- public welcome page
- dedicated login page
- protected-route redirect-back behavior
- current-user bootstrap/profile APIs
- first-sign-in provisioning
- standalone profile page
- hard device authorization semantics across list/detail/history/realtime
- logout that tears down live state cleanly
- OTEL-compatible auth/profile metrics plus useful Grafana dashboards in local/dev

## Source Of Truth

Use these inputs in this order:

1. `docs/architecture/adr/ADR-0020-authenticated-entry-routing-profile-and-device-authorization.md`
2. `/Users/jpaljasma/Downloads/ecoflow-pulse-auth-profile-codex-plan.md`
3. `AGENTS.md`
4. `docs/architecture/README.md`
5. Relevant architecture configs:
   - `docs/architecture/config-05-auth-security.md`
   - `docs/architecture/config-02-local-k3d-simple.md`
   - `docs/architecture/config-03-platform-ha-defaults.md`

## Locked Decisions For This Increment

- Google login is active.
- Facebook stays configured infrastructure-side but hidden and disabled by env.
- `/profile` is a standalone full feature route, not a bottom tab.
- Pulse-owned profile edits override future social-profile claim refreshes.
- Weather location consent/timezone preferences ship in this milestone.
- Trusted social-profile fields are hydrated from brokered Keycloak OIDC claims, not direct Google profile API calls in the app.
- Stable app identity remains `users.keycloak_subject`; social email is persisted but never used as the primary identity key.
- Daily implementation/validation stays local-first on k3d; GKE dev is only for OAuth/redirect validation when needed.

## Agent Roster

Use these conceptual agents and keep their work recorded separately:

| Agent | Scope | Memory file |
|---|---|---|
| `project-manager` | sequencing, task board, scope control, cost/quality reporting | `.codex/memories/project-manager/auth-profile-entry-routing.md` |
| `backend-go` | schema, control-plane RPCs, provisioning, authz hardening, WS authz | `.codex/memories/backend-go/auth-profile-entry-routing.md` |
| `bff-node` | `/api/v1/me`, bootstrap DTOs, request validation, auth boundary mapping | `.codex/memories/bff-node/auth-profile-entry-routing.md` |
| `frontend-universal` | welcome/login/profile routes, protected guards, redirect-back, logout UX | `.codex/memories/frontend-universal/auth-profile-entry-routing.md` |
| `qa` | redirect/authz/profile regression tests and review gates | `.codex/memories/qa/auth-profile-entry-routing.md` |
| `product-review` | final UX/spec review for login/profile/onboarding/logout | `.codex/memories/product-review/auth-profile-entry-routing.md` |

## Progress Output Format

Use concise progress updates:

```text
Progress
- done:
- in flight:
- next:
- tests:
- blockers:
- cost note:
```

## Work Breakdown

### Phase 0: Tracking + Contract

Owner: `project-manager`

- Import ADR-0020 into the repo and index it.
- Mark the architecture task `PROGRESS` with checkbox sub-steps.
- Create `.codex` plan/task/memory files for this project.
- Record the branch name and base commit in the task board.

### Phase 1: Backend contract + provisioning path

Owner: `backend-go`

- Extend the control-plane schema for profile/preferences.
- Extend the user model for trusted social-profile hydration (`email_verified`, `given_name`, `family_name`, `avatar_url`, `locale` as needed).
- Add first-sign-in current-user provisioning/upsert behavior keyed by trusted auth subject.
- Add gRPC current-user bootstrap/read/update RPCs.
- Harden unauthorized device reads to `401` at the HTTP boundary and role failures to `403`.
- Recheck realtime authorization path for per-device subscriptions.

### Phase 2: BFF bootstrap/profile path

Owner: `bff-node`

- Add `GET /api/v1/me`.
- Add `PATCH /api/v1/me`.
- Reuse JWT validation and forward the bearer token to Go.
- Keep validation strict for timezone/location payloads.

### Phase 3: Universal auth/product flow

Owner: `frontend-universal`

- Replace `/` redirect with public welcome page.
- Add `/login`.
- Add protected-route handling with sanitized `returnTo`.
- Add post-login bootstrap + redirect-back flow.
- Add standalone `/profile` with display name, readonly email, timezone, and weather location consent.
- Add logout to app chrome and Settings.

### Phase 4: QA and product review

Owner: `qa`, `product-review`

- Redirect-back correctness
- Unauthorized device access behavior
- Profile update and revoke-location behavior
- Logout teardown and public-entry return
- Local k3d walkthrough and final PR evidence

### Phase 5: Observability

Owner: `backend-go`, `bff-node`, `qa`

- Emit auth/profile metrics from the public Node layer and Go control-plane layer.
- Cover at least:
  - login/bootstrap success/failure
  - protected-route redirects
  - first-sign-in provisioning
  - profile update success/failure
  - unauthorized device reads
  - websocket auth rejections
- Add Grafana dashboards under the existing platform dashboard templates.
- Validate the panels against local k3d Prometheus/Grafana before PR closeout.
