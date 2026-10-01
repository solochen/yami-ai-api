---
name: market-cards-brand-ui
description: Apply and review the Market Cards Ethereal Vault / Midnight Editorial design system to frontend web UI work. Use when building, refactoring, or reviewing pages, components, layout, styling, responsive behavior, accessibility, or motion for a premium financial-gallery interface; do not use for unrelated backend or business-logic work.
metadata:
  short-description: Apply the Market Cards premium editorial UI system
---

# Market Cards Brand UI

Use this skill when the requested frontend work should follow the Market Cards design language. The durable visual contract is in [references/DESIGN.md](references/DESIGN.md). Read it for every substantial UI change, then read only the focused reference that matches the change:

- Layout, gutters, breakpoints, and shell spacing: [references/design-system/layout-contracts.md](references/design-system/layout-contracts.md).
- Hover, motion, and prohibited interaction patterns: [references/design-system/hover-system.md](references/design-system/hover-system.md).
- Dialogs and overlays: [references/design-system/dialog-system.md](references/design-system/dialog-system.md).
- Loading and skeleton states: [references/design-system/loading-states.md](references/design-system/loading-states.md).
- Numbers, prices, and metric formatting: [references/design-system/number-formatting.md](references/design-system/number-formatting.md).
- CSS reuse and token ownership: [references/design-system/css-architecture.md](references/design-system/css-architecture.md).
- Reference index: [references/design-system/README.md](references/design-system/README.md).

## Working contract

1. Inspect the target frontend's existing tokens, shared primitives, route structure, and responsive conventions before editing. Preserve business behavior, API contracts, data flow, and existing interaction semantics unless the user explicitly asks to change them.
2. Use the target project's shared components and tokens first. If it does not have the source project's `mcdl` primitives, translate the semantic rules into its existing design-token system; do not blindly copy source-specific imports or create a parallel page-local design system.
3. Keep the visual direction premium, editorial, calm, tactile, and deliberate: vary information weight, use intentional whitespace, support asymmetry inside the grid, and group sections with surface hierarchy rather than heavy dividers.
4. Prefer flat solid colors for ordinary surfaces and buttons. Use transparency, blur, or controlled gradients only for established elevated/glass primitives. Do not use pure black for light-theme surfaces, decorative gradients as a substitute for hierarchy, or opaque borders as the default sectioning method.
5. Preserve the typography roles when the target project supports the fonts: Manrope for body/interface copy, Commissioner for financial numbers and metrics, Epilogue sparingly for display headlines, and monospace only for genuinely machine-like identifiers. Do not change the browser root font size.
6. Use large soft radii, tactile restrained interaction feedback, shared button/input/chip/badge patterns, and semantic surface tiers. Reuse shared loading, empty, error, and numeric-display primitives instead of duplicating page-local versions.
7. Keep accessibility non-negotiable: maintain readable contrast, obvious focus states, keyboard usability, semantic labels, and reduced or removed non-essential motion under `prefers-reduced-motion`.
8. Prefer `rem` for routine sizing and promote repeated patterns into the shared layer. Remove dead compatibility styling, debug residue, and exploratory overrides before finishing.

## Validation

After implementation, run the smallest relevant typecheck/lint/test/build command available in the target frontend. Inspect the affected page at desktop and mobile widths when possible. Check overflow, focus visibility, contrast-sensitive surfaces, loading/empty/error states, and reduced-motion behavior. Report any validation that could not be run.

This skill defines visual and implementation guidance only; it does not authorize external publishing, deployment, credential changes, or destructive operations.
