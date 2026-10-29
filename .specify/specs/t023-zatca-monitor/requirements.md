# T023 ZATCA Monitor Frontend Requirements

## Problem

The ZATCA monitor page currently renders hard-coded values and can imply successful
submission activity even when the authenticated backend has no data or is unavailable.
The frontend must consume the read-only monitor contract and present the backend's
actual tenant-scoped results without changing backend, daemon, or deployment code.

## Goals

- Replace all monitor mock values with authenticated calls to the documented monitor
  endpoints.
- Model and render real aggregate statistics, branch rows, and recent submission rows.
- Preserve the existing Arabic/English localization keys and page navigation.
- Make loading, backend error, and empty-data states explicit.
- Render explicit submission/branch statuses, warning details, pending age, and provider
  references without inventing success.
- Add a focused handler/template regression test proving that mocked values are absent and
  backend values are rendered.

## Scope

### In scope

- `handlers/zatca_monitor.go` and focused handler tests.
- `templates/zatca-monitor.html` and directly related localization additions only when
  existing keys cannot express a required state.
- Authenticated proxying to:
  - `GET /api/v2/zatca/monitor/stats`
  - `GET /api/v2/zatca/monitor/branches`
  - `GET /api/v2/zatca/monitor/submissions?limit=50`
- Frontend mapping of the documented response fields, including nullable provider and
  timestamp fields.
- Frontend-focused documentation/artifacts required by the repository workflow.

### Out of scope

- Any backend, daemon, database, schema, API-contract, or deployment changes.
- Retrying or resubmitting invoices from the monitor.
- Creating false fallback values when an upstream response is missing or fails.
- Replacing the page with a client-side mock or a new unauthenticated API.

## User stories

1. As an authenticated administrator, I can see the real ZATCA submission totals for my
   tenant instead of demo numbers.
2. As an authenticated administrator, I can identify each branch's connection state,
   certificate expiry, daily volume, success rate, and last submission time.
3. As an authenticated administrator, I can inspect recent rows with explicit state,
   warning/provider details, pending age, and provider references.
4. As an authenticated administrator, I can distinguish loading, empty, and backend error
   states from a successful page with zero values.
5. As a bilingual user, I continue to see the existing localized labels in Arabic or
   English.

## Functional requirements

- **FR-01:** The handler must require the existing authenticated session and use the session
  token when calling all three monitor endpoints.
- **FR-02:** The handler must decode the documented stats response, including
  `total_submitted`, `accepted`, `accepted_with_warning`/`warnings`, `rejected`, and
  `pending` (and preserve additional contract fields for display or safe future use).
- **FR-03:** The handler must decode branch rows with `branch_id`, `branch_name`,
  `zatca_status`, nullable `cert_expiry`, `today_count`, `success_rate`, and nullable
  `last_submission_at`.
- **FR-04:** The handler must decode submission rows with invoice identity, branch, explicit
  state/status, warning fields, provider code/reference, ZATCA reference, and submitted or
  last-attempt timestamps.
- **FR-05:** The template must render a branch status derived from backend status values,
  not hard-coded branch names or statuses.
- **FR-06:** The template must render each submission state distinctly, including accepted,
  accepted-with-warning/warning, rejected, pending, ambiguous, and terminal-failure
  states when supplied by the backend; unknown states must not be shown as accepted.
- **FR-07:** Warning/provider information and provider references must be rendered when
  available and represented as unavailable when nullable/missing.
- **FR-08:** Pending rows must expose their age using the backend timestamp and a localized
  label; no pending row may falsely display an accepted result.
- **FR-09:** Empty branch and submission collections must render localized empty-state
  feedback rather than blank tables or mock rows.
- **FR-10:** Any upstream failure or malformed response must render a localized visible
  error state and must not render mock values or false success.
- **FR-11:** Refresh must re-request current backend data rather than restore hard-coded
  values.
- **FR-12:** A focused regression test must fail against the current mock implementation,
  then pass after proxy wiring by asserting request paths/authentication and backend-specific
  values in rendered HTML.

## Non-functional requirements

- Preserve the existing handler routing and authentication behavior.
- Keep tenant isolation delegated to the authenticated backend contract.
- Avoid leaking access tokens or sensitive backend response details into HTML/logs.
- Keep the page server-rendered and compatible with the existing template cache and
  localization function.
- Keep changes isolated to the frontend worktree and task T023.

## Acceptance criteria

- No hard-coded demo stats, branches, or submissions remain in the monitor handler/template.
- All three documented endpoints are called with authenticated requests.
- Backend-provided branch and submission values, including nullable provider fields, render
  correctly.
- Loading/error/empty states are visible and localized; a failed request cannot look like
  success.
- The focused regression test is red before implementation and green afterward.
- `gofmt` leaves changed Go files unchanged, targeted tests pass, and `go test ./...` passes.
- The final commit contains only PR-ready frontend changes; no push or PR is created.

## Assumptions

- The backend contract documented by the sibling backend worktree is the contract available
  to this frontend: the routes are mounted under `/api/v2/zatca/monitor/*`.
- Branch `zatca_status` retains the established numeric meanings: active, expired,
  not-active, and expiring-soon values used by the existing ZATCA UI.
- The backend returns JSON arrays for branches and submissions and a JSON object for stats.
- The existing frontend's `helpers.DoAuthedRequestWithRetry` is the required authenticated
  proxy path.

## Constraints

- Do not edit backend/daemon/deployment files or sibling worktrees.
- Do not change existing localization semantics or expose a retry/resubmit action.
- Do not create mock or fallback success data.

## Risks and unknowns

- The exact deployed backend version may include either `warnings` or the newer
  `accepted_with_warning` stats field; the frontend must accept both without double-counting.
- Nullable timestamps may be RFC3339 strings or empty/null; formatting must fail safe.
- Existing localization lacks some state/error/empty/pending-age labels; additions may be
  required while retaining the current keys.
- No browser-level loading transition is observable in a single server render; the server
  response must make backend request failure and empty results explicit.

## Open questions

- None blocking after inspecting the sibling backend contract and handler implementation.
