# Energy Calendar Ralph-Loop Plan

Status: Active implementation plan
Last updated: 2026-05-09

## Goal

Ship a full-page Energy Calendar at `/(tabs)/energy-calendar` with backend-backed
monthly solar/value data, primary navigation, device deep links, selected-date
Energy routing, and a centralized in-app Pulse mark refresh.

## Source Of Truth

- User-approved implementation plan from 2026-05-09.
- Mockup A:
  `/Users/jpaljasma/.codex/generated_images/019e0cbb-6880-7600-9814-6ed3fde8cb5c/ig_070620dae2683c5c0169ff2c769c388198921e0793cd581ecf.png`
- Mockup B:
  `/Users/jpaljasma/.codex/generated_images/019e0cbb-6880-7600-9814-6ed3fde8cb5c/ig_070620dae2683c5c0169ff2cd0c3d8819885f93959b6e0b2ba.png`
- Root `AGENTS.md`, `apps/universal/AGENTS.md`, `docs/explanation/ui-visual-system.md`.
- Architecture docs under `docs/architecture/README.md` and `docs/architecture/config*.md`.

## Locked Decisions

- Route: `/(tabs)/energy-calendar`.
- Nav placement: primary item immediately after `Energy`.
- Calendar default: current local month, `All devices`.
- Date click target: `/(tabs)/energy?device=<all|uuid>&preset=today&compare=1&date=YYYY-MM-DD&tz=<timezone>`.
- Selected day semantics: historical dates are full local days; current day is local midnight to now; comparison is prior local day.
- Adjacent-month days: show real prefetched data when available, muted; show preloading glyph while neighbor month queries warm.
- Future dates: visible and navigable by month, but unavailable and non-clickable.
- Brand mark scope: in-app chrome only; do not replace Expo install icons, favicon, Apple touch icon, or adaptive icon assets.
- Implementation style: Mockup B calendar anatomy and top-right totals, Pulse-rounded materials, A-style selected blue dot and prefetch glyphs.

## Agent Roster

- `project-manager`: branch hygiene, task board, integration sequencing, PR packaging.
- `backend-go`: proto, Go energy window/calendar contract, service tests.
- `bff-node`: Node BFF route/client normalization and tests.
- `frontend-universal`: universal API/model/route/UI/navigation/date picker/brand mark.
- `qa`: test matrix, targeted gates, browser screenshots.
- `product-review`: mockup fidelity, accessibility, UX polish, docs.

Max parallel workers: 4. Use cheaper worker models for bounded tasks with clear
file ownership; coordinator reviews and integrates locally.

## Progress Output Format

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

1. Red tests for backend selected-date windows and calendar aggregation.
2. Red tests for BFF calendar route and selected-date pass-through.
3. Red tests for universal route/model/calendar helpers and navigation.
4. Implement backend proto/service/calendar model and codegen if needed.
5. Implement BFF route/client schema.
6. Implement universal Calendar page, Energy date picker, nav, device deep link, and in-app brand mark.
7. Update docs for route/date semantics and brand-mark guidance.
8. Run targeted tests, typecheck/lint, visual QA, and package PR.
