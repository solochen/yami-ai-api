# Design System Specification: The Ethereal Vault & Midnight Editorial

This document is the product-facing design-system source of truth for maintained product pages in `market-cards-web`.
The product should feel like a premium financial gallery: editorial, calm, tactile, and deliberate.
It should never feel like a generic utility dashboard, and it should never drift into a parallel page-local design system when shared `mcdl` primitives already cover the job.

Implementation details that change with selectors, props, or helpers live closer to the code or in focused docs under `docs/design-system/`.

## 1. Scope And Detail Placement

`DESIGN.md` should keep the durable design contract: creative direction, hierarchy, rhythm, token semantics, shared component expectations, accessibility, and guardrails.

Use code comments for details a developer must see while editing one file:

- Selector mechanics, such as `.mcdl-page` or `.mcdl-app-page-shell`.
- Component prop semantics, such as `PCAppShell contentWidth`.
- Helper constraints, such as `AutoFitText` fit behavior.
- Short implementation gotchas that prevent local misuse.

Use focused docs for cross-file or policy-heavy details:

- Layout contracts: `docs/design-system/layout-contracts.md`.
- Dialog semantics and runtime behavior: `docs/design-system/dialog-system.md`.
- Hover grammar, tokens, and prohibited interaction patterns: `docs/design-system/hover-system.md`.
- Loading skeleton organization: `docs/design-system/loading-states.md`.
- Number formatting and MintBox currency display: `docs/design-system/number-formatting.md`.
- CSS reuse and token governance: `docs/design-system/css-architecture.md`.

## 2. Overview & Creative North Star

**Creative North Star: "The Digital Curator in the Ethereal Vault"**

This design system moves away from the utility-first density of traditional fintech dashboards toward a high-end editorial experience.
By merging the tactile nature of rare cards with a futuristic, glass-aware digital interface, the product should feel secure, weightless, and premium at the same time.

Whether operating in the warm light of the Ethereal Vault or the intensely focused dark of the Nocturnal Curator, the interface should feel like a financial gallery rather than a spreadsheet.
The visual language is a flat-glass hybrid: color remains flat and solid, while surfaces and overlays gain depth through transparency, layering, and blur.

## 3. Layout & Information Rhythm

To achieve a premium editorial tone, layouts should prioritize clarity and pacing over density.

- **Dynamic rhythm:** Vary the visual weight of information. A large display value followed by a small descriptor is encouraged when it improves hierarchy.
- **Editorial asymmetry:** Create asymmetry inside the content grid through media crops, staggered blocks, and internal composition. Do not break the page's base gutter system to create that effect.
- **Eyebrow copy:** Use small uppercase eyebrow text as a locator above major headlines. The canonical treatment is a compact label in the accent color with a small filled dot indicator.
- **Intentional whitespace:** Whitespace is a luxury material. Use generous interior spacing so cards and sections feel deliberate, breathable, and expensive.
- **Section layering:** Alternate between the base surface and a recessed panel rhythm so related content feels grouped without relying on hard separators.
- **Grounding elements:** Footer zones, support rails, and secondary blocks may use the recessed surface to create a subtle visual base.

Implementation reference: page gutters, responsive breakpoints, app bottom spacing, and PC content width rules live in `docs/design-system/layout-contracts.md`. Selector-level mechanics live in `src/styles/mcdl.css`, `src/styles/mcdl-app-shell.css`, `src/constants/breakpoints.js`, `src/hooks/useIsDesktop.js`, and `src/components/PCAppShell.jsx`.

## 4. Colors & Surface Philosophy

The palette rejects pure black in light mode and relies on warm neutrals to preserve material warmth.
Surfaces should feel physical and layered, not sterile.

Exact color values live in `src/styles/mcdl.css`; this document owns the semantic intent.

### Core Accent

- `--mcdl-color-accent`

Use the accent color for primary actions, active states, key highlights, and a controlled amount of strategic emphasis.

### Surface Hierarchy

Boundaries should usually come from surface shifts, not hard lines.

