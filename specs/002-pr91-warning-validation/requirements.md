# Requirements: PR91 Warning-Only Frontend Validation

## Problem

PR #91 added frontend-side checks for purchase-bill bulk imports and credit-note
submissions. Some business-invalid values are rejected before the backend sees the
request. This creates a second validation authority, can hide the backend's
field-specific response, and can leave a user unable to recover or retry.

## Goal

Make frontend business validation informative rather than blocking when the request
can still be serialized. Users should see a clear warning, the request should reach
the backend unchanged, and the backend response should remain the authoritative
acceptance or rejection.

## Scope

### In scope

- The purchase-bill bulk-import preview checks introduced by PR #91.
- Credit-note reason and request-field shape checks introduced by PR #91.
- User-visible warning presentation for those checks.
- Rendering backend validation/rejection results after a warning-only submission.
- Focused regression tests for warning-only behavior and preserved backend rejection.
- Spec Kit artifacts documenting the behavior.

### Out of scope

- Authentication, authorization, CSRF, request-size, malformed-encoding, and
  transport failures.
- Backend validation, transaction, idempotency, source-snapshot, or security rules.
- Changing the import mapping prerequisites needed to construct a serializable
  request.
- Re-enabling or redesigning sales import, which is a separate PR #91 gate.
- Changing the credit-note source verification contract.

## User stories

1. As a user importing purchase bills, I want questionable row values called out
   before submission without losing the ability to submit the rows and receive
   backend per-row results.
2. As a user creating a credit note, I want guidance for an empty or malformed
   field, but I want the request to reach the backend so its authoritative error is
   shown and I can recover without a frontend-only dead end.
3. As an operator, I want invalid data to remain rejected by the backend and not be
   silently normalized into a different business request.

## Functional requirements

### FR-001 Warning-only purchase preview

When a purchase-bill row has a business-invalid or incomplete value that can still
be represented in the request payload, the preview must show a visible warning and
must not prevent the user from continuing to the bulk submission step.

### FR-002 Preserve purchase payload

Showing a warning must not silently remove, rewrite, or replace the row values
being submitted. The serialized payload must be the same payload the preview
constructed before the warning was shown.

### FR-003 Backend purchase authority

The bulk-import request must reach the backend for warning-only rows. The UI must
show the backend's per-row success or rejection result, including field-specific
errors.

### FR-004 Warning-only credit-note guidance

Credit-note reason and request identity shape guidance may be shown inline or in the
form, but it must not preemptively return a frontend-generated business-validation
error when the form can be serialized.

### FR-005 Backend credit-note authority

The credit-note request must reach the backend with the user's submitted values
unchanged whenever the frontend can construct the source-backed request. Backend
success, rejection, and recoverable source errors must continue to be rendered using
the existing response path.

### FR-006 Preserve hard failures

The frontend may continue to block or fail explicitly when it cannot construct a
safe, serializable request, including malformed transport input, unavailable or
unverified source data, authentication failure, request-size overflow, and CSRF
failure.

### FR-007 No silent identity changes

Warning-only handling must not generate a replacement business identity, alter a
retry identity, or make a failed request appear successful.

### FR-008 Regression coverage

Tests must prove that an invalid-but-serializable credit-note submission reaches the
backend and renders the backend rejection, and that purchase bulk-import warnings
do not prevent submission while backend row results remain visible.

## Non-functional requirements

- Warnings must be understandable in the existing Arabic/English UI.
- Existing successful purchase import and credit-note flows must remain successful.
- The change must not weaken backend validation or durable idempotency.
- The warning path must not add an unbounded retry loop or leave controls permanently
  disabled.

## Acceptance criteria

- AC-1: A purchase import with a row-level business warning reaches the bulk endpoint
  and displays the backend's row-level response.
- AC-2: A credit-note form with invalid-but-serializable reason or identity shape
  reaches the backend; the frontend does not return its own blocking 400 first.
- AC-3: The submitted payload preserves user values and identity fields.
- AC-4: Existing hard failures and valid successful flows retain their current
  behavior.
- AC-5: Focused Go/JavaScript tests and the repository's required checks pass.
- AC-6: Relevant user-facing documentation or status artifacts state that frontend
  business validation is warning-only and backend validation is authoritative.

## Assumptions

- The backend endpoints already return usable error bodies for invalid purchase rows
  and credit-note requests.
- Existing response rendering can show backend rejection without a new API contract.
- Mapping prerequisites remain blocking only when no meaningful serializable request
  can be constructed.

## Constraints

- Keep the change limited to PR #91's validation boundary.
- Do not alter unrelated pre-existing form validation.
- Do not push or write to GitHub from this task without separate authorization.

## Risks

- Removing a frontend gate may expose more backend error responses; tests must verify
  those responses remain visible and actionable.
- A warning-only preview could let users submit several invalid rows; per-row backend
  results must make failures explicit and preserve successful rows.

## Unknowns resolved during research

- The active PR review explicitly requests warning-only frontend business validation
  and identifies the purchase bulk validator and credit-note preflight as the
  affected paths.
