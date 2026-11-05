# Design: PR91 Warning-Only Frontend Validation

## Boundary

The frontend remains responsible for collecting input, showing guidance, and
rendering backend responses. It is not responsible for deciding whether a
business-invalid purchase row or credit note is accepted.

## Purchase bulk-import flow

1. Build the existing `payloads` from the workbook mapping.
2. Run the existing row checks only to collect a warning message.
3. Show the warning in the preview step without returning from the preview action.
4. Keep the existing bulk submit button enabled and submit the unchanged payload.
5. Render the existing structured per-row backend results, including validation
   errors and partial success.

The warning container is cleared whenever a new preview is built or the import is
reset. A warning does not mutate `parsedBillsForImport`.

## Credit-note flow

1. Parse the form and retain the submitted strings.
2. Use the bill identifier to request the source snapshot needed to build the
   source-backed payload.
3. Do not reject reason, POS identity, request identity, or other business-field
   shape in the frontend before the backend request.
4. Submit the source-backed payload unchanged to the backend.
5. Render the backend response through the existing HTMX error/success path.

If the source cannot be retrieved or verified, the frontend reports that explicit
hard failure and does not fabricate a payload. This is a prerequisite failure, not
business acceptance validation.

## User feedback

- Purchase-import warnings use the existing localized labels and are shown inline
  near the preview summary.
- Credit-note reason guidance is shown inline below the field and updates as the
  user edits it. It does not call `preventDefault`, set native invalid state, or
  replace the backend response.
- Backend errors remain visible through the existing form response target.

## Data integrity

- No frontend normalization is introduced by this change.
- `pos_id` and `bill_request_id` remain exactly as supplied by the existing identity
  helper.
- Backend validation, source verification, transactionality, and idempotency remain
  unchanged.

## Error handling

- Structural upload/mapping failures: explicit existing failure path.
- Warning-only business checks: visible warning, continue.
- Backend rejection: structured response, visible to the user.
- Authentication, CSRF, transport, request-size, or unverified-source failure:
  existing explicit error path.
- Malformed UTF-8 in forwarded credit-note text: explicit hard failure before
  source retrieval so serialization cannot replace user bytes.
- Incomplete purchase rows with any mapped product input: retain for warning and
  backend reporting; only fully empty rows are ignored.

## Test strategy

- Update the credit-note handler test to prove invalid-but-serializable fields reach
  the backend.
- Add a test that backend credit-note rejection is returned rather than replaced by
  frontend validation.
- Add JavaScript coverage for purchase preview warning behavior if the existing
  test harness can load the template helper without a browser.
- Run focused Go and JavaScript tests, then formatting, vet, lint, and the repository
  check command.

## Traceability

| Requirement | Design element |
|---|---|
| FR-001, FR-002 | Purchase warning collection and unchanged preview payload |
| FR-003 | Existing structured bulk endpoint response rendering |
| FR-004, FR-005 | Credit-note preflight removal and inline guidance |
| FR-006 | Structural/source/transport hard-failure paths |
| FR-007 | Existing identity helper and no normalization |
| FR-008 | Focused handler and JavaScript regression tests |
