# Requirements: Invoice Detail ZATCA QR Rendering

## Problem

The invoice detail endpoint receives persisted ZATCA data from the backend, and
the raw bill parser already retains the persisted `qr_code` value. The detail
handler does not currently expose that value to the template, so the existing
localized QR block never renders for issued invoices. Missing or pending QR
data must remain non-rendering and must not create an unsafe client-side QR
script.

## Goals

- Render a persisted ZATCA QR value on the invoice detail page when one exists.
- Preserve any persisted ZATCA artifact/reference values available from the
  detail response for the detail view data path.
- Keep empty, null, or pending QR data explicit, safe, and free of a QR
  generation attempt.
- Preserve existing localized labels and invoice status/type behavior.

## Scope

### In scope

- The authenticated invoice detail GET handler and its template data map.
- The invoice detail QR conditional and its empty/pending presentation.
- Parsing/pass-through of persisted ZATCA detail metadata already returned by
  the backend.
- Focused handler/template regression coverage.

### Out of scope

- ZATCA monitor behavior or submission statistics.
- ZATCA onboarding flows or configuration schema.
- Outcome, monitoring, or other backend schema changes.
- QR generation, signing, submission, or persistence changes.
- Changes to invoice status calculation or localization architecture.

## User Scenarios

### Scenario 1: Persisted QR is available

1. An authenticated user opens an issued invoice detail page.
2. The backend response contains a non-empty persisted ZATCA QR value.
3. The page renders the localized QR section and uses that persisted value for
   the QR display.

### Scenario 2: QR is empty or pending

1. An authenticated user opens an invoice whose QR value is absent, null, or
   blank because ZATCA processing is pending or unavailable.
2. The page makes the unavailable state explicit using localized presentation.
3. The page does not emit a QR canvas initialization or attempt to synthesize a
   QR value.

### Scenario 3: Existing invoice status remains unchanged

1. An authenticated user opens an invoice in any existing state.
2. The status text, status class, invoice type, and draft/credit behavior remain
   the same as before this change.

## Functional Requirements

- **FR-001**: The invoice detail handler MUST pass the persisted `qr_code`
  value to the invoice detail template data under the `qr_code` key.
- **FR-002**: The handler MUST normalize missing, null, and non-string QR
  values to an empty safe value rather than exposing an unsafe or fabricated
  value.
- **FR-003**: When `qr_code` is non-empty, the template MUST render the
  existing localized QR label and initialize the QR display from that persisted
  value.
- **FR-004**: When `qr_code` is empty, the template MUST NOT render QR
  initialization code and MUST present an explicit localized unavailable or
  pending state.
- **FR-005**: Persisted ZATCA artifact/reference fields available in the raw
  bill response MUST remain available through the invoice detail data path
  without introducing a new backend schema.
- **FR-006**: Existing invoice status, status class, type label, draft controls,
  credit controls, and localized strings MUST remain behaviorally unchanged.
- **FR-007**: A focused regression test MUST fail when `qr_code` is absent from
  handler template data and pass when the data path is corrected.

## Non-Functional Requirements

- The change MUST remain safe for HTML/JavaScript template escaping.
- The change MUST not make an additional backend request solely to obtain the
  QR or artifact data.
- The change MUST preserve the existing request authentication and error
  handling behavior.
- The change MUST be limited to T021 frontend behavior and pass the repository
  Go test suite.

## Acceptance Criteria

- [x] A persisted QR fixture produces an invoice detail response containing the
      localized QR section and persisted QR value.
- [x] An empty/pending QR fixture produces no QR initialization script and an
      explicit localized unavailable/pending state.
- [x] Existing status/type assertions remain unchanged.
- [x] The focused regression test is red on the starting commit and green after
      implementation.
- [x] `gofmt` and targeted tests pass.
- [x] `go test ./...` passes, or any limitation is recorded accurately.
- [x] No monitor, onboarding, or outcome schema files are changed.

## Assumptions, Constraints, Risks, and Unknowns

### Assumptions

- The backend detail response may contain `qr_code` and related persisted
  metadata under stable JSON field names; absent fields are valid.
- Existing `helpers.SafeString` semantics are the appropriate normalization for
  optional persisted strings.
- Existing localization infrastructure can provide the unavailable/pending
  label without changing locale selection.

### Constraints

- Work only in the requested frontend worktree and branch.
- Do not push or create a pull request.
- Commit only isolated T021 changes.

### Risks

- Backend deployments may omit artifact/reference fields even after the
  frontend pass-through is added; the UI must remain usable with those fields
  absent.
- QR values are embedded into a client-side value attribute, so the template
  must continue using contextual escaping and must not bypass it.

### Unknowns resolved during research

- The raw parser already retains `qr_code`, but the detail handler omits it from
  the template map.
- The existing QR template block is already conditional and uses the
  localized `tpl.invoice.qr_code` label.
- The backend-facing frontend contract is raw JSON, so pass-through fields can
  be added without changing frontend models or backend schemas.
