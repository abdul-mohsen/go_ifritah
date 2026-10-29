# Research: T005 Frontend C2 Submission Outcome UI

## Repository baseline

- Frontend worktree: `C:\ssda\chatGPT\worktrees\zatca-fix-t005-frontend`
- Branch: `fix/zatca/c2-submission-ui`
- Starting commit: `18f7988` (`frontend main`)
- Working tree was clean before T005 work.
- Frontend is a Go HTTP/HTML-template application. Invoice list rendering is in
  `handlers/invoices.go` and `templates/invoices.html`; invoice detail rendering
  is in the same handler file and `templates/invoice-detail.html`.

## Existing frontend behavior

1. `helpers.InvoiceStatus(models.Invoice)` maps only legacy numeric
   `CreditState`/`State` values. It maps state `3` to `صادرة`/`badge-issued`,
   state `1` to `قيد المعالجة`/`badge-pending`, and state `2` to
   `تمت المعالجة`/`badge-issued`.
2. `HandleInvoices` fetches `/api/v2/bill/all`, converts each invoice into a
   map, and the list template derives edit/credit/delete links from numeric
   `.state` and `.credit_state`.
3. `HandleGetInvoice` fetches `/api/v2/bill/:id`, parses the raw bill, and
   passes `status`, `status_class`, `is_draft`, and invoice data to the detail
   template. The detail template shows submit/delete/edit actions for drafts.
4. `models.Invoice` does not currently carry durable ZATCA submission fields.
   `helpers.ParseBillRaw` preserves a limited `extra` map and currently ignores
   the approved submission outcome fields.
5. The existing ZATCA monitor page is explicitly mock data and is out of scope
   for T005.

## Approved backend/daemon contract evidence

Read-only inspection of the related T003 backend and T004 daemon worktrees
confirmed:

- T003's monitor route is
  `GET /api/v2/zatca/monitor/submissions`.
- Each response row includes `invoice_id`, `state` (and compatibility
  `status`), `document_id`, `provider_code`, `provider_warning_code`,
  `provider_warning`, `provider_reference`, `retry_count`,
  `last_attempt_at`, `next_retry_at`, `request_xml_path`,
  `cleared_xml_path`, and `outcome_at`.
- Allowed durable states are exactly
  `pending`, `accepted`, `accepted_with_warning`, `rejected`, `ambiguous`, and
  `terminal_failure`.
- T004 daemon processing sets legacy bill `state=3` only for `accepted` and
  `accepted_with_warning`; rejected, pending, ambiguous, and terminal failures
  remain unissued.
- T003's bill list SQL does not include durable submission columns, so the
  frontend must merge monitor rows by `invoice_id` rather than expecting bill
  list/detail payloads to contain the new fields.
- The contract has no approved frontend retry route. Retryability is therefore
  carried as page data, but no new retry action is created by T005.

## Localization evidence

Existing resource keys include localized legacy invoice statuses:

- `tpl.invoice.status_draft`
- `tpl.invoice.status_processing`
- `tpl.invoice.status_processed`
- `tpl.invoice.status_issued`
- `invoice_status.*`
- ZATCA monitor accepted/warning/rejected/pending labels.

The implementation should reuse `resources.L` and add only narrowly scoped
durable-outcome keys if no existing key expresses a distinct state. Existing
legacy labels must remain the fallback.

## Test evidence and red plan

Existing handler tests use `httptest.Server`, session tokens, and template
rendering. The focused T005 regression will return:

1. a bill list containing invoices whose legacy numeric state would otherwise
   make the generic helper render `قيد المعالجة` or `صادرة`; and
2. monitor rows with explicit rejected, ambiguous, and pending states.

The test will assert each page row/detail carries the explicit status and that
the issued-only presentation/action is absent for all three non-success
outcomes. On the base implementation, monitor data is never fetched/merged,
so the test must fail before source implementation.

## Contract mismatch report

The following limitations are recorded rather than hidden:

1. T003 mounts the monitor submissions route behind admin middleware, while
   invoice list/detail routes are RBAC-protected for any role with invoice-view
   permission. A non-admin invoice viewer may receive 403 and therefore falls
   back to legacy numeric status because T003 does not expose durable outcome
   fields in the bill list/detail response.
2. T003 exposes retry count and next-retry timestamps but does not define an
   explicit `retryable` boolean or a frontend retry endpoint. The adapter
   accepts optional `retryable`/`can_retry` fields when deployments provide
   them, carries the metadata into detail data, and deliberately renders no
   invented retry action.

Older deployments may also omit the monitor route or return only `status`; the
adapter accepts both keys and falls back to legacy invoice mapping when the
monitor read is unavailable. No backend/daemon changes are included.
