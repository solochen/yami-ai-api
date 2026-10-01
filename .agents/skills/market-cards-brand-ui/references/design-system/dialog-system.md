# Dialog System

Market Cards uses semantic overlay kinds instead of choosing business components by physical shape. The resolver lives in `src/components/ui/overlayResolver.js`; the wrapper lives in `src/components/ui/MCDLResponsiveOverlay.jsx`; low-level portal, stacking, dismissal, animation, and scroll lock remain in `src/components/ui/MCDLDialog.jsx`.

## Where This Lives

This document owns cross-component overlay policy: semantic kinds, presentation rules, prohibited patterns, migration status, and verification commands.

Code comments should stay short and local. `src/styles/mcdl-dialog.css` owns visual selector mechanics, while `src/components/ui/MCDLDialog.jsx`, `src/components/ui/MCDLResponsiveOverlay.jsx`, and `src/components/ui/overlayResolver.js` own runtime behavior.

## Semantic Kinds

| Kind | Use for | Desktop presentation | Mobile presentation | Reason |
| --- | --- | --- | --- | --- |
| `selection` | sort, filter, time range, card/status/type filters | anchored popover when the trigger is supplied; centered modal without an anchor | bottom sheet | Selection should stay close to the control on desktop and remain thumb-friendly on mobile. |
| `form` | withdraw address, MintBox commit, invite binding, trade form | centered modal | bottom/high sheet | Forms need stable focus and keyboard room; desktop should not reuse mobile bottom sheets. |
| `confirm` | trade, cancel, claim, withdraw, commitment confirmations | centered modal | centered modal by default | Risk decisions deserve interruption and focus on both platforms. |
| `status` | loading, success, error process states | centered modal | centered modal | Process feedback should be compact, focused, and independent from trigger location. |
| `detail` | code lists, poster preview, explanatory copy, wallet detail | centered modal | centered modal for short content; sheet for long content | Details are reading surfaces; long mobile content needs scrollable sheet space. |

Use `MCDLResponsiveOverlay` for new business overlays. Use raw `MCDLDialog` only when a component owns a custom frame, and always pass `kind`. `MCDLBottomSheet` is now a mobile-only primitive/compatibility wrapper; business pages should not render `<MCDLBottomSheet>` directly.

## Mobile Header Contract

Header-to-content spacing is measured from the header row bottom edge, or from the divider bottom edge when a divider exists, to the first content component's outer edge. The dialog frame owns this spacing; first content components must not add top margin to create it.

| Mobile pattern | Title | Divider | Header-to-content gap |
| --- | --- | --- | --- |
| Bottom sheet selection | left aligned | none | `0.75rem` |
| Bottom sheet form or long detail | left aligned | none | `1rem` |
| Center review or transaction | centered | required | `1rem` after divider |
| Center status or short decision | centered in content | none | owned by the status/decision panel |
| Center short detail | left aligned | none | `1rem` |
| Third-party tool modal | local constraint | allowed exception | `0.75rem` to `1rem` |

If a dialog title has a subtitle, use `0.375rem` from title to subtitle and `1rem` from subtitle to the main content. Prefer a dialog variant or token for exceptions; do not add page-local `margin-top` to the first body child.

Center status and short-decision dialogs must use `AsyncStagePanel` for the icon, title, description, detail, and progress state. Do not create page-local status icons or copy groups. Use `.mcdl-dialog-sheet--status-panel` for the centered frame; add actions after the panel only when the state needs a user decision or acknowledgement.

## Viewport Sizing And Scroll Contract

Dialog height must be content-driven until the dialog reaches the usable viewport. The frame should then stop growing and the body region should become the only vertically scrollable area.

- Desktop centered modals must not use a fixed `rem`/`px` height cap as their primary maximum height. Use the available viewport height minus overlay padding, so large displays can reveal more content without unnecessary scrollbars.
- `MCDLResponsiveOverlay` desktop modal sizing follows this contract with `max-height: calc(100dvh - (var(--mcdl-dialog-overlay-padding) * 2))`, with a `100vh` fallback. Mobile and bottom-sheet presentations may keep stricter caps because they need thumb-friendly boundaries and safe-area room.
- Dialogs should keep header and footer/action regions visible when content overflows. Put vertical overflow on the body/scroll region rather than the whole sheet whenever the frame owns a header or action row.
- Fixed maximum heights are allowed only for physically constrained surfaces such as anchored popovers, poster previews, or specialized media/art panels. If a business dialog needs one, document the reason in the local component stylesheet.

