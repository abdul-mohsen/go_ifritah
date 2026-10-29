# T012 Design

## Boundary

Change only the frontend handler projection, ZATCA settings template,
localization labels, and focused frontend tests/spec artifacts. T010/T011
backend and daemon code remain untouched.

## Data flow

1. `FetchZatcaConfigForBranch` decodes the non-secret T010 metadata from the
   existing frontend proxy response.
2. `HandleSettingsPage` retains the existing numeric status map and adds a
   per-branch onboarding metadata map.
3. `settings.html` initializes the metadata map from JSON and renders state,
   step, error, retry, and completion guidance.
4. OTP submission and polling update the same metadata cache.
5. A durable `failed` or `done` response updates the UI and clears the stored
   interval before any further polling.

## Interfaces

### Frontend handler projection

Extend `ZatcaBranchStatus` and the backend response decoder with:

- `onboard_state`
- `onboard_step`
- `onboard_attempt`
- `onboard_retry_count`
- `onboard_error_code`
- `onboard_error`
- `onboard_queued_at`
- `onboard_started_at`
- `onboard_completed_at`
- compatibility `last_error` and `last_attempt_at`

Add a JSON-safe `ZatcaOnboardingState` view map to the settings template data.
Do not alter any backend route or payload.

### Template state model

The template keeps:

- `zatcaStatusByBranch` for legacy numeric status;
- `zatcaOnboardingByBranch` for durable metadata;
- `zatcaOnboardPolls` for one interval per branch.

State precedence:

1. Explicit `failed`: show failure, force non-active display, stop polling.
2. Explicit pending/worker state (`queued`, `csr`, `compliance`, `invoices`,
   `production`): show localized progress, force non-active display.
3. Explicit `done`: show completion and allow active status only when the
   numeric status is active.
4. Empty/legacy state: preserve the existing numeric-status behavior.

### Localized UX

Add labels for queued progress, provider/OTP/generic failures, retry action and
guidance, and retry count. Existing step labels and T028 save labels remain
the source for shared copy.

## Failure handling

- HTTP OTP validation/provider errors use the durable error code when present;
  known codes map to localized copy, and bounded detail is a fallback.
- Polling failure responses do not change the cached state; the interval ends
  only at terminal state or the existing maximum-attempt timeout.
- A terminal failed attempt shows retry guidance and a form reset action; the
  old OTP is cleared and never resubmitted.
- A terminal done attempt shows localized completion and clears polling.

## Test strategy

- Add a handler test table with durable invalid-OTP, provider-unavailable, and
  done responses; assert the frontend endpoint preserves all relevant metadata.
- Add a template-source test that asserts metadata projection, localized
  mappings, false-active guards, retry reset, and terminal polling cleanup.
- Run those tests red before production edits.
- Keep `TestT028ZatcaOnboardingWaitsForCompleteSave` green.
- Run `gofmt`, targeted frontend packages, and `go test ./...`.

## Compatibility, security, and performance

- No OTP or credentials are selected, stored, or rendered.
- Existing numeric status and missing-field UX remains intact for older
  backends.
- Polling is bounded and de-duplicated per branch.
- No new dependency or browser runtime is required.

## Requirement traceability

| Requirement | Design response |
|---|---|
| FR-001 | Extended frontend decoder plus `ZatcaOnboardingState` map |
| FR-002 | State/step maps and localized progress feedback |
| FR-003 | Error-code localization with bounded fallback |
| FR-004 | State precedence in status/readiness helpers |
| FR-005 | Stored interval and terminal cleanup |
| FR-006 | Form reset/retry action clears old OTP |
| FR-007 | Existing toast/modal paths retained and enhanced |
| FR-008 | T028 save functions and readiness guard preserved |
| FR-009 | Handler table test and template contract test |

