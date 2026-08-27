# Afrita Release Notes

## Quality and reliability update

- Search and filters now keep the same typed-query and pagination behavior
  across dashboard list pages.
- Dashboard defaults to the current quarter and provides separate quarter and
  year selectors with a read-only resolved date range; custom date ranges
  remain available separately.
- The dashboard now labels the annual period "By Year" and lists only years
  containing sales or purchase-bill data. Its filter controls also stay aligned
  on narrow screens.
- Sales and purchase bill Excel exports now include Date, Total Before VAT, VAT,
  and Total columns on the Bills sheet.
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
