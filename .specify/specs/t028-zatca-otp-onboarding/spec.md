# T028: Await ZATCA Prerequisite Saves Before OTP Onboarding

## Overview

Prevent the ZATCA OTP onboarding action from running while branch
configuration or the linked store address is still being saved, and make any
save failure visible to the user.

## User value

Users must not consume a ZATCA OTP for a branch whose configuration is only
partially saved. They should see a clear failure and be able to retry without
guessing whether the address was persisted.

## In scope

- ZATCA settings-page save ordering and pending-state behavior.
- Existing localized save status/toast feedback.
- A browserless Go/template regression test.

## Out of scope

- Backend, database, or daemon changes.
- Other settings flows.
- Browser automation and new dependencies.

## User scenarios

1. A successful configuration save waits for the linked store-address save,
   then enables Connect and preserves existing success feedback.
2. A failed configuration or store-address save shows the existing localized
   failure feedback and keeps onboarding unavailable.
3. A dirty configuration invoked through Connect is saved first; the OTP
   dialog opens only after the complete save succeeds.

## Functional requirements

- The complete save operation includes branch configuration and linked
  store-address persistence.
- OTP onboarding cannot start while either prerequisite save is pending.
- A failed or rejected prerequisite keeps the saved state false and the
  onboarding control disabled.
- Existing localized status and toast feedback is used for failures.
- Existing successful UX, field validation, branch switching, and polling are
  preserved.
- A Go/template/browserless regression test fails against the baseline
  fire-and-forget behavior and passes after the fix.

## Non-functional requirements

- Frontend-only implementation.
- No new runtime or test dependency.
- Preserve Arabic/English localization and existing layout.
- Avoid duplicate requests and uncaught promise failures.

## Acceptance criteria

- Regression test is demonstrated red before the production edit and green
  afterward.
- Both saves are ordered before OTP onboarding.
- Save failures are visible and pending controls remain disabled.
- `gofmt`, targeted tests, and `go test ./...` are attempted as practical.
- The final local commit is isolated, not pushed, and contains no backend or
  daemon changes.

## Assumptions and risks

- Existing endpoint HTTP success status is the save success signal.
- Existing localized keys are sufficient.
- Full-suite failures, if any, may be environment-dependent and must be
  recorded rather than hidden.
