# Afrita Release Notes

## Quality and reliability update

- Search and filters now keep the same typed-query and pagination behavior
  across dashboard list pages.
- Dashboard date presets include an explicit all-dates option.
- Zero-total purchase bills are rejected before submission.
- Purchase-bill PDF requirements are saved in the database and applied after
  reloads and service restarts.
- Settings can be cleared intentionally, failed writes are surfaced, and the
  settings page is not served from browser cache.
- Low-stock notifications use live inventory quantities and saved user
  thresholds.
- Notification listing, unread counts, read actions, and release-note
  delivery now use one consistent contract.
- Purchase-bill CSV actions now use the shared outlined button style.
- Smart-search controls now expose a consistent keyboard focus ring.
- Warning banners use shared theme tokens and remain readable in dark mode.
- Invoice VIN verification no longer submits the invoice form.
- Shared form fields, registration fields, and pagination now expose clearer
  labels and keyboard/screen-reader semantics.
