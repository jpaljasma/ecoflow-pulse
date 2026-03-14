# Auth Profile Task Board

Status: `PROGRESS`  
Plan: `.codex/plans/auth-profile-ralph-loop.md`  
ADR: `docs/architecture/adr/ADR-0020-authenticated-entry-routing-profile-and-device-authorization.md`  
Source plan: `/Users/jpaljasma/Downloads/ecoflow-pulse-auth-profile-codex-plan.md`  
Branch: `codex/auth-profile-entry-routing`  
Base commit: `5a0912a`

## Assumptions

- Google login is the only visible provider in this increment.
- Facebook remains hidden/disabled by env and is not removed from Keycloak bootstrap.
- `/profile` is standalone and reachable from app chrome/settings, not a bottom tab.
- Pulse profile edits override later IdP claim refreshes.
- Weather location consent/timezone preferences are part of this milestone.
- Trusted social-profile fields come from brokered Keycloak OIDC claims, not direct Google profile fetches.
- Stable identity remains `users.keycloak_subject`; social email is persisted but not used as the primary key.

## Workstreams

| Status | Owner | Workstream | Dependency | Latest validation |
|---|---|---|---|---|
| DONE | `project-manager` | Import ADR-0020, create plan/task/memory scaffolding, mark architecture task `PROGRESS` | none | tracking files created |
| DONE | `backend-go` | Control-plane schema + current-user provisioning/profile RPCs | tracking | `buf generate` + `go test ./internal/controlplane ./internal/grpcmw ./cmd/ecoflow-grpc-api -count=1` |
| DONE | `backend-go` | Device/realtime authz hardening for unauthorized `401` semantics and role-failure `403` semantics | current-user RPC shape | `npm run -w apps/pulse-platform test -- device_routes.test.ts history_routes.test.ts me_routes.test.ts` + `npm run -w apps/pulse-realtime-gateway test -- gateway.test.ts` |
| DONE | `bff-node` | `/api/v1/me` bootstrap/update endpoints | Go RPCs | `npm run -w apps/pulse-platform typecheck` + `npm run -w apps/pulse-platform test -- me_routes.test.ts device_client.test.ts` |
| DONE | `frontend-universal` | Welcome/login/protected-route/logout flow | BFF bootstrap endpoint | `npm run -w apps/universal test -- src/features/auth/useReturnTo.test.ts` + `npm run -w apps/universal typecheck` |
| DONE | `frontend-universal` | Standalone `/profile` page | BFF bootstrap/update endpoints | `npm run -w apps/universal typecheck` |
| DONE | `backend-go` + `bff-node` | Auth/profile observability metrics + Grafana dashboards | core auth/profile behavior stable | `npm run -w apps/pulse-platform typecheck` + `npm run -w apps/pulse-platform test -- me_routes.test.ts device_routes.test.ts history_routes.test.ts` + `npm run -w apps/pulse-realtime-gateway typecheck` + `npm run -w apps/pulse-realtime-gateway test -- gateway.test.ts` + `helm lint deploy/charts/pulse-platform -f deploy/env/local/values.platform.yaml` + `helm lint deploy/charts/pulse-platform -f deploy/env/dev/values.platform.yaml` |
| DONE | `qa` | Redirect/authz/profile/logout regression coverage | per workstream | `npm run -w apps/pulse-platform test -- device_routes.test.ts history_routes.test.ts me_routes.test.ts` + `npm run -w apps/pulse-realtime-gateway test -- gateway.test.ts` + `npm run -w apps/universal test -- src/features/auth/useReturnTo.test.ts src/features/auth/useRequireAuth.test.ts src/features/auth/LogoutButton.test.ts src/features/profile/hooks.test.ts src/features/telemetry/store.test.ts src/shared/config/env.test.ts` |
| TODO | `product-review` | Final UX review and local walkthrough | QA green | pending |

## Decisions

- 2026-03-13: Implement ADR-0020 as a new M1 follow-up off updated `main`, not as an extension of an existing UI-only PR.
- 2026-03-13: Keep the Google-only visible login surface for now while leaving Facebook disabled by env instead of deleting provider bootstrap support.
- 2026-03-13: Treat current-user bootstrap/profile as a first-class control-plane capability rather than stuffing profile state into the existing device routes.
- 2026-03-13: Keep role-specific work recorded in project-specific `.codex/memories/<agent>/auth-profile-entry-routing.md` files to preserve previous loop history.
- 2026-03-13: Treat auth/profile observability as in-scope implementation work, not post-merge cleanup; ship metrics and dashboards with the feature.
- 2026-03-13: Persist enough social-profile data for correct bootstrap and profile rendering: `email`, `email_verified`, `display_name`, `given_name`, `family_name`, Google `picture` mapped to `avatar_url`, and `locale` when present.
- 2026-03-13: Use `401` for unauthorized HTTP auth failures and keep `403` for authenticated role failures in this milestone.
- 2026-03-13: Post-login routing resolves in this order: sanitized `returnTo`, then `/devices` when the user owns devices, otherwise the templated authenticated `/onboarding` wizard shell.
- 2026-03-13: Protected universal routes now gate through a shared `useRequireAuth()` flow; invalid sessions redirect to `/login` while `/onboarding` remains first-time-only and immediately returns users with devices to `/devices`.
- 2026-03-13: Capture local cold-start recovery hardening as a follow-up task: dependency health gates, startup retry/backoff, and a clearer local recovery path after Docker/k3d restarts.
- 2026-03-14: Profile avatar backfill should use a background authenticated Keycloak `userinfo` refresh when `avatar_url` is still empty, while the UI keeps the first-letter fallback when no avatar exists upstream.
- 2026-03-14: Homepage/profile/history queries should keep the previous successful payload visible during rollout-time refetches instead of flashing empty state.

## Blockers

- None currently. Scope questions on provider visibility, profile ownership, consent behavior, and public-edge `401` semantics were resolved before coding.

## Next Actions

1. Package the current branch into a focused PR once the local acceptance slice is complete.
2. Finish the remaining cold-start dependency gating and graceful request/websocket drain acceptance evidence.
3. Decide whether avatar backfill should remain profile-triggered only or also run during current-user bootstrap when broker claims are sparse.
4. Follow with a dedicated local-platform resilience task covering deeper recovery diagnostics after Docker/Kubernetes restarts.