| Layer Role | Token | Usage |
| :--- | :--- | :--- |
| Base Canvas | `--mcdl-color-surface` | Main page background |
| Recessed Surface | `--mcdl-color-surface-subtle` | Section panels, chips, grouped metric areas |
| Elevated Card | `--mcdl-color-surface-card` | Cards, dialogs, overlays |
| Stage / Media | `--mcdl-color-stage` | Image stages, media zones, showcases |

### Text and Structural Tokens

| Role | Token |
| :--- | :--- |
| Title / Heading | `--mcdl-text-title` |
| Body / Secondary | `--mcdl-text-body` |
| Ghost Border | `--mcdl-border-ghost` |
| On Accent | `--mcdl-color-on-accent` |
| Positive | `--mcdl-color-positive` |
| Negative | `--mcdl-color-negative` |
| On Inverse | `--mcdl-color-on-inverse` |

### Surface Rules

- **No-line principle:** Do not use opaque borders for major sectioning. Prefer background shifts and depth.
- **Ghost-border fallback:** When a boundary is needed, use `--mcdl-border-ghost`. Acceptable forms are subtle solid, dashed empty-state borders, or transparent-to-visible hover edges.
- **Flat color first:** Normal surfaces and buttons should stay flat. Do not introduce decorative gradients to compensate for weak hierarchy.
- **Glass only where justified:** Floating overlays and high-priority elevated surfaces may use semi-transparency plus blur.
- **Known exception:** Shared acrylic or floating-card treatments may use controlled gradients when they are part of the established showcase or glass primitive set, not ad hoc page styling.

## 5. Typography: Editorial Authority

The system uses a strict 3+1 hierarchy to balance editorial impact, readability, financial precision, and utility scanning.

1. **Manrope** via `--mcdl-font-body`
   Used for body text, structural hierarchy, card names, labels, and readable interface copy.
2. **Commissioner** via `--mcdl-font-numeric`
   Reserved for monetary amounts, prices, valuations, counts, totals, and core numeric metrics.
3. **Epilogue** via `--mcdl-font-display`
   Used for display headlines, dramatic accents, major section titles, and non-financial high-impact markers.
4. **Mono fallback** via `--mcdl-font-mono`
   Reserved for raw identifiers or utility strings that genuinely benefit from fixed-width scanning.

### Typography Rules

- Use `--mcdl-font-numeric` for financial numbers and core metrics.
- Use `--mcdl-font-body` for readable UI titles, labels, and paragraphs unless the pattern explicitly calls for display typography.
- Use `--mcdl-font-display` sparingly and intentionally. Overuse weakens the editorial contrast.
- Use `--mcdl-font-mono` only for utility text such as raw hashes or machine-like fragments, not for general UI labels.
- Keep the browser default root font size. Do not change the root font size by page or breakpoint.

## 6. Elevation, Depth, and Motion

Depth is communicated through layered surfaces, shadows, blur, and motion rather than harsh outlines.

- **Layering principle:** A card should not visually collapse into the surface beneath it. Elevation starts with different surface tiers before it starts with shadow.
- **Two-tier shadow system:** Use `--mcdl-shadow-soft` for resting elevation and `--mcdl-shadow-strong` for promoted hover or hero elevation.
- **Ghost edge fallback:** If a container edge is needed, use `--mcdl-border-ghost`. It should be felt, not announced.
- **Single interaction system:** Mobile and desktop use the same interaction grammar unless a component is physically different.
- **Tactile restraint:** Hover and press feedback should be clear, small, and material-aware rather than decorative or flashy.
- **Reduced motion:** Non-essential motion must reduce or disappear when `prefers-reduced-motion` is active.

Detailed hover categories, token names, helper classes, and prohibited patterns live in `docs/design-system/hover-system.md`.

## 7. Shared Component Expectations

Maintained product UI should compose the shared `mcdl` primitives rather than recreate a parallel set of wrappers or page-local patterns.

Current primitive inventory and code-level usage notes live in `src/components/ui/README.md`. Dialog-specific rules, including mobile header alignment, dividers, and header-to-content spacing, live in `docs/design-system/dialog-system.md`.

### Cards and Lists

- Use large, soft radii for major structural cards.
- Avoid list dividers as the default separation pattern. Prefer spacing, grouping, and surface changes.
- Keep interaction tactile, not flashy.

### Buttons

