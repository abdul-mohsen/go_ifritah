# T028 Design

## Boundary

Only the inline ZATCA workflow in `templates/settings.html` and a focused
browserless Go test change. No backend or daemon code changes.

## State and data flow

1. A branch is selected and its existing config/address is loaded.
2. `saveZatcaConfig` starts a single prerequisite operation:
   - PUT the branch ZATCA config.
   - If that succeeds, PUT the linked store address and wait for its result.
3. Only after both requests succeed:
   - mark `zatcaConfigSaved` true,
   - update the cached branch data,
   - show the existing success status/toast,
   - re-run readiness checks.
4. If either request fails:
   - mark the saved state false,
   - show the existing localized failure status/toast,
   - keep Connect disabled.
5. `openZatcaOtpModal` awaits a dirty-config save result. It opens the modal
   only for a successful result and returns without opening it on failure.
6. `checkZatcaConnectReady` includes the pending-save state in its enablement
   condition, preventing a stale enabled control during an in-flight save.

## Interfaces

- Existing `PUT /api/zatca/branch/:id` remains unchanged.
- Existing `PUT /api/branch/:id/store-address` remains unchanged.
- Existing `POST /api/zatca/branch/:id/onboard` remains unchanged.
- The JavaScript function `saveZatcaConfig` gains a promise result representing
  complete-save success; existing button-click callers remain compatible.

## Failure handling

- Non-OK responses and rejected requests are failures.
- Existing localized error text is rendered in `zatca-save-status` and passed to
  the existing error toast.
- A failed prerequisite never opens the OTP modal and never sends onboarding.
- Pending state is cleared in success and failure paths so the user can retry.

## Test strategy

- Add a Go test under `config/` that reads `templates/settings.html` and
  verifies:
  - the save function returns/awaits the linked store-address save before
    marking the config successful;
  - the OTP modal flow awaits the save result and places modal opening after
    the awaited result;
  - the readiness guard includes pending-save state.
- Run that test before the production edit to prove it is red, then after the
  edit to prove it is green.
- Run `gofmt`, the focused test, the relevant package tests, and `go test ./...`
  as practical.

## Compatibility and UX

- Preserve existing localization calls, labels, layout, and successful flow.
- Do not add a backend dependency or alter response payloads.
- Browser automation is not required for this regression; report any runtime
  limitation separately.

## Requirement traceability

| Requirement | Design response |
|---|---|
| FR-001–FR-003 | Promise-based complete save and awaited modal flow |
| FR-004–FR-006 | Shared pending/saved state, failure feedback, readiness guard |
| FR-007–FR-008 | Existing success paths and localized UI retained |
| FR-009 | Browserless Go/template contract test |
