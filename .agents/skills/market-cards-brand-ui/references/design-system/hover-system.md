# Hover System

Market Cards uses one semantic hover system across maintained product UI. Hover should make interactive elements feel tactile and deliberate without creating page-local motion languages.

## Source Of Truth Split

`DESIGN.md` owns the durable product principle: interaction should feel premium, calm, and tactile, and mobile and desktop should not fork into separate hover grammars.

This document owns the implementation-facing hover policy: semantic hover categories, token usage, helper classes, prohibited patterns, and verification checks.

`src/styles/mcdl.css` owns the canonical token values and helper class definitions. Page and component CSS should consume those tokens instead of inventing local hover scales, shadows, timings, or accent mixes.

## Core Rules

- Use the same hover grammar on mobile and desktop. Do not fork hover behavior by breakpoint unless the component's physical presentation is different.
- Use shared timing: `--mcdl-hover-duration` and `--mcdl-hover-ease`.
- Use semantic hover tokens and helper classes before adding page-local selectors.
- Keep hover tactile, not flashy. Prefer surface, border, shadow, brightness, and small transform changes.
- Make press states feel like a physical response by returning vertical lift to `0` while applying a shared press scale.
- Reduce or remove non-essential motion when `prefers-reduced-motion` is active.

## Semantic Grammar

| Category | Use for | Expected behavior | Shared helper |
| --- | --- | --- | --- |
| Quiet hover | menu rows, nav rows, tabs, chips, inline links, tertiary controls | surface, border, or text shift only; no lift | `.mcdl-hover-quiet` |
| Control hover | compact buttons, filter controls, icon buttons, secondary action controls | small lift, stronger surface, optional soft control shadow | `.mcdl-hover-control` |
| Card hover | actionable cards, list cards, product tiles, history records | card lift, promoted border, strong card shadow | `.mcdl-hover-card` |
| Primary action hover | primary CTAs and accent-filled actions | keep accent fill; use slight brightness and action shadow | `.mcdl-hover-primary-action` |
| Press state | clickable controls and cards that already have hover feedback | scale down and reset lift to avoid stacking active and hover transforms | `.mcdl-hover-press` |
| Media hover | artwork, cover images, showcase thumbnails | tokenized media scale; avoid unrelated parallax or one-off zoom values | direct token usage |
| Danger hover | destructive actions | danger surface, border, text, or shadow tokens; never reuse primary accent hover | direct token usage |

## Token Families

Use these token families from `src/styles/mcdl.css`:

- Timing: `--mcdl-hover-duration`, `--mcdl-hover-ease`.
- Lift and press: `--mcdl-hover-lift-quiet`, `--mcdl-hover-lift-card`, `--mcdl-hover-lift-hero`, `--mcdl-press-scale-soft`, `--mcdl-press-scale`, `--mcdl-press-scale-strong`.
- Surfaces: `--mcdl-hover-surface`, `--mcdl-hover-surface-strong`, `--mcdl-hover-surface-accent`, `--mcdl-hover-surface-muted`, `--mcdl-hover-surface-raised`, `--mcdl-hover-surface-danger`, `--mcdl-hover-surface-positive`.
- Text and borders: `--mcdl-hover-text`, `--mcdl-hover-text-accent`, `--mcdl-hover-text-danger`, `--mcdl-hover-border`, `--mcdl-hover-border-strong`, `--mcdl-hover-border-muted`, `--mcdl-hover-border-danger`.
- Shadows and rings: `--mcdl-hover-shadow-card`, `--mcdl-hover-shadow-control`, `--mcdl-hover-shadow-action`, `--mcdl-hover-shadow-danger`, `--mcdl-hover-ring-accent`.
- Media and emphasis: `--mcdl-hover-brightness-action`, `--mcdl-hover-brightness-strong`, `--mcdl-hover-scale-action`, `--mcdl-hover-scale-subtle`, `--mcdl-hover-media-scale`, `--mcdl-hover-icon-shift`.

## Usage Guidance

Prefer adding a shared helper class when the markup can express the semantic category directly:

```jsx
<button className="mcdl-hover-control mcdl-hover-press">Filter</button>
<article className="mcdl-hover-card mcdl-hover-press">...</article>
```

Use direct token consumption when the component has local structure, nested selectors, or a special state:

```css
.example-card {
  transition:
    transform var(--mcdl-hover-duration) var(--mcdl-hover-ease),
    box-shadow var(--mcdl-hover-duration) var(--mcdl-hover-ease),
    border-color var(--mcdl-hover-duration) var(--mcdl-hover-ease);
}

.example-card:hover {
  transform: translateY(var(--mcdl-hover-lift-card));
  border-color: var(--mcdl-hover-border);
  box-shadow: var(--mcdl-hover-shadow-card);
}
```

When hover changes nested media or icons, keep the parent interaction category stable and only token-drive the child motion:

```css
.example-card:hover .example-cover {
  transform: scale(var(--mcdl-hover-media-scale));
}

.example-link:hover svg {
  transform: translateX(var(--mcdl-hover-icon-shift));
}
```

## Prohibited Patterns

- Do not create page-local hover token families such as `--page-hover-*` when an `--mcdl-hover-*` token already fits.
- Do not hard-code near-duplicate timings, easing curves, lift distances, or press scales.
- Do not make desktop and mobile hover behavior visually different just because the width changes.
- Do not use large movement, bounce, rotation, or decorative animation for normal product hover.
- Do not use primary accent hover treatment for quiet menu rows or destructive actions.
- Do not rely on hover as the only way to reveal required functionality on touch devices.
- Do not remove focus-visible treatment when adding hover.

## Reduced Motion

Non-essential transform motion must reduce under `prefers-reduced-motion: reduce`. Shared helper classes already handle this. Custom hover selectors that add transforms must provide an equivalent reduced-motion override or avoid transform entirely.

## Verification

Use these checks when changing hover behavior:

```bash
rg -n "transition:|:hover|:active|:focus-visible" src --glob "*.css" --glob "!src/html/**"
rg -n "hover.*[0-9](ms|s)|translateY\\(-|scale\\(1\\.|cubic-bezier" src --glob "*.css" --glob "!src/html/**"
```

Review matches for page-local timing, easing, lift, scale, shadow, or color values that should be replaced with shared `mcdl` tokens.
