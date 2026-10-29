# T028 Requirements: Await ZATCA Configuration Before OTP Onboarding

## Problem

The ZATCA settings page saves branch configuration and the linked store address
through separate asynchronous operations. The page currently starts the OTP
onboarding flow without waiting for the linked store-address save to finish,
and it can report success before that save has succeeded. A failed save is
therefore easy to miss and onboarding can run with stale or incomplete address
data.

## Goals

- Ensure OTP onboarding cannot begin until the branch configuration and linked
  store-address saves have both completed successfully.
- Make either save failure visible using the page's existing localized status
  and toast UX.
- Keep the relevant save/connect/OTP submit controls disabled while their
  prerequisite operation is pending.
- Preserve the current localization, page layout, and successful-user flow.

## Scope

### In scope

- The ZATCA settings page behavior in `templates/settings.html`.
- A focused Go regression test that validates the template's browserless
  save-before-onboard contract.
- Formatting and tests required for these frontend changes.

### Out of scope

- Backend API or database behavior.
- The ZATCA integration daemon or worker.
- Changes to localization catalogs or unrelated settings flows.
- Browser automation or new runtime dependencies.

## User scenarios

### Scenario 1: Successful save then onboarding

1. An authorized user edits a branch's ZATCA configuration.
2. The user saves the configuration while a linked store address exists.
3. The page keeps the save/connect controls unavailable while both saves are
   pending.
4. After the branch configuration and linked store-address requests both
   succeed, the page shows the existing success feedback and enables Connect.
5. The user opens the OTP dialog and submits the OTP; onboarding starts only
   after the two saves have completed.

### Scenario 2: Linked store-address save fails

1. The branch configuration request succeeds.
2. The linked store-address request fails or returns a non-success response.
3. The page reports the failure using the existing localized status/toast
   treatment.
4. The saved state remains false and the Connect/OTP submit control remains
   disabled until a later complete save succeeds.

### Scenario 3: Auto-save before opening OTP

1. The user invokes the Connect flow while the ZATCA configuration is dirty or
   not yet confirmed saved.
2. The page waits for the complete save operation.
3. The OTP dialog opens only when the complete save succeeds; on failure it
   stays closed and the failure is visible.

## Functional requirements

- **FR-001**: The complete ZATCA save operation MUST represent both the branch
  configuration save and the linked store-address save.
- **FR-002**: The page MUST wait for the linked store-address save to settle
  before treating the ZATCA configuration as saved or enabling Connect.
- **FR-003**: The page MUST NOT initiate the OTP onboarding request while the
  complete save operation is pending.
- **FR-004**: A non-success response or rejected request from either save MUST
  leave the saved state false and prevent OTP onboarding.
- **FR-005**: Save failures MUST be shown through the existing localized status
  element and error toast behavior; no new untranslated user-facing copy is
  required.
- **FR-006**: Save/connect/OTP controls involved in the prerequisite flow MUST
  remain disabled while the corresponding save or onboarding request is
  pending.
- **FR-007**: A successful save MUST preserve existing success feedback and
  enablement behavior once all required fields and validation rules pass.
- **FR-008**: Existing branch switching, address loading, status polling, and
  localization behavior MUST remain unchanged except for the required ordering
  and failure handling.
- **FR-009**: The regression test MUST fail against the current
  fire-and-forget implementation and pass after the production change without
  requiring a browser or live backend.

## Non-functional requirements

- No backend, daemon, database, API contract, or dependency changes.
- Keep the implementation localized to the frontend template and focused test.
- Preserve the current Arabic/English localization keys and visual UX.
- Do not introduce unbounded timers, duplicate save requests, or uncaught
  promise rejections.

## Acceptance criteria

- [x] A focused browserless Go/template test is added and is red before the
      production edit.
- [x] The test is green after the production edit.
- [x] OTP onboarding is ordered after both prerequisite saves.
- [x] Save failures are visible and leave the onboarding control disabled.
- [x] Pending prerequisite saves keep the relevant submit/connect control
      disabled.
- [x] `gofmt` and targeted tests pass; `go test ./...` is run when practical.
- [x] No backend/daemon files or other worktrees are modified.
- [x] The final diff is isolated in one PR-ready commit and is not pushed.

## Assumptions, constraints, risks, and unknowns

### Assumptions

- Existing save endpoints use HTTP success status as their success signal.
- Existing localized keys such as `tpl.settings.zatca_save_failed`,
  `tpl.settings.zatca_config_saved`, and `ui.saving` are sufficient.
- The linked store-address request remains a required prerequisite because the
  daemon reads the store city and address fallback data.

### Constraints

- Work only in `C:\ssda\chatGPT\worktrees\zatca-fix-t028-frontend`.
- Start from commit `18f7988` on
  `fix/zatca/m2-await-config-before-onboard`.
- Do not implement backend or daemon changes.

### Risks

- A template-source regression test can become brittle if unrelated formatting
  changes move JavaScript markers; keep assertions focused on behavior and
  stable function boundaries.
- Full repository tests may include environment-dependent tests or pre-existing
  failures unrelated to this change.

### Resolved unknowns

- The existing `saveZatcaConfig` callback currently marks the config saved and
  enables Connect before calling `saveBranchStoreCity` without awaiting it.
- `openZatcaOtpModal` currently calls `saveZatcaConfig()` without awaiting it
  before opening the OTP modal.
- Existing UI status/toast localization is available, so no localization
  catalog change is necessary.
