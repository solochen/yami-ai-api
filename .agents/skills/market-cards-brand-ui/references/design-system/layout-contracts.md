# Layout Contracts

This document owns cross-file layout usage rules. The exact selector mechanics still live beside the runtime CSS and component code:

- `src/styles/mcdl.css` owns `.mcdl-page`, `.mcdl-page--compact`, and resolved gutter variables.
- `src/styles/mcdl-app-shell.css` owns `.mcdl-app-page-shell` and bottom-spacing modifiers.
- `src/components/PCAppShell.jsx` owns desktop shell content width modes.
- `src/constants/breakpoints.js` and `src/hooks/useIsDesktop.js` own the JS mobile/desktop boundary.

## Page Gutters

All maintained product pages use the shared `mcdl` page gutter system.

Approved gutter families:

| Family | Desktop | Small screens | Primary use |
| --- | ---: | ---: | --- |
| `comfortable` | `1.5rem` | `1.25rem` | Default product content rhythm. |
| `compact` | `1rem` | `1rem` | Dense list, feed, and filter-heavy scanning. |
| `chrome` | `1.25rem` | `1rem` | Sticky headers, filter bars, and app chrome. |
| `layer offset` | `1rem` | `0.75rem` | Recessed panels and nested content layers. |
| `marketing` | `1.75rem` | `1.25rem` | Standalone brand-led surfaces outside the standard app shell. |

Use `.mcdl-page` for standard product content. Add `.mcdl-page--compact` only when the primary task benefits from denser scanning. At one breakpoint, a page should have one primary content gutter; different spacing is valid only across different layout layers.

Page CSS should consume resolved variables such as `--mcdl-page-gutter-resolved`, `--mcdl-page-chrome-gutter-resolved`, and `--mcdl-page-layer-offset-resolved` instead of hard-coding a gutter family directly.

## Responsive Breakpoints

These named breakpoints apply to maintained React pages and shared CSS under `src/pages`, `src/components`, and `src/styles`. Static wireframes and previews under `src/html` are not part of this contract.

| Name | Value | Use |
| --- | --- | --- |
| `app-desktop` | `48rem` / `768px` | The only mobile/desktop render boundary. JS must use `useIsDesktop()` or constants from `src/constants/breakpoints.js`. |
| `pc-wide` | `80rem` / `1280px` | Wide desktop density changes, such as reducing hero/header affordances before the main layout stacks. |
| `pc-stack` | `72rem` / `1152px` | Desktop page layout stacking, including dashboard sidebars, detail columns, and settings grids. |
| `pc-compact` | `64rem` / `1024px` | Narrow desktop and tablet-landscape chrome adjustments, sticky sidebar removal, footer/header compaction. |
| `phone` | `30rem` / `480px` | Narrow-phone dialog, sheet, form, button, and poster sizing. |
| `phone-xs` | `23.5rem` / `376px` | Extreme narrow-phone fallbacks for gutters, short labels, compact card media, and last-resort text fitting. |

New viewport `@media` rules should use these named values unless there is a clear content-driven reason. If a component breaks at a value outside this table, prefer `@container`, `auto-fit/minmax()`, `clamp()`, or intrinsic sizing before adding a custom viewport breakpoint.

Capability queries such as `@media (hover: hover)` and `@media (prefers-reduced-motion: reduce)` are not layout breakpoints and remain valid outside the named width table.

## Mobile Bottom Spacing

Standard mobile app pages use `MCDLAppShell` plus `.mcdl-app-page-shell` on the scrollable page content layer. Pick exactly one bottom-spacing modifier for the active fixed chrome:

| Modifier | Use when |
| --- | --- |
| `.mcdl-app-page-shell--plain` | The page has no fixed bottom navigation or fixed bottom action. |
| `.mcdl-app-page-shell--tabbar` | The page renders `MCDLTabbar`. |
| `.mcdl-app-page-shell--cta` | The page renders a fixed `.mcdl-floating-bar` / `.mcdl-floating-action` CTA. |

Bottom avoidance is owned by `src/styles/mcdl-app-shell.css` and the spacing tokens in `src/styles/mcdl.css`: `--mcdl-app-plain-spacing`, `--mcdl-app-tabbar-spacing`, `--mcdl-app-cta-spacing`, and `--mcdl-floating-bar-pad-bottom`.

Fixed bottom CTAs should use the shared `.mcdl-floating-bar` and `.mcdl-floating-action` primitives. Page CSS may tune published primitive variables such as width, z-index, color, font size, or horizontal padding, but must not redefine fixed CTA bottom safe-area padding or add page-local content `padding-bottom` to compensate.

If a page only sometimes renders a fixed bottom CTA, select `--cta` only in those states and use `--plain` otherwise. Loading skeletons may use `--cta` only when a fixed CTA skeleton is rendered.

## Desktop Content Width

Maintained desktop product pages use `PCAppShell` content width modes instead of page-local outer width rules.

Use `contentWidth="full"` for multi-column, dashboard, catalog, detail, and operational product surfaces. This is the default.

Use `contentWidth="feed"` and the shared `--mcdl-pc-feed-max-width` token only for inherently linear, single-column reading or scanning flows.

Desktop pages should not add their own top-level `max-width`, `width: min(...)`, `margin: 0 auto`, or extra horizontal padding to create a second content lane. Build internal rhythm inside sections and cards instead.
