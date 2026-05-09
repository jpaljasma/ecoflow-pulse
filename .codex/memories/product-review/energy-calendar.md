# product-review Memory — Energy Calendar

## Current focus

Keep UI faithful to the approved hybrid: B grid/top totals, Pulse roundness, A prefetch glyph and selected blue dot.

## Files to inspect first

- `docs/explanation/ui-visual-system.md`
- `apps/universal/AGENTS.md`
- Mockup A and Mockup B image paths in the Ralph-loop plan.

## Decisions made

- No marketing copy.
- Sunday-start weekday headers.
- Top-right summary shows solar and saved money.
- Desktop calendar cells use explicit web sizing:
  `calc((100% - 12px) / 7)` with fixed `flex-basis` and `max-width`.
- The heatmap now uses green/teal energy tones with sun and dollar icons,
  matching the approved mockup language while preserving light and dark themes.
- Mobile uses the clean iOS-inspired week-row treatment rather than shrinking
  the desktop tiles.

## Open risks

- The active route uses the existing product sidebar width, so exact pixel
  parity with the standalone concept mockup is constrained by real app chrome.

## Next step

No product-review blocker remains before branch packaging.
