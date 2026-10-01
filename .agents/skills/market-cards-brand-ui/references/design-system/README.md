# Design System Docs

`DESIGN.md` remains the product-facing source of truth for the Market Cards design system: visual direction, shared principles, and rules that should be understood before designing or reviewing product UI.

This folder holds implementation-facing design-system details that are important, but too specific or too volatile to keep inline in `DESIGN.md`.

## Placement Rules

Use code comments when the rule is local to a selector, component prop, helper API, or behavior that a developer must see while editing that file.

Good fits for code comments:

- CSS selector contracts such as `.mcdl-page` and `.mcdl-app-page-shell`.
- Component prop semantics such as `PCAppShell contentWidth`.
- Helper constraints such as `AutoFitText` measuring single-line numeric text.
- Short implementation gotchas that prevent misuse at the call site.

Use docs in this folder when the rule crosses files, explains product policy, records a migration boundary, or needs examples and verification commands.

Good fits for docs:

- Layout contracts spanning CSS, React shells, and JS breakpoints.
- Dialog semantics spanning resolver, primitive, CSS, and business components.
- Number and currency display rules spanning utilities, product copy, and MintBox surfaces.
- CSS reuse and token governance that applies across many pages.

## Index

- [Layout Contracts](layout-contracts.md): page gutters, breakpoints, mobile bottom spacing, and PC content width.
- [Dialog System](dialog-system.md): semantic overlay kinds, runtime behavior, prohibited patterns, and migration inventory.
- [Hover System](hover-system.md): semantic hover grammar, shared tokens, helper classes, and prohibited hover patterns.
- [Loading States](loading-states.md): approved skeleton rules, shared loading primitives, and related layout rule.
- [Number Formatting](number-formatting.md): shared number display, MintBox currency policy, numeric typography, and fit behavior.
- [CSS Architecture](css-architecture.md): token reuse, local aliases, sizing rules, and shared primitive ownership.