This contract follows the same broad pattern used by major design systems: Carbon modals recommend moving to larger modal sizes or full pages when capped content becomes too scroll-heavy; Bootstrap provides viewport-bounded scrollable modals; SAP Fiori dialogs grow with content up to the available screen; WAI-ARIA guidance for modal dialogs highlights careful focus handling for large scrollable content. References:

- Carbon modal usage: https://carbondesignsystem.com/components/modal/usage/
- Bootstrap modal docs: https://getbootstrap.com/docs/5.2/components/modal/
- SAP Fiori dialog usage: https://www.sap.com/design-system/fiori-design-android/v24-12/components/system-components/dialogs/usage
- WAI-ARIA modal dialog pattern: https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/

## Runtime Behavior Contract

Shared dialog motion and behavior must stay aligned across CSS and React primitives:

- Enter animation: backdrop fades in while mobile sheets slide up from the bottom.
- Exit animation: close transitions must fully play before unmount.
- State reset timing: form values, loading state, and success state reset in `onAfterClose`, not during the close click or exit frame.
- Backdrop dismiss: enabled by default; critical flows such as submitting, signing, or waiting for chain confirmation must explicitly disable it.
- Escape dismiss: follows the backdrop policy by default, and only the top-most dialog may respond when dialogs are stacked.
- Close button: enabled by default. If backdrop dismiss is disabled for a flow, the close button should follow the same policy unless there is a strong UX reason.
- Scroll lock: opening any dialog locks body scroll; stacked dialogs must use reference counting so scroll is restored only after the last dialog closes.
- Layering: regular sheets use the base dialog layer. Nested confirms and progress dialogs should opt into the stacked layer so only the top sheet handles dismissal.
- Third-party popup anchoring: DatePicker, Select, and other popup components rendered inside dialogs must not mount into `.mcdl-dialog-sheet`, `.mcdl-dialog-scroll-area`, or a trigger parent inside them. Mount to `.mcdl-dialog-root` or `document.body` via `resolveAntdPopupContainer`.

## Prohibited Patterns

- Do not add a new business dialog by importing `MCDLBottomSheet` directly.
- Do not use unclassified `<MCDLDialog>` in pages or business components.
- Do not mount AntD `DatePicker` or `Select` popups inside `.mcdl-dialog-sheet` or `.mcdl-dialog-scroll-area`; use `resolveAntdPopupContainer`.
- Do not reset form or stage state during the close click if the component already uses `onAfterClose` for that timing.
- Do not add desktop bottom sheets for `form`, `confirm`, `status`, or `detail`.
- Do not add fixed desktop modal height caps that make large screens scroll earlier than the usable viewport requires.

## Inventory And Migration Checklist

