## Summary

- add the end-to-end `/energy` dashboard across Go gRPC, Node REST, and the Expo universal app
- add auth-aware fleet and per-device Energy deep links from fleet, device-card, and device-detail surfaces
- add Energy QA coverage, responsive PV envelope rendering, battery/power/energy chart polish, and final docs/task closeout

## Why

- ship the Energy dashboard planned under `.codex/plans/energy-dashboard-ralph-loop.md`
- provide a truthful local-calendar energy view for solar, load, battery movement, PV envelope, and estimated value
- validate the feature on local k3d before merge and capture the acceptance evidence in repo docs

## Validation

- `make lint`
- `go test ./internal/energydashboard ./internal/telemetryquery ./internal/grpcmw ./cmd/ecoflow-grpc-api -count=1`
- `npm run -w apps/universal e2e:web -- devices.spec.ts energy.spec.ts`
- `make dev-deploy`

## Notes

- interim derived energy buckets are accepted for v1 and documented as such in the task/memory closeout
- localhost review on `https://localhost` was accepted before PR closeout