- **Primary:** Accent-colored, flat, high-clarity CTA. Pill shape by default.
- **Secondary:** Surface-integrated button with subtle border or surface shift.
- **Tertiary:** Minimal emphasis, often text-forward, with restrained hover treatment.
- Do not introduce decorative gradients or page-local CTA systems when a shared button primitive or close shared variant already fits.

### Inputs and Controls

- All maintained product inputs use the `mcdl` system. Standalone brand-led surfaces may opt out only when they intentionally do not share the product chrome.
- Keep shell, border, radius, focus ring, placeholder treatment, and read-only behavior aligned with shared form tokens.
- Specialized controls may keep custom behavior or third-party internals such as Ant Design `Select` / `DatePicker`, but their shell language, popup anchoring, and theme tokens must still match `mcdl`.
- Do not create a new page-local input design system.

### Selection Chips

- Use the shared chip scale and uppercase, compact labeling treatment.
- Unselected chips should remain surface-based.
- Selected chips may take the accent surface directly when the pattern calls for strong clarity.

### Status Badges

- Status badges should be compact, uppercase, high-contrast, and semantically themed.
- Floating media badges should usually live in the top-right of the media or card stage.
- Reuse shared badge semantics and avoid one-off status badge systems.

### Empty and Loading States

- Centered layout, generous vertical padding, and restrained copy.
- Dashed ghost-border treatment is acceptable for empty-state framing.
- Non-blocking loads should preserve the final layout with shared skeleton primitives, not page-local duplicate placeholder systems.
- Repeated list and card loading states should live in the same item or card renderer as the loaded state through a `loading` prop or equivalent helper option.
- Skeleton placeholders stay `aria-hidden` and token-driven; use feedback primitives for blocking loading, empty, and error states.

Skeleton/loading-state organization and implementation references live in `docs/design-system/loading-states.md`.

## 8. Numeric Display Rules

Numbers are part of the product system, not a page-local implementation detail.

- Route user-facing numeric display through the shared number-formatting system.
- Pick the display strategy before styling: exact, compressed, abbreviated, or intentionally truncated identifier text.
- MintBox monetary amounts are product USD values, display with `$`, and do not follow the global currency selector.
- Use `--mcdl-font-numeric` for prices, balances, valuations, totals, percentages, counts, and stat values.
- Use `AutoFitText` only for single-line numeric UI that must remain fully visible in bounded width.

Detailed product policy and implementation references live in `docs/design-system/number-formatting.md`.

## 9. Accessibility Standards

While the system relies on tonal shifts and transparency, readability remains non-negotiable.

- Keep main text against its surface above a 7:1 contrast ratio whenever practical.
- Keep secondary text at or above 4.5:1 contrast.
- Transparent and blurred surfaces must still preserve readable text contrast.
- Focus states must remain obvious and consistent with shared control tokens.
- Reduce or remove non-essential motion when `prefers-reduced-motion` is active.

## 10. Strict Guardrails

### Do

- Use shared `mcdl` tokens and primitives first.
- Prefer `rem` for routine layout, spacing, radius, and typography.
- Promote repeated page patterns into the shared layer instead of cloning them.
- Use the shared shadow and motion system on elevated interactive UI.
- Keep code direct, readable, and explicit.

### Do Not

- Do not introduce page-local parallel design systems.
- Do not use solid opaque borders as your default sectioning method.
- Do not use pure black for light-theme surfaces or text.
- Do not rely on gradients to make normal surfaces or buttons feel premium.
- Do not change the root font size by page or breakpoint.
- Do not keep compatibility, transitional, or patch-style sizing code once the shared system covers that use case.

CSS reuse details, local alias rules, and shared primitive ownership live in `docs/design-system/css-architecture.md`.

## 11. Working Method

- Start with the shared layer, then apply the smallest possible semantic override at the page or component level.
- Before adding a new size or pattern, check whether the shared layer already exposes a reusable token or primitive.
- If a pattern appears more than once, promote it into the shared layer.
- Remove obsolete helpers and compatibility branches after the shared primitive lands.
- Leave the codebase cleaner than you found it: no debug residue, no exploratory styling, no dead wrappers, and no invalid tokens.