| File | Overlay | Kind | Current migration status |
| --- | --- | --- | --- |
| `src/components/ui/MCDLDialog.jsx` | low-level dialog | primitive | Added semantic resolver support through `kind` and presentation metadata. |
| `src/components/ui/MCDLResponsiveOverlay.jsx` | responsive semantic overlay | primitive | Added. |
| `src/components/ui/MCDLBottomSheet.jsx` | bottom sheet wrapper | primitive | Kept as mobile-only/compatibility primitive. |
| `src/components/TimeFilterDialog.jsx` | time filter | `selection` | Migrated to `MCDLResponsiveOverlay`; desktop trigger support uses popover. |
| `src/components/CardFilterDialog.jsx` | card filter | `selection` | Migrated to semantic overlay. |
| `src/components/MarketFilterDialog.jsx` | market activity filter | `selection` | Migrated to semantic overlay. |
| `src/components/AccountFundTypeFilterDialog.jsx` | fund type filter | `selection` | Migrated to semantic overlay. |
| `src/components/IPOStatusFilterDialog.jsx` | MintBox status filter | `selection` | Migrated to semantic overlay. |
| `src/pages/MarketList.jsx` | mobile sort/filter sheets | `selection` | Direct `<MCDLBottomSheet>` removed; semantic overlay used. |
| `src/pages/Referral.jsx` | mobile referral sort | `selection` | Direct `<MCDLBottomSheet>` removed; semantic overlay used. |
| `src/pages/IPOHistoricalList.jsx` | mobile historical sort | `selection` | Direct `<MCDLBottomSheet>` removed; semantic overlay used. |
| `src/pages/FundHistoryDesktop.jsx` | desktop time filter | `selection` | Time trigger moved into anchored semantic popover. |
| `src/pages/MarketActivitiesDesktop.jsx` | desktop time filter | `selection` | Time trigger moved into anchored semantic popover. |
| `src/components/USDCWithdrawDialog.jsx` | withdraw address form | `form` | Classified; desktop resolves centered, mobile remains sheet. |
| `src/components/IPOSubscribeDialog.jsx` | MintBox commit form | `form` | Classified; desktop resolves centered, mobile remains sheet. |
| `src/components/ReferralInviteDialog.jsx` | invite bind form | `form` | Classified; desktop resolves centered, mobile remains sheet. |
| `src/components/WalletDialog.jsx` | wallet detail/action hub | `detail` | Classified as long detail; desktop centered, mobile sheet. |
| `src/pages/CardDetail.jsx` | mobile trade form | `form` | Classified; mobile remains sheet. |
| `src/components/TradeConfirmDialog.jsx` | trade confirm/status | `confirm`/`status` | Confirm stage now resolves centered; status remains centered. |
| `src/components/IPOSubscribeConfirmDialog.jsx` | commitment confirm/status | `confirm`/`status` | Confirm, loading, and success resolve centered. |
| `src/components/IPOReminderConfirmDialog.jsx` | reminder status | `status` | Classified; now resolves centered. |
| `src/components/IPOTicketDetailDialogBase.jsx` | MintBox Ticket list | `detail` | Classified as long detail; desktop centered, mobile sheet. |
| `src/components/CancelOrderDialog.jsx` | cancel confirm/status | `confirm`/`status` | Classified; already centered. |
| `src/components/WithdrawConfirmDialog.jsx` | withdraw confirm/status | `confirm`/`status` | Classified; already centered. |
| `src/components/WithdrawProgressDialog.jsx` | withdraw progress | `status` | Classified; already centered. |
| `src/components/IPOClaimConfirmDialog.jsx` | MintBox claim confirm/status | `confirm`/`status` | Hand-written confirm overlay replaced with `MCDLDialog`; progress uses `WithdrawProgressDialog`. |
| `src/components/ReferralRewardClaimConfirmDialog.jsx` | reward claim confirm/status | `confirm`/`status` | Hand-written confirm overlay replaced with `MCDLDialog`; progress uses `WithdrawProgressDialog`. |
| `src/components/IPOReferralBoostInfoDialog.jsx` | referral boost info | `detail` | Classified; already centered. |
| `src/components/ReferralInviteDialog.jsx` | poster preview | `detail` | Hand-written portal overlay replaced with `MCDLDialog`. |
| `src/pages/MyAccount.jsx` | info modal | `detail` | Hand-written modal replaced with `MCDLDialog`. |
| `src/components/HeaderDesktop.jsx` | language/currency/notice/profile popovers | `selection`/menu | Already desktop popovers; kept as anchor-first desktop behavior. |
| `src/pages/MarketActivitiesDesktop.jsx` | status/direction/type popovers | `selection` | Already desktop popovers; kept. |
| `src/pages/UserSettingsContent.jsx` | avatar crop AntD modal, language `Select` popup | third-party form/detail | Kept as third-party exception; crop modal is already centered and `Select` uses `resolveAntdPopupContainer`. |
| `src/components/AppDrawers.jsx`, `src/components/Layout.jsx` | menu/profile/navigation drawers | navigation drawer | Documented exception: route/account navigation drawers are not transactional dialogs and keep drawer presentation. |

## Verification Commands

- `node --test src/components/ui/__tests__/overlayResolver.test.js`
- `npm run build`
- `rg -n "<MCDLBottomSheet\\b|import \\{[^}]*MCDLBottomSheet[,\\n}]|import MCDLBottomSheet" src/pages src/components src/contexts -g '*.jsx' -g '*.js'`

The final grep should not report business-level `<MCDLBottomSheet>` usage. `MCDLBottomSheetList`, `MCDLBottomSheetOption`, and `MCDLBottomSheetFooterButton` can remain as option/footer primitives inside semantic overlays.
