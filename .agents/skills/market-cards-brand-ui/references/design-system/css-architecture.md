# CSS Architecture

Maintained page-level CSS should treat the shared `mcdl` layer as the source of truth instead of recreating tokens under a page prefix.

Code comments should explain local selector mechanics. This document owns cross-page governance: when to reuse primitives, when to introduce local variables, and when to promote a pattern into the shared layer.

## Sizing System

The project maintains one sizing system: shared `mcdl` tokens, shared primitives, and a small number of semantic local overrides.

Use `rem` for routine layout, spacing, radius, typography, dialogs, forms, chips, badges, tabs, section headers, and recurring card internals.

Prefer consuming existing `--mcdl-*` variables directly. If local tuning is truly necessary, define a small semantic custom property for that role instead of mirroring global tokens one-to-one.

Use canonical shared sizes. Do not scatter near-duplicate literal sizes throughout page CSS.

`px` is allowed only when device-pixel precision or external constraints genuinely require it, such as borders, hairlines, shadows, raster alignment, icon glyph sizing, or third-party integration limits.

## Required Reuse Pattern

Prefer shared primitives directly in markup before adding page-local wrappers.

Prefer direct token consumption in CSS for color, typography, radius, motion, shadow, and shared spacing values.

When a shared primitive needs local tuning, override published `mcdl` custom properties instead of cloning the primitive under a page prefix.

Shared wrapper components should own the shared stylesheet imports they depend on. Direct raw class usage should still import the matching shared stylesheet and follow the ownership and popup-anchoring contracts.

Reference code-level comments in:

- `src/components/ui/MCDLField.jsx`
- `src/components/ui/MCDLBottomSheet.css`
- `src/styles/mcdl-dialog.css`
- `src/utils/antdPopup.js`

## Local Aliases

Allowed:

- Page-semantic aliases that express actual page meaning instead of duplicating a shared token, such as stage aliases or page-specific watermark sizing.
- Local custom properties when there is no existing shared token for that role and the value is reused enough to justify a named variable.

Disallowed:

- One-to-one mirror aliases for shared tokens such as surface, text, accent, positive, negative, shadow, border, or timing tokens.
- Page-local token families that merely restate the same global scale under another prefix.
- Unused helper classes or token wrappers once a shared primitive covers the same responsibility.

## Promotion Rule

Start with the shared layer, then apply the smallest semantic override at the page or component level.

Before adding a new size or pattern, check whether the shared layer already exposes a reusable token or primitive. If a pattern appears more than once, promote it into the shared layer.

Remove obsolete helpers and compatibility branches after the shared primitive lands.
