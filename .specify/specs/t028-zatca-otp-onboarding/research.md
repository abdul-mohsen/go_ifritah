# T028 Research

## Repository baseline

- Worktree: `worktrees/zatca-fix-t028-frontend`
- Branch: `fix/zatca/m2-await-config-before-onboard`
- Starting commit: `18f7988`
- Baseline status was clean.
- There is no repository README and no existing `.specify` tree at the
  baseline; this feature directory is the scoped Spec Kit artifact location.

## Existing implementation evidence

### `templates/settings.html`

- The ZATCA save button invokes `saveZatcaConfig()`.
- `saveZatcaConfig` sends `PUT /api/zatca/branch/:id`, marks
  `zatcaConfigSaved = true`, updates the success status, calls
  `checkZatcaConnectReady`, and only then calls `saveBranchStoreCity(...)`
  without returning or awaiting its promise.
- `saveBranchStoreCity` already returns a promise and converts non-OK/rejected
  requests to `false`.
- `openZatcaOtpModal` checks company name and city, then calls
  `saveZatcaConfig()` without awaiting it when the config is dirty, and opens
  the modal immediately afterward.
- The Connect button is enabled by `checkZatcaConnectReady`, which currently
  considers only field validity and `zatcaConfigSaved`.
- Existing save error handling uses `zatca-save-status`, `showToast`, and the
  localized `tpl.settings.zatca_save_failed` key.
- The OTP submit button is already disabled during the onboarding request; the
  missing guard is the prerequisite-save phase.

### `handlers/branch_store_address.go`

- The frontend proxy exposes GET and PUT branch store-address routes.
- The PUT path validates the city and returns an error response when the
  linked-store update/create fails.
- This confirms that the frontend can reliably use the request's HTTP success
  status as the prerequisite result.

### Existing tests and conventions

- `config/template_render_test.go` validates that all templates parse.
- Handler tests use Go's `httptest` package and table/assertion conventions.
- `handlers/zatca_onboard_async_test.go` covers backend async onboarding
  semantics, but does not exercise the settings template's save ordering.
- No browserless test currently protects the template's save-before-onboard
  ordering.

## Decisions

### Decision 1: Make the complete save await both requests

**Rationale:** The branch configuration and linked store address are one user
visible prerequisite for onboarding. Treating the first request as success
would continue to expose the race.

**Alternatives considered:**

- Keep the two requests independent and poll the store save later: rejected
  because onboarding could still race and failures would remain ambiguous.
- Move the store save into the backend: out of scope and violates the
  frontend-only task.

### Decision 2: Reuse existing localized failure feedback

**Rationale:** The template already has the correct status element, toast
  mechanism, and localized save-failure key. Reusing them preserves UX and
  avoids catalog changes.

### Decision 3: Add a source-level browserless regression test

**Rationale:** The bug is in inline template JavaScript and the requested test
  must not require a browser or live backend. A focused Go test can verify the
  stable contract that the save function awaits the address save and that the
  OTP modal waits for the save result before opening.

**Alternatives considered:**

- Add Playwright coverage: useful for runtime validation but not browserless
  and slower/environment-dependent.
- Test only the backend handlers: cannot detect the frontend fire-and-forget
  race.

## Remaining uncertainty

- Browser runtime availability is not assumed; targeted Go tests are the
  authoritative validation available in this environment.
