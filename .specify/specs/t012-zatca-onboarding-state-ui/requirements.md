# T012 ZATCA Onboarding State UI

## Problem

The settings page currently knows only the branch's numeric ZATCA status. The
T010/T011 contract now persists a separate onboarding state, worker step,
retry count, bounded error code/message, and completion state. Without consuming
that metadata, the page can show a branch as active while onboarding is still
queued or has failed, hide useful progress, continue polling after a terminal
failure, and provide no localized recovery guidance.

## Goals

- Render durable onboarding state and step progress in the existing ZATCA
  settings experience.
- Show localized guidance for invalid OTP, provider-unavailable, and generic
  onboarding failures, including retry guidance.
- Treat a branch as connected only after the durable state is complete; never
  infer active status from a stale numeric status while onboarding is pending or
  failed.
- Stop polling as soon as the durable attempt reaches `failed` or `done`.
- Preserve T028's complete-save ordering, existing settings layout, and
  successful configuration flow.

## Scope

### In scope

- Frontend ZATCA metadata projection in the settings handler.
- `templates/settings.html` progress, failure, retry, completion, status, and
  polling behavior.
- Frontend localization labels required for the new guidance.
- Focused handler and template-source regression tests for invalid OTP,
  provider failure, and success/completion.
- Directly related Spec Kit artifacts and validation.

### Out of scope

- Backend or daemon code, schema, migrations, queue messages, or provider APIs.
- Changes to the T028 save-order implementation except compatibility guards
  needed to consume the new state.
- Reusing or persisting OTPs.
- New polling services, dependencies, browser automation, or unrelated settings
  UX.

## User scenarios and testing

### Scenario 1: OTP is rejected durably

1. The user submits a validly formatted OTP.
2. The daemon records `onboard_state=failed`, `onboard_step=failed`, and
   `onboard_error_code=invalid_otp`.
3. The settings page shows a localized invalid-OTP message and tells the user
   to request a new OTP and retry.
4. The branch does not appear connected and polling stops.

### Scenario 2: Provider is unavailable

1. The user submits an OTP and onboarding enters a durable processing state.
2. The daemon records `failed` with `provider_unavailable`, bounded error
   evidence, and retry metadata.
3. The page shows the current failure and retry guidance without exposing an
   active/connected state.
4. Polling stops at the terminal failure; a later user retry may start a new
   backend attempt.

### Scenario 3: Successful onboarding

1. The page consumes queued, CSR, compliance, invoices, and production progress
   as available.
2. When the durable state becomes `done`, the page shows localized completion
   feedback and may show the connected badge.
3. The page does not show connected while the state is still pending, even if a
   stale numeric status says active.
4. Existing save-before-OTP behavior remains unchanged.

## Functional requirements

- **FR-001**: The settings handler MUST project the T010/T011 non-secret
  onboarding state, step, attempt, retry count, error code/message, and
  completion metadata to the template without exposing OTPs or credentials.
- **FR-002**: The template MUST consume `onboard_state` and `onboard_step` and
  render localized queued/in-progress stage guidance for `queued`, `csr`,
  `compliance`, `invoices`, and `production`.
- **FR-003**: The template MUST map `invalid_otp`, `provider_unavailable`, and
  other bounded onboarding errors to localized failure guidance and a retry
  instruction; bounded backend text may be used only as a fallback.
- **FR-004**: A branch MUST NOT be presented as active/connected while its
  durable onboarding state is `queued`, `csr`, `compliance`, `invoices`,
  `production`, or `failed`. Active presentation is allowed only for `done` or
  a legacy response with no onboarding state.
- **FR-005**: Polling MUST stop on durable `failed` and `done` terminal states,
  and MUST update the cached branch metadata before rendering the terminal
  feedback.
- **FR-006**: A failed attempt MUST leave the existing settings form usable and
  provide a retry path that does not resubmit the old OTP.
- **FR-007**: HTTP-level bad-OTP/provider errors and successful queued/done
  responses MUST retain existing localized toast/modal behavior where
  applicable.
- **FR-008**: T028's save promise, linked store-address ordering, pending-save
  guard, and settings layout MUST remain intact.
- **FR-009**: Focused handler/template regression tests MUST fail against the
  starting template/handler and pass after implementation.

## Non-functional requirements

- No backend or daemon files may change.
- Existing Arabic/English localization conventions and settings controls must
  be preserved.
- Polling must not create duplicate intervals or continue indefinitely after a
  terminal response.
- Error display must remain bounded and must not expose OTPs or credentials.
- Go source and tests must be formatted with `gofmt`.

## Acceptance criteria

1. Handler regression coverage proves invalid-OTP, provider-unavailable, and
   done metadata are returned by the frontend ZATCA endpoint.
2. Template regression coverage proves localized state/error/retry rendering,
   false-active protection, terminal failure polling shutdown, and retry reset.
3. The focused tests are red before the production edits and green afterward.
4. T028 save-order assertions remain green.
5. `go test ./...`, `gofmt`, and diff checks complete; unrelated failures are
   recorded rather than hidden.
6. Only frontend/template/localization/tests/spec artifacts are in one isolated
   local commit; no push or PR is created.

## Assumptions, constraints, risks, and unknowns

### Assumptions

- T010's GET response exposes `onboard_state`, `onboard_step`,
  `onboard_attempt`, `onboard_retry_count`, `onboard_error_code`,
  `onboard_error`, and completion metadata.
- T011 uses `invalid_otp`, `provider_unavailable`, and a bounded generic
  onboarding error code for terminal failures.
- A new retry is initiated by the existing OTP submission route; the frontend
  must never reuse the previous OTP.

### Constraints

- Start at frontend commit `55a5024` on
  `fix/zatca/c5-onboarding-ui`.
- Do not edit T010/T011 worktrees or backend/daemon source.
- Preserve T028 save-order behavior and settings UX.

### Risks

- Older deployments may omit the new fields; the UI must retain the existing
  numeric-status fallback without treating an explicitly pending/failed state
  as active.
- Backend error text may be localized or provider-specific; code-based labels
  must be preferred and raw bounded text used only as a fallback.
- Source-level template tests are intentionally browserless and cannot prove
  CSS rendering or real timer behavior.

### Unknowns resolved by research

- T010 defines states `not_started`, `queued`, `compliance`, `production`,
  `done`, and `failed`, with legacy `csr`/`invoices` readable.
- T010 defines steps `queued`, `csr`, `compliance`, `invoices`, `production`,
  and `done`; T011 writes `failed` as the failure step.
- T011 normalizes failures to `invalid_otp`, `provider_unavailable`, or a
  bounded generic onboarding error and makes failed attempts retry-eligible
  through a later backend request.

