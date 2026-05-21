# UI Visual System

This page captures the agreed visual direction for the universal app refresh.
It is implementation guidance for product surfaces, iconography, and branding.

## Brand Position

The product-facing brand is `Pulse`.

- Pulse is the UI identity across app name, iconography, splash/loading states,
  and visible product copy.
- EcoFlow remains an integration and device-source detail, not the primary
  product brand presented in the app shell.
- Product surfaces should read as a premium energy operating system rather than
  an OEM companion skin.

## Design Skill Workflow

Codex UI work should use the external design skills listed in root `AGENTS.md`
as development helpers for consistent multi-page web/tablet experiences. Install
them first in any fresh Codex environment, then use them as follows:

- [`design-systems`](https://skills.sh/mindrally/skills/design-systems) for
  shared tokens, component variants, interaction states, and cross-page UI
  rules.
- [`flex-grid-flow`](https://skills.sh/oerlellijk/design-system-skill/flex-grid-flow)
  for responsive grids, split panels, fluid spacing, typography rhythm, and
  desktop/tablet/phone layout transitions.
- [`web-design-guidelines`](https://skills.sh/vercel-labs/agent-skills/web-design-guidelines)
  as a final review pass for accessibility, spacing, affordances, and
  consistency before opening a PR.

These skills should reinforce the Pulse visual system and existing Tamagui
implementation patterns, not introduce a parallel component framework.

## Product Modes

Pulse uses two intentionally different presentation modes.

### Overview

Overview screens should feel:

- glanceable
- atmospheric
- solar-led
- large-value-first

Overview pages use:

- a hero metric panel
- supporting summary tiles
- ambient gradients and contextual imagery where useful

Home and fleet overview screens should use the first viewport as an operating
console, not a sparse landing page.

- Fill wide hero panels with useful navigation and state, not empty center
  space.
- Keep the fleet hero anchored by the primary daily solar metric, active device
  shortcuts, and core metric tiles.
- Device shortcuts should be image-forward and comparable in footprint to
  adjacent metric tiles: thumbnail, device name, the shared SOC progress bar,
  and only a compact state icon when charging or discharging.
- Device shortcut grids show two items per row, active devices first, with a
  two-row maximum on desktop/tablet and a one-row maximum on phone layouts.
- Device shortcuts navigate directly to the device; a separate `All Devices`
  control should sit below the shortcut grid and jump to the full inventory list
  on the same page.
- Solar generation history is the primary day context on the Devices overview
  and should own a full-width row before secondary widgets.
- Devices overview chart/impact sections should render with stable reserved
  space; load them as soon as they are even partially visible, lazy-load only
  truly offscreen panels, and preserve the last useful chart payload during
  refetches instead of clearing to an empty chart.
- Rolling live-power charts should repair short all-zero holes between valid
  non-zero samples as ingest gaps, while preserving leading, trailing, and
  longer idle zero windows.
- Complementary operational widgets can sit side by side at `50/50` on
  desktop/tablet with matched card height, and stack on phones. Remove redundant
  telemetry summaries when the same signal is already present in primary tiles.
- Optional header chrome must yield on narrow widths before it overlaps the
  page title or product identity.

### Analysis

Analysis screens should feel:

- denser
- calmer
- more chart-led
- more precise

Analysis pages use:

- one dominant square or near-square chart
- a small number of supporting cards
- quieter motion and lower visual drama than overview pages

Device Detail and Energy analysis surfaces should keep the current-window hero,
primary history, and live power profile in the initial render path. Secondary
PV-envelope, forecast, inference, and explainer panels should lazy-load as they
approach the viewport while preserving their reserved card frames and any
previous useful chart data during refreshes.

## Visual Language

The target visual blend is:

- Victron chart readability
- EcoFlow-style ambient hero presence
- Anker-level cleanliness and restraint
- Apple-like polish in spacing, typography, and controls

The default shell is dark and canonical.
Light mode is fully supported, but should feel like the same product translated
to a brighter material system.

## Typography

Typography should feel premium, appliance-grade, and slightly technical.

- one premium sans family across the app
- large display numerals for hero metrics
- compact but breathable UI text
- strong hierarchy through size, spacing, and weight rather than decorative
  effects

Suggested sizing rhythm:

- `12` small labels
- `14` support text
- `16` body / primary control text
- `20` section titles
- `24` large panel values
- `32+` hero values

## Spacing And Shape

The core spacing rhythm is:

- `8`
- `12`
- `16`
- `24`
- `32`

Usage guidance:

- hero panels: `24-32` padding
- summary tiles: `16-20` padding
- chart cards: `20-24` padding
- compact rows: `12-16` padding

The shape language is consistent across the app:

- medium corner radius
- tight, aligned internal spacing
- minimal shadow
- calm borders and tonal contrast

## Surfaces

Pulse uses a dark slate / graphite material system.

- background: deep graphite / blue-black
- elevated surface: cool slate
- interactive surface: slightly brighter slate
- separators: low-noise blue-gray borders
- highlights: soft internal glows and restrained gradients

Do not use:

- high-noise rainbow gradients
- thick borders everywhere
- ornamental shadows
- multiple unrelated accent colors on one panel

## Controls

The control system should be shared globally.

- one segmented-control pattern for time windows
- one primary button
- one secondary button
- one quiet / ghost button
- one text-input family with the same focus ring, label spacing, helper text,
  and disabled behavior

Control rules:

- minimum `44px` hit targets
- visible hover and focus states on web
- AA contrast minimum for text and control labels

## Navigation

Large screens use a sidebar.

- expanded labeled sidebar by default on desktop and tablet
- collapsible to an icon rail
- remembers last state

Phones keep a compact mobile navigation pattern.

Within Energy, the experience is split into three deep-linkable panes:

- `Overview` for the main balance and trend analysis
- `Solar` for weather-aware forecast and verification
- `Impact` for the full avoided-emissions detail view

`Energy Calendar` is a separate primary navigation page directly after
`Energy`. Its calendar surface should reuse the Pulse Fleet hero's full-width
gradient/lattice material while keeping the Apple-like month grid dense,
legible, and tile based. Desktop and tablet layouts use a true seven-column
grid (`repeat(7, minmax(0, 1fr))`) so every day cell has identical horizontal
math; avoid percentage rows that can drift under padding or wrapping. Clickable
day tiles should use the same pointer cursor, subtle lift, alpha background
change, stable shadow, and press feedback as the Pulse Fleet device tiles.
Calendar route state should not
carry timezone query parameters; local day labels come from the profile
timezone.

Compact Energy Impact widgets on Devices and per-device pages should stay summary-only
and link into the full `Energy > Impact` pane using the current fleet or device scope.

Device detail pages should read as hardware-first operating views.

- Battery Packs and Solar Inputs belong before secondary insight widgets.
- Redundant live-telemetry and diagnostic panels should stay hidden from the
  default detail view when their signal is already represented in hero tiles,
  hardware sections, or System Signals.
- Metric-derived System Signals, such as solar passthrough, should stay
  generic across device models and come from observed PV/load/net balance
  rather than provider- or model-specific mode guesses.
- Energy Impact and Device Solar Forecast should sit side by side at `50/50`
  with matched height on tablet/desktop, then stack on phones.
- Shared header weather/solar chrome should open `Energy > Solar` with device
  scope on device pages and fleet scope elsewhere.

In-app chrome should use the shared `PulseMark` component backed by
`apps/universal/assets/icon.png` for sidebar, menu, about, logo, and loading
surfaces. That asset is part of the same generated icon system as the Expo
install icons, favicon, touch icon, and social share preview.

Settings subpages should follow the same shell contract:

- sidebar-visible on larger layouts
- breadcrumb-first header chrome
- shared card, button, and input styling
- no fallback to plain admin forms for credential or account workflows

Integration Settings belongs under Settings as a first-class product page.

- use a connector inventory plus detail workspace layout instead of a single long credentials form
- list saved provider connections inside the selected connector detail pane
- keep one active connection per user per connector
- reject duplicate access keys for the same provider, even across different
  users or saved connection rows
- when rotating EcoFlow keys, reuse the existing discovery plus MQTT probe path
  as the activation gate instead of introducing a second validation concept in
  the UI
- allow existing saved inactive credentials to be activated directly without
  forcing the user to re-enter the same key material
- only switch the active EcoFlow connection after provider discovery and MQTT
  validation succeed for the user’s already-enabled devices
- connector discovery failures caused by rejected provider credentials should
  surface as inline warnings with a direct path back to Integrations, not as
  fatal page-level crashes

## Chart Grammar

Charts are a core part of the brand and should be visually consistent.

### Default Energy Chart

The default energy chart is a stacked bar chart.

- positive Y-axis: generation, charging, inbound energy
- negative Y-axis: load, discharge, outbound energy
- calm gridlines
- square or near-square chart area
- one dominant chart per screen

### Comparison Treatment

Previous-period comparison uses:

- a thin overlay line only

Avoid:

- competing full-weight duplicate series
- too many toggles
- chart legends that behave like a settings panel

### Supporting Cards

Supporting cards may use:

- tiny sparklines
- compact gauges
- concise breakdown strips

These exist only to support the dominant chart, not compete with it.

## Color Roles

Recommended semantic roles:

- solar: warm yellow
- battery charge: green
- battery discharge / load: amber-orange
- grid: cyan-blue
- neutral structure: slate / graphite / cool gray
- alerts: sparing, high contrast, never decorative

## Branding Assets

### App Icon

The icon should be Pulse-branded, not OEM-branded.

Approved v2 direction:

- abstract Pulse monogram first
- horizon-cut `P` with a subtle solar-rise cue
- rounded-square tile
- luxury-tech finish
- dark graphite base
- cyan/teal primary glow
- restrained warm solar highlight

Avoid:

- literal houses, panels, plugs, or bolts
- hearts or heart-pulse symbols
- generic green-energy clichés
- over-detailed marks that collapse at small size

### Active Icon Concept

Concept name: `Horizon Cut P`

Design intent:

- abstract `P` formed by an open cyan arc and short stem
- thin horizon cut through the bowl
- restrained warm sun rising from the horizon
- energy autonomy without literal hardware
- strong silhouette at app-icon sizes

Source and export workflow:

- vector master: `apps/universal/assets/pulsemark-v2-horizon-cut.svg`
- raster export script: `apps/universal/scripts/generate_app_icons.py`
- generated outputs include app icon, adaptive foreground, favicon, touch icon,
  social share image, and web metadata shell

## Icon Prompt Directions

These prompts are intended for designer handoff or image-generation workflows.

### Prompt A

Create a premium app icon for a product named `Pulse`.
The icon should use an abstract monogram or energy mark, not literal solar
hardware.
Use a rounded-square dark graphite tile with a cyan-teal luminous primary mark
and a very restrained warm-gold accent suggesting sunrise energy.
The mark should feel luxury-tech, crisp, geometric, and distinctive at small
sizes.
Avoid text, houses, panels, plugs, leaves, lightning bolts, and generic green
energy symbolism.

### Prompt B

Design a luxury-tech mobile app icon for `Pulse`, a premium home-energy control
room.
Use a sculpted abstract `P` that also hints at a pulse path and a solar horizon.
The surface should be deep slate with subtle glass-metal polish, soft inner
glow, strong contrast, and minimal complexity.
Primary accent is cyan-teal; secondary accent is a tiny warm solar highlight.
The result should feel ultra-modern, exciting, and Apple-quality.

### Prompt C

Generate a high-end dark app icon with an abstract energy-flow monogram for
`Pulse`.
The icon should look premium, clean, and futuristic, with a single dominant
symbol centered on a rounded-square tile.
Use blue-black, graphite, cyan-teal, and a restrained gold accent.
Keep the design simple enough to read at favicon size and compelling enough for
an App Store listing.

## Splash / Loading Screen Direction

The splash screen should match the icon system.

- dark atmospheric background
- centered Pulse icon mark
- subtle cyan-to-gold ambient light
- optional small `Pulse` wordmark
- no busy illustration
- no OEM co-branding in the hero treatment

## Rollout Priority

The UI refresh should ship in this order:

1. visual tokens and branding
2. app icon and shared shell surfaces
3. energy analysis screen
4. overview dashboard
5. device detail hybrid view
