# Afrita QA Change Report

## Scope

This report records the application-wide quality fixes included in the current
frontend and backend contribution. Frontend and backend remain separate
repositories and are reviewed through their repository-specific pull requests.

## Completed changes

- Unified typed search, filter, and pagination state across the dashboard list
  pages, including purchase bills, orders, users, and the dashboard period
  selector.
- Added an explicit all-dates dashboard option and canonical period handling.
- Rejected purchase bills whose calculated total is zero.
- Persisted the purchase-bill PDF policy in the backend settings table and
  reloaded it before rendering add/edit forms.
- Preserved explicit empty settings values, surfaced settings write failures,
  refreshed local state from the backend after saves, and disabled browser
  caching for the settings page.
- Added low-stock notification generation after successful catalog stock
  deductions, using the real `product.quantity` column and each user's saved
  threshold.
- Repaired notification API envelopes, read methods, unread counts, timestamp
  conversion, and partial settings updates.
- Added an idempotent system notification for the current release note. The
  backend creates it on notification list or unread-count access without
  relying on browser storage.
- Added audit timestamps to the canonical backend schema and migration. That
  migration is folded into the backend aggregate branch; publication is
  pending repository access.

## Intensive UI/UX findings resolved

- Corrected the purchase-bill CSV import actions to use the shared outlined
  button variant instead of an undefined class.
- Restored visible `:focus-visible` rings for smart-search clear, recent,
  chip, typed-filter, popover, and hint controls.
- Replaced hardcoded light-theme warning colors in purchase-bill credit
  warnings and the ZATCA company-name warning with the shared warning alert
  tokens, including dark-mode support.
- Replaced the RTL shell's physical sidebar border with a logical border
  property so the layout remains correct when direction changes.
- Prevented the invoice VIN verifier from submitting its parent form.
- Associated shared form-field labels with stable input and textarea IDs.
- Added label associations and browser autocomplete hints to registration
  fields.
- Added accessible previous/next labels and `aria-current` to shared
  pagination.

## Regression coverage

- Frontend handler tests cover settings persistence, empty-value saves, PDF
  policy reloads, dashboard periods, and notification rendering.
- Backend handler tests cover settings write failures, notification contracts,
  low-stock generation, per-user thresholds, and release-note idempotency.
- Full frontend and backend Go build, vet, and test suites pass after SQLC
  regeneration.

## Remaining QA risks

- Live-browser E2E and visual regression checks remain blocked by the local
  review server returning HTTP 401 for the configured admin session.
