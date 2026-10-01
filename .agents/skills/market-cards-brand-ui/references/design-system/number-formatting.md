# Number Formatting

User-facing numbers are a product display system, not page-local formatting details. This document owns cross-page display policy. Code-level API behavior lives in `src/utils/numberFormat`, `src/hooks/useDisplayCurrency.js`, and `src/components/ui/AutoFitText.jsx`.

## Shared Formatting

All user-facing numeric display in pages, dialogs, drawers, charts, and confirmations should route through the shared number-formatting system in `src/utils/numberFormat` or related helpers.

Do not hand-roll page-local formatting with `toFixed`, `toLocaleString`, ad hoc `$` or `%` strings, or custom `K/M/B/T` suffix logic.

Pick the display strategy before styling:

| Strategy | Use for |
| --- | --- |
| Fully visible exact value | Detail, trade, confirmation, and financial decision flows. |
| Visually compressed value | Overviews, KPI summaries, and bounded metric blocks where scanning matters. |
| Abbreviated value | Explicitly compact displays where the UI accepts less precision. |
| Truncated identifier-like string | Addresses, hashes, cert strings, and other identifiers, not financial values. |

## MintBox Currency Display

MintBox monetary amounts are product USD values and must display with a `$` prefix.

MintBox amounts do not follow the global currency selector and do not use dual-currency display. This applies to Issue Price, Box Price, Sale Valuation, Target Raise, Total Raised, committed amount, refund, Total Investment, available balance, shortfall, and Ticket amount.

Token quantities, boxes, tickets, card counts, HITS, and percentages are not monetary amounts and keep their own formats.

MintBox product surfaces should avoid user-facing `USDC`; use `USD` or neutral copy like `funds`. Keep `USDC` only for wallet or chain mechanics such as token approvals, wallet balances, and transaction plumbing.

Implementation rule: use `formatUSD` or fixed `$` helpers for MintBox money. Do not use global-currency formatters or `DualCurrencyAmount` for MintBox monetary values.

## Numeric Typography

Use `--mcdl-font-numeric` for prices, balances, valuations, totals, percentages, counts, and stat values. Keep numeric UI visually stable and easy to scan.

Avoid mixing font families inside a single value unless a smaller suffix or unit clearly improves readability.

## Numeric Fit

Use `AutoFitText` for single-line numeric UI that must remain fully visible in bounded width. Keep the authored max size in CSS and let `AutoFitText` shrink only on real overflow.

Use it for balances, prices, valuations, totals, and metric numbers. Do not use it for names, body copy, multi-line text, addresses, transaction hashes, cert strings, or other identifiers with explicit display rules.

Fit still depends on layout: the parent row or cell must allow shrinkage with normal CSS such as `min-width: 0` and flexible grid or flex tracks.
