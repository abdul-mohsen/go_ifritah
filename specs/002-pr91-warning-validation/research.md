# Research: PR91 Warning-Only Frontend Validation

## Evidence reviewed

### Handover

- `HANDOVER.md` identifies `go_ifritah #91` as open and records the latest review
  hold at commit `2b96ec3`, later updated by the local branch to `967ea49`.
- The handover narrows the active PR blockers and distinguishes direct frontend
  contract work from stack-level release gates.
- The handover states that frontend rendering is not the authority for backend
  validation, totals, permissions, or durable business decisions.

### Pull request discussion

- PR comment `6644057191` at `187357e` says credit-note frontend support is deferred
  rather than claiming compatibility with backend #108.
- The latest review comment at `967ea49` requires:
  - invalid-but-serializable credit-note values to reach the backend;
  - no frontend-generated blocking 400 before backend validation;
  - purchase bulk-import validation to become warnings/reporting;
  - backend rejection to remain authoritative and visible.
- The same review separately identifies skipped E2E coverage, sales-import gating,
  attempt-key reliability, and bulk-import idempotency. Those are outside this
  warning-only change.

### Local implementation

- `templates/import-purchase-bills.html` defines
  `validatePurchaseImportBills`, which returns an error string and `previewBills`
  stops before showing the preview when it finds a business-invalid row.
- `handlers/invoices.go` validates `bill_id`, reason, `pos_id`, and
  `bill_request_id` before fetching the credit source and returns a frontend
  `400` for invalid values.
- `templates/add-credit-note.html` uses native `required` validation for the
  reason field, which can prevent the request from reaching the backend.
- `handlers/purchase_bill_bulk_import.go` already produces per-item validation
  results and returns a structured success response; this is the correct
  authoritative result path to preserve.
- `handlers/credit_note_test.go` currently asserts that invalid input is rejected
  before any source request; that test encodes the behavior the review asks to
  change.

## Decisions

1. Keep structural prerequisites that make a request impossible to construct
   (missing workbook/sheet/mapping, unreadable file, malformed HTTP body) as explicit
   failures.
2. Convert row/business checks in the purchase preview into a visible warning while
   continuing to the existing preview and submit flow.
3. Remove the credit-note frontend preflight gate for business field shape. Keep
   source retrieval and source verification because they are required to construct
   the source-backed request and are not a duplicate of backend acceptance.
4. Remove native `required` blocking for credit-note reason and provide inline
   guidance; the request still flows through HTMX to the backend.
5. Add focused tests at the handler boundary and, where practical, a JavaScript
   test for warning-only preview behavior.
6. Preserve malformed UTF-8 rejection as a hard failure because JSON serialization
   would otherwise replace invalid bytes and change the submitted value.
7. Retain incomplete purchase rows that contain any mapped product input so they
   can be warned and reported by the backend; discard only genuinely empty rows.

## Alternatives considered

- **Keep the frontend 400 and duplicate backend validation:** rejected because the
  active PR review explicitly identifies this as a second blocking authority.
- **Silently allow invalid values without warning:** rejected because users need
  actionable feedback and the requirement calls for visible warnings.
- **Normalize invalid values before submission:** rejected because it can change the
  business request and retry identity.
- **Remove all client-side checks, including structural upload checks:** rejected
  because some failures mean there is no safe request to serialize.

## Remaining uncertainty

- Full browser/E2E execution requires the repository's authenticated test services.
  Focused unit/JavaScript checks will be run locally; unavailable external checks
  will be recorded in `STATUS.md`.
