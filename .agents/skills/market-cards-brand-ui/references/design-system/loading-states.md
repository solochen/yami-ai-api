# Loading States

This document organizes the approved skeleton/loading-state rules that already live in `DESIGN.md` and shared UI code. It does not add a new visual direction or change the existing skeleton policy.

## Source Of Truth Split

`DESIGN.md` owns the durable design-system principle for empty and loading states.

`src/components/ui/README.md` owns the shared primitive index.

Component comments own local implementation intent for shared loading helpers such as `MCDLSkeleton`, `MCDLDataListSkeleton`, `MCDLSectionHeader loading`, and `MCDLCardMeta loading`.

`docs/design-system/layout-contracts.md` owns layout-specific loading rules such as fixed bottom CTA spacing.

## Approved Design Rules

The approved design-system rules are:

- Centered layout, generous vertical padding, and restrained copy.
- Dashed ghost-border treatment is acceptable for empty-state framing.
- Non-blocking loads should preserve the final layout with shared skeleton primitives, not page-local duplicate placeholder systems.
- Repeated list and card loading states should live in the same item or card renderer as the loaded state through a `loading` prop or equivalent helper option.
- Skeleton placeholders stay `aria-hidden` and token-driven; use feedback primitives for blocking loading, empty, and error states.

## Shared Skeleton Primitives

Use the shared loading primitives listed in `src/components/ui/README.md` before adding page-local skeleton markup:

- `MCDLSkeleton`
- `MCDLDataListSkeleton`
- `MCDLSectionHeader` with `loading`
- `MCDLCardMeta` with `loading`

`MCDLSkeleton` is the shared non-blocking loading primitive. Callers own the final layout footprint via shared classes/tokens; use page-local skeleton markup only when no shared loading wrapper fits the rendered pattern.

`MCDLDataListSkeleton` is the shared key/value list loading helper for `mcdl` data rows. Use it instead of rebuilding page-local placeholder rows when the loaded state is already expressed as an `mcdl` data list.

`MCDLSectionHeader loading` keeps section header loading inside the shared primitive so header rhythm, sizing, and token usage remain identical between loading and loaded states.

`MCDLCardMeta loading` keeps loading and loaded states on the same structural primitive so pages do not fork card-meta skeleton markup or drift away from shared spacing/token rules.

## Related Layout Rule

If a page only sometimes renders a fixed bottom CTA, select `.mcdl-app-page-shell--cta` only in those states and use `.mcdl-app-page-shell--plain` otherwise. Loading skeletons may use `--cta` only when a fixed CTA skeleton is rendered.
