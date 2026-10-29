# T023 Research

## Repository evidence

- `handlers/zatca_monitor.go` at baseline `18f7988` returned fixed statistics,
  four fixed branch rows, and ten fixed submission rows.
- `templates/zatca-monitor.html` consumed those values directly and had no
  backend error or empty-state handling. Its refresh action only reloaded the
  page.
- `helpers.GetTokenOrRedirect` validates the session cookie and refreshes an
  expiring token; `helpers.DoAuthedRequestWithRetry` is the established
  authenticated proxy with 401 refresh support.
- `templates/components/error-alert.html` already provides a localized
  `role="alert"` rendering pattern when `.error` is present.
- Existing handler tests use `httptest.Server`, override `config.BackendDomain`,
  inject `helpers.HttpClient`, and register a session token in
  `config.SessionTokens`.
- Existing monitor localization keys cover the original summary, table, and
  basic statuses, but not load/empty/unknown/extended provider/pending-age
  messages.

## Contract evidence

The sibling backend contract document
`specs/003-zatca-submission-state/contracts/zatca-monitor.md` and its model define:

- `GET /api/v2/zatca/monitor/stats`
- `GET /api/v2/zatca/monitor/branches`
- `GET /api/v2/zatca/monitor/submissions`

The backend implementation returns:

- Stats: `total_submitted`, `accepted`, `accepted_with_warning`, `warnings`,
  `rejected`, `ambiguous`, `terminal_failure`, `pending`.
- Branches: `branch_id`, `branch_name`, numeric `zatca_status`, nullable
  `cert_expiry`, `today_count`, `success_rate`, nullable
  `last_submission_at`.
- Submissions: `invoice_id`, `invoice_no`, nullable `branch_id`, `branch_name`,
  explicit `status`/`state`, `status_code`, `document_id`, provider code and
  warning/reference fields, retry/timestamp fields, `zatca_ref`, and
  `warning_msg`.
- Submissions accept the `limit` query parameter; the documented frontend
  request is `limit=50`.

Numeric branch status meanings are the established values: `0` deleted,
`1` active, `2` expired, `3` not active; `4` is retained for expiring-soon
compatibility in the frontend.

## Decisions

1. Use the session ID with `DoAuthedRequestWithRetry` for each request instead
   of forwarding a token manually. This preserves existing refresh behavior and
   ensures every backend request is authenticated.
2. Fetch the three independent read-only endpoints concurrently, then retain
   successful sections when another section fails. A per-section error is
   visible; failed sections render placeholders rather than zeros/mock data.
3. Keep all backend response fields in frontend structs needed by the page,
   with pointers for nullable provider/timestamp values. Normalize legacy
   `warning` to `accepted_with_warning` for presentation while preserving
   unknown states as unknown.
4. Compute pending age only for pending rows from the backend timestamp. If
   parsing fails, display an unavailable marker rather than inventing an age.
5. Add localized labels for new states and data conditions. Do not remove
   existing keys or add a retry/resubmit action.
6. Preserve the existing invoice-type column but render an unavailable marker
   because the documented monitor contract does not provide invoice type.

## Unresolved uncertainty

The currently checked-out frontend has no browser-level async loading
endpoint. Loading is therefore represented by an explicit loading/refresh
indicator in the server-rendered page and by section placeholders while
backend data is unavailable; the handler remains the authoritative source of
truth and never renders a successful mock state.
