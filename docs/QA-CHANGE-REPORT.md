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
  migration is tracked separately until it is folded into the backend
  aggregate branch.

## Regression coverage

- Frontend handler tests cover settings persistence, empty-value saves, PDF
  policy reloads, dashboard periods, and notification rendering.
- Backend handler tests cover settings write failures, notification contracts,
  low-stock generation, per-user thresholds, and release-note idempotency.
- Existing frontend and backend targeted handler suites pass.

## Remaining QA risks

- Full frontend/backend suites and live-browser E2E checks still depend on the
  development backend being healthy.
- The intensive visual pass remains open for cross-page spacing, typography,
  color, responsive behavior, and accessibility consistency.
