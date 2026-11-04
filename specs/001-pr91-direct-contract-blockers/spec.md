# Feature Specification: PR #91 Direct Contract Blockers

**Feature Branch**: `PR #91 / feat/sale-identity-fields-2`

**Created**: 2026-11-04

**Status**: Approved to proceed with the recorded autonomous defaults

**Input**: User-reported direct blockers in PR #91: sale discounts, quick-draft values, credit-note fields, purchase import fields and dates, Riyadh effective-date handling, and sale request identity after abandoning a form.

## User Scenarios & Testing

### User Story 1 - Complete a sale accurately (Priority: P1)

A user applies product or manual discounts and submits a sale, or saves a quick draft. The submitted transaction must retain the intended line discounts and valid branch/payment context.

**Why this priority**: Incorrect discounts or missing draft context can make a sale fail or record a total different from what the user saw.

**Independent Test**: Exercise the real sale and quick-draft request paths and inspect the serialized requests and resulting transaction values.

**Acceptance Scenarios**:

1. **Given** a sale with product and manual line discounts, **When** it is submitted, **Then** the transaction preserves the applicable discounts and matches the confirmed backend contract.
2. **Given** a quick draft with a selected store and branch, **When** it is submitted, **Then** it uses valid branch and payment values.
3. **Given** a quick draft missing required context, **When** the user submits it, **Then** no invalid request is sent and the user receives actionable feedback.

### User Story 2 - Issue a credit note or import a purchase (Priority: P1)

A user creates a credit note or imports purchase data. Each workflow must provide the information needed to record the intended transaction without fabricating accounting values.

**Why this priority**: Incomplete credit-note or purchase data can be rejected or recorded incorrectly.

**Independent Test**: Submit each workflow through its actual request path and verify the backend-required fields and line data.

**Acceptance Scenarios**:

1. **Given** a source bill with a provable remaining credit, **When** a credit note is submitted with a reason, **Then** it contains the required branch, stable identity, amount, and every remaining source line.
2. **Given** a source bill with missing line metadata, no remaining lines, or an unreconciled remaining amount, **When** a credit note is opened or submitted, **Then** the workflow blocks it and explains that the remaining credit cannot be verified.
3. **Given** a purchase import with supplier and item data, **When** it is submitted, **Then** it carries supplier sequence, cost price, payment method, and dates in the backend-approved representation.
4. **Given** incomplete or invalid import data, **When** the import is submitted, **Then** the user sees the failure and no success-shaped result is shown.

### User Story 3 - Preserve business dates and sale identity (Priority: P1)

A user imports a purchase effective at Riyadh local midnight or abandons a new-sale form and starts another. The date must retain its intended business meaning, and a distinct sale must not reuse a stale request identity.

**Why this priority**: A shifted effective date can affect ZATCA issue-date correctness, while a reused identity can cause a different sale to conflict with an earlier request.

**Independent Test**: Cover Riyadh midnight boundaries and the new-sale versus retry/resume identity lifecycle.

**Acceptance Scenarios**:

1. **Given** an effective date at Riyadh local midnight, **When** the request is normalized by the backend, **Then** its intended business date remains unchanged.
2. **Given** an abandoned sale followed by a different new sale, **When** the new sale is submitted, **Then** it uses a distinct request identity.
3. **Given** the same sale is retried or resumed, **When** it is submitted again, **Then** it retains its original request identity.

## Edge Cases

- Product discount and manual override are both present, zero, or omitted.
- A store is selected but has no valid branch/payment context.
- A credit note has no remaining creditable lines or its amount cannot be derived safely.
- A prior credit changes the remaining line quantities or total; the source snapshot and POST must agree or submission is blocked/rejected.
- A purchase workbook has missing supplier sequence, cost price, or an ambiguous date cell.
- Riyadh midnight falls on a different UTC calendar date.
- A user abandons a form, refreshes, resumes it, or starts a separate sale in another tab.

## Requirements

### Functional Requirements

- **FR-001**: The sale workflow MUST preserve and submit each applicable line discount according to the confirmed backend contract, with recorded values consistent with the user's preview.
- **FR-002**: Quick-draft creation MUST use valid selected branch and payment values; it MUST block submission when required context is unavailable.
- **FR-003**: Credit-note creation MUST provide the backend-required branch, identity, reason, amount, and note lines using a full-remaining-only policy. It MUST derive the request from a backend-verified snapshot and block if the source remainder cannot be proven.
- **FR-004**: Purchase import MUST submit supplier sequence, item cost price, payment method, and the backend-approved date representation through a working request path.
- **FR-005**: Effective dates MUST preserve the intended Riyadh business date through frontend serialization and backend normalization.
- **FR-006**: A newly started sale MUST receive a distinct request identity after an earlier form is abandoned; retries and resumes of that same sale MUST retain its identity.
- **NFR-001**: Regression tests MUST exercise the actual request construction/submission path, not only a mock that restates expected fields.
- **NFR-002**: Backend accounting formulas and credit-note POST validation MUST remain unchanged; only required line-discount persistence and an additive read-only credit-source snapshot are in scope.

### Key Entities

- **Sale**: A transaction with line items, applicable discounts, and a request identity that remains stable for retries/resume.
- **Quick draft**: An unsaved sale requiring valid branch and payment context.
- **Credit note**: A transaction referencing a source bill and containing the required identity, reason, amount, and note lines.
- **Credit source snapshot**: A backend-derived view of the full creditable remaining quantities and exact amount; the POST revalidates the snapshot transactionally.
- **Purchase import**: Supplier and item data, including sequence, cost, and effective date.
- **Effective date**: The user's intended Riyadh business calendar date, independent of UTC conversion details.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All six reported blockers are either covered by verified existing behavior or fixed with regression tests against the actual request path.
- **SC-002**: Contract tests reject missing required values and zero sentinels only where the backend contract requires a positive/nonzero value; valid zero-valued line discounts remain accepted.
- **SC-003**: Riyadh-midnight tests preserve the expected business date after normalization.
- **SC-004**: New-sale tests show distinct identities after abandonment and stable identities for retries/resumes.
- **SC-005**: The PR head is verified after an independently approved, non-force update to PR #91.

## Assumptions

- Backend #105 and #108 contracts are authoritative after verification against their source and tests.
- PR #91 remains the target; current head is `f14ed79876ae9f9ef6d473dc88da83d6621d5f08`.
- Existing changes may already address some frontend symptoms; current source and actual request tests determine what remains.
- The owner directed autonomous continuation and selected a full-remaining-only credit policy, minimal discount persistence, additive read-only source data, and the existing PR #91 baseline.

## Recorded Decisions

- **Credit-note policy**: Full remaining source-bill credit only; require a reason and stable request identity; derive amount/lines from backend data and block when not provable.
- **Backend scope**: Minimal line-discount persistence fix plus additive read-only credit-source endpoint; preserve accounting formulas and POST validation.
- **Implementation baseline**: Existing PR #91 at `f14ed79`; update only after independent QA and use a non-force push.
