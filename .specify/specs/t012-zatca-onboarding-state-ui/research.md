# T012 Research

## Evidence inspected

| Area | Evidence | Finding and decision |
|---|---|---|
| Starting frontend | `55a5024`, `templates/settings.html`, `config/settings_zatca_flow_test.go` | T028 already awaits branch/store saves and guards pending saves. Keep that behavior unchanged. |
| Frontend handler | `handlers/zatca.go` and `handlers/settings.go` | The frontend already fetches `onboard_state`/`last_error` partially, but drops durable step, retry, error-code, and completion fields before rendering settings. Extend only the frontend projection. |
| Backend contract | T010 `README.md`, `requirements.md`, `design.md`, `pkg/handlers/zatca_onboarding_state_test.go` | GET exposes state, step, attempt, retry count, error code/message, and queued/started/completed metadata. New attempts start as `queued`; done is the completion state. |
| Daemon contract | T011 `README.md`, `requirements.md`, `design.md`, `data-model.md`, `cmd/daemon/onboarding_state.go` | Progress steps are queued/CSR/compliance/invoices/production/done; terminal failures use bounded codes including `invalid_otp` and `provider_unavailable`; failed state is retry-eligible. |
| Existing UI | `templates/settings.html` | Current polling stops for any non-3 numeric status, which can falsely stop/show active before durable `done`; failure only shows raw `last_error` and has no retry guidance. |
| Existing localization | `resources/labels.go` | Existing progress labels cover CSR/compliance/invoices/done, but queued, error-code, and retry guidance labels are missing. Add only directly related labels. |
| Existing tests | `handlers/zatca_onboard_async_test.go`, `config/settings_zatca_flow_test.go` | Handler tests already protect no-false-active async queue behavior; add durable metadata cases. Preserve T028 source assertions and add T012 source assertions. |

## Contract decisions

1. Keep `ZatcaStatusByBranch` as the numeric compatibility map so T028 and
   existing settings rendering remain stable. Add a separate JSON-safe
   onboarding metadata map for the template.
2. Prefer the durable state/step over numeric status whenever state is
   explicitly pending or failed. Only `done` may confirm active completion.
3. Stop polling only on durable `failed`/`done` (with a legacy no-state + active
   fallback for deployments that predate T010), not merely on a non-3 status.
4. Map error codes to localized copy. Show bounded backend error text only when
   no known code is available.
5. Retry guidance resets the OTP form and never resends the previous code.

## Alternatives considered

- **Change the backend response**: rejected; T010 already defines the contract
  and the user explicitly forbids backend/daemon edits.
- **Replace the numeric status map with a new composite type**: rejected;
  retaining the existing map minimizes T028/settings regressions.
- **Poll indefinitely on all non-active statuses**: rejected; terminal failure
  must stop polling and active status cannot be inferred before `done`.
- **Show provider error strings as the primary copy**: rejected; code-based
  localization avoids leaking provider wording and preserves Arabic/English UX.

## Remaining limitations

- The regression tests are browserless source/handler contract tests; they do
  not execute a real browser DOM or timer.
- Older backend deployments that omit durable state retain the legacy numeric
  status fallback and cannot provide stage/error/retry detail.

