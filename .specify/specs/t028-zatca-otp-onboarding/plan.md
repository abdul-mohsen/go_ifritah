# T028 Implementation Plan

## Technical context

- Go 1.21 frontend serving Go HTML templates with inline JavaScript.
- The relevant behavior is entirely in `templates/settings.html`.
- Existing Go tests can read and inspect templates without a browser.
- No unresolved design questions remain after repository research.

## Constitution check

No repository constitution was present at the clean baseline. The plan follows
the applicable task constraints: frontend-only scope, no new dependencies,
localized UX preservation, focused regression coverage, and isolated commit.

## Implementation phases

### Phase 0: Regression first

1. Add the focused browserless template test.
2. Run the focused test and record the expected red result.

### Phase 1: Production behavior

1. Add pending-save state to the ZATCA template flow.
2. Make `saveZatcaConfig` return a promise that completes only after the
   branch config and linked store address both succeed.
3. Reuse existing localized status/toast failure handling.
4. Await the complete save in `openZatcaOtpModal` before opening the modal.
5. Keep readiness/submit controls disabled while pending or failed.

### Phase 2: Validation and convergence

1. Run gofmt and focused/package tests.
2. Run `go test ./...` as practical and record any pre-existing/environment
   limitations.
3. Review the diff for scope, localization, and backend/daemon isolation.
4. Run convergence/self-review checks.
5. Commit only the T028 changes with the required co-author trailer.

## Rollback

Revert the isolated commit. No data migration or backend rollback is required.
