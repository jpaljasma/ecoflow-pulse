# AGENTS

## Scope
This file defines repository-level working rules for humans and AI agents.

## Required Git Workflow
Always use a branch -> pull request -> merge workflow.

1. Start from an up-to-date `main`.
2. Create a dedicated feature branch before making code changes.
3. Commit changes only on the feature branch.
4. Push the branch to `origin`.
5. Open a pull request to `main`.
6. Merge only through the pull request after checks pass.

## Branch Rules
1. Do not commit directly to `main`.
2. Use clear branch names. For Codex-created branches, use `codex/<topic>`.
3. Keep branches focused on one logical change.

## Pull Request Rules
1. Include a clear title and summary of what changed and why.
2. Include test results for the change.
3. Update documentation when behavior or configuration changes.
4. Resolve review feedback before merge.

## Merge Rules
1. Merge only after CI and required checks are green.
2. Prefer squash merge unless the repository maintainers request otherwise.
3. Delete merged feature branches.

## Exceptions
Direct commits to `main` are allowed only if explicitly requested by a maintainer for an urgent reason.

## ML Training Workflow (ETA Models)
Use the built-in trainer at `cmd/ecoflow-ml-train` for fast, repeatable tuning.

### Models to train
1. `d2m` (DELTA 2 Max specific)
2. `dpu` (DELTA Pro Ultra specific)
3. `generic` (cross-device fallback)

### Efficiency requirements
Always use all three optimization techniques together:
1. Stratified sampling: preserve `profile x mode x transition` coverage.
2. Successive halving: evaluate many candidates on partial data, keep top subset for full data.
3. Feature precompute cache: rely on cached rolling features in trainer (prefix sums + window caches).

### Standard training commands
Use full telemetry CSV unless explicitly testing subsets.

```bash
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile d2m -candidates 4000 -stages 0.15,0.4,1.0 -seed 88
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile dpu -candidates 4000 -stages 0.15,0.4,1.0 -seed 88
go run ./cmd/ecoflow-ml-train -csv logs/telemetry_training.csv -profile generic -candidates 4000 -stages 0.15,0.4,1.0 -seed 808
```

### Selection and reproducibility
1. Compare `best_score` first, then `best_coverage`.
2. Keep and record the exact seed used for final params.
3. If quality is close, prefer the simpler/stabler window configuration.

### Applying trained params
1. Update profile constants in `cmd/ecoflow-mqtt-sub/estimates_profiled.go`.
2. Keep model selection priority:
   - device-specific profile
   - generic profile
   - unit (MPPT/device) estimate fallback
3. Run:
   - `go test ./cmd/ecoflow-mqtt-sub`
   - `go test ./cmd/ecoflow-ml-train`
   - `go test ./...`
