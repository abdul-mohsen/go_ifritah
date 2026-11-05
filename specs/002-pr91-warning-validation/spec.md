# Feature Specification: PR91 Warning-Only Frontend Validation

## User Scenarios & Testing

### User Story 1 - Review a purchase import warning (Priority: P1)

A user maps a purchase workbook containing one or more questionable row values.
The preview identifies the issue as a warning, still allows submission, and shows
the backend's per-row result after submission.

**Acceptance scenarios**

1. Given a serializable purchase payload with a business-invalid row, when the user
   previews it, then a visible warning is shown and the preview remains available.
2. Given the warned preview, when the user submits it, then the backend receives
   the unchanged payload and the row-level response is displayed.

### User Story 2 - Submit a credit note with backend validation (Priority: P1)

A user enters a credit-note reason or identity value that fails frontend guidance.
The form shows guidance but submits the serializable request so the backend can
accept or reject it authoritatively.

**Acceptance scenarios**

1. Given an invalid-but-serializable reason or identity value, when the user submits,
   then the frontend does not return a validation-only 400 before the backend.
2. Given a backend rejection, when the response returns, then the rejection remains
   visible to the user and the submitted identity is not silently replaced.

### User Story 3 - Preserve hard failures (Priority: P2)

A user submits an unreadable upload, malformed request, or unavailable source.
The application reports the hard failure instead of fabricating or forwarding an
unsafe request.

**Acceptance scenarios**

1. Given an unreadable workbook or missing mapping, when the user previews, then the
   existing structural error remains visible.
2. Given an unavailable or unverified credit source, when the user submits, then the
   existing source error remains visible and no fabricated credit note is sent.

## Requirements

### Functional Requirements

- **FR-001**: The application MUST present purchase row business checks as warnings
  when a request can still be serialized.
- **FR-002**: The application MUST allow a warned purchase preview to reach the
  backend without changing the constructed payload.
- **FR-003**: The application MUST display backend per-row purchase results.
- **FR-004**: The application MUST present credit-note field-shape guidance without
  preemptively blocking a serializable request.
- **FR-005**: The application MUST display backend credit-note acceptance or rejection.
- **FR-006**: The application MUST retain explicit failures for malformed transport,
  authentication, CSRF, request-size, and unavailable/unverified source cases.
- **FR-007**: The application MUST preserve existing request identity and retry
  values without silent replacement.

### Key Entities

- Purchase import row
- Credit-note request
- Request identity (`pos_id`, `bill_request_id`)
- Backend validation result
- User-visible warning

## Success Criteria

- **SC-001**: 100% of focused invalid-but-serializable credit-note tests reach the
  backend path instead of returning a frontend-only business-validation 400.
- **SC-002**: 100% of focused purchase preview warning cases reach bulk submission
  without a frontend validation return.
- **SC-003**: No successful or failed retry changes the request identity values.
- **SC-004**: Existing hard-failure tests remain green.
- **SC-005**: Users can distinguish a warning from a backend rejection in the UI.

## Assumptions

- The backend response format and existing HTMX error rendering are sufficient.
- Structural mapping prerequisites are not business acceptance decisions.
- No new persistence or migration is required.

## Out of Scope

- Backend validation or idempotency changes.
- Sales-import feature gating and skipped E2E coverage.
- Purchase bulk-import durable request identity.
