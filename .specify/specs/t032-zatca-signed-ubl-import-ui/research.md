# T032 Research

## Evidence inspected

1. Frontend baseline is clean at `d7b5034` on
   `fix/zatca/m4-import-ui`. Existing authenticated pages use
   `helpers.GetTokenOrRedirect`, `helpers.Render`, and backend proxy handlers
   in `handlers/`.
2. Frontend routes are registered in `main.go`; templates are preloaded by
   `config.LoadTemplates`, and localized labels are server-side `resources.L`
   keys.
3. Backend commit `94aa8e2` adds authenticated T031 routes:
   `POST/GET /api/v2/zatca/import`, `GET /api/v2/zatca/import/:id`, and
   `POST /api/v2/zatca/import/:id/{review,post,reject}`, with plural
   `/imports` aliases.
4. T031 initial status is `review`; allowed lifecycle statuses are `review`,
   `approved`, `posted`, and `rejected`. The backend accepts `xml_content`,
   derives source hash/size when bytes are present, persists immutable
   evidence, and requires a reason for reject.
5. T031 detail responses include `evidence`, `validation_errors`, `reviews`,
   and `status_history`; list responses omit large evidence blobs.
6. T031 explicitly separates import workflow state from outbound
   `zatca_submission` state. `posted` records import workflow completion and
   does not mean an invoice is issued.
7. Existing frontend settings/onboarding and ZATCA monitor templates use
   localized `L` keys and no shared import state. Existing template tests
   enumerate every loaded page, so new templates must be registered there.

## Decisions

### Decision 1 — Use frontend proxy routes, not browser-to-backend calls

The frontend already owns session-token refresh and CSRF middleware. Proxying
through `/api/zatca/import...` preserves those boundaries and keeps backend
URLs/tokens out of browser code.

### Decision 2 — Upload raw XML as `xml_content`

T031 accepts `xml_content` as a UTF-8 convenience alias and derives a verified
hash/size. This avoids base64 inflation in browser JavaScript while retaining
the exact source bytes as interpreted by the backend. The UI marks uploads
review-required and unverified; it does not pretend to run T030 parsing.

### Decision 3 — Derive a display-only failed state from evidence

T031 intentionally keeps lifecycle status at `review` for a failed or
review-required import. The frontend derives `failed` only when a failure class
or error-severity validation issue exists, while preserving the raw lifecycle
status for action gating.

### Decision 4 — Add list and detail pages with explicit action gating

The list page handles upload/filter/status overview. The detail page handles
evidence inspection and review/post/reject actions. Post is only shown for
raw `approved`; reject is only shown for raw `review`/`approved`.

## Remaining uncertainty

The browserless suite cannot validate actual File API encoding, CSRF headers,
or live MySQL/object-storage persistence. These are reported as limitations
after local validation.
