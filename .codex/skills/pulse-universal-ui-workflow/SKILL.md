---
name: pulse-universal-ui-workflow
description: Use for Pulse universal-app UI polish, dashboard/device-page layout changes, responsive visual refinements, shared chrome behavior, and PR-ready frontend follow-through in this repository.
---

# Pulse Universal UI Workflow

## Overview

Use this project skill when changing `apps/universal` product UI, especially
Devices, device detail, Energy, shared header chrome, chart/widget composition,
or design-language follow-through.

The preferred shape is iterative and evidence-backed: clarify the intended
screen rhythm, lock the behavior with focused tests, implement through shared
helpers/components, verify in browser screenshots, update durable guidance, and
commit from a clean feature branch.

## Workflow

1. **Sync and isolate**
   - Start from clean, up-to-date `main`.
   - Create a focused `codex/<topic>` branch.
   - Read root `AGENTS.md`, `apps/universal/AGENTS.md`, and
     `docs/explanation/ui-visual-system.md` before changing layout.

2. **State the compact design**
   - Restate the requested hierarchy and route behavior in one or two
     implementation sentences.
   - For small follow-ups, do not create a heavy design doc; use the existing
     visual system and the user's accepted direction.
   - Prefer dense operational UI over landing-page spacing on product screens.

3. **Write or update focused tests first**
   - Use Playwright E2E when the request changes visible layout, navigation, or
     route params.
   - Use unit tests for pure helpers such as sorting, preview limits, route
     state, or formatting.
   - Invert tests for removed panels instead of leaving stale positive
     assertions.

4. **Implement through shared primitives**
   - Reuse route builders such as `buildEnergyRouteParams`.
   - Reuse image/device helpers instead of repeating model matching.
   - Add small `fill`, `testID`, or wrapper props to shared cards when equal
     height or testability is needed in more than one place.
   - Remove dead imports and obsolete panel-local helpers as panels disappear.

5. **Verify like a product change**
   - Run targeted universal gates:
     - `npm run -w apps/universal typecheck`
     - `npm run -w apps/universal lint`
     - targeted `npm run -w apps/universal test -- ...` when helper logic
       changes
     - targeted `npm run -w apps/universal e2e:web -- ...` for visible flows
   - Export and serve the web build when screenshots are needed:
     - `CI=1 npm run -w apps/universal export:web`
     - `npx serve -s apps/universal/dist -l <free-port>`
   - Capture desktop and mobile screenshots with mocked data or localhost, then
     inspect them with `view_image`.
   - Stop temporary servers before final handoff.

6. **Update durable guidance**
   - If a UI decision should repeat, update `apps/universal/AGENTS.md` and/or
     `docs/explanation/ui-visual-system.md`.
   - If the workflow itself improved, update this skill before committing.
   - Run `make lint` whenever Markdown guidance changes.

7. **Package cleanly**
   - Use `git diff --check`, `git status --short`, and a focused commit.
   - For PRs, push the branch, create a draft PR, then verify the body with
     `gh pr view --json body` or the GitHub connector result.

## Pulse UI Defaults

- Overview/dashboard panels should use available space for useful state and
  navigation, not empty center areas.
- Device shortcuts and detail panels should be image-forward, operational, and
  responsive without text overlap.
- Pair complementary widgets at `50/50` with matched height on tablet/desktop;
  stack on phones.
- Hide redundant telemetry or diagnostics panels when the same signal is already
  clearer in primary tiles, hardware sections, or route-specific detail views.
- Header weather/solar chrome should route to Energy's Solar panel with the
  current scope (`device` when explicit, otherwise `all`).

## Common Validation Targets

- Devices overview: `apps/universal/e2e/devices.spec.ts`
- Energy route state: `apps/universal/src/features/energy/model.test.ts`
- Device detail composition:
  `apps/universal/src/features/device-detail/components/DeviceDetailBody.tsx`
- Shared header solar/weather route:
  `apps/universal/src/shared/ui/AppMenu.tsx`
