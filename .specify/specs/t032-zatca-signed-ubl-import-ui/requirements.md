# T032 — Authenticated signed-UBL import UI

## Problem

The backend T031 commit persists signed-UBL import evidence and a separate
review lifecycle, but the frontend has no authenticated route or view for
uploading, inspecting, reviewing, or transitioning those imports. Reviewers
therefore cannot see provenance and validation evidence or make an explicit
post/reject decision.

## Goals

- Add an authenticated signed-UBL upload/import page and detail/review page.
- Show durable lifecycle status, provenance, validation errors, review
  decisions, and append-only status history.
- Expose explicit pending, failed, approved, posted, and rejected states.
- Allow review approval/request-changes, import-lane posting, and rejection
  through authenticated frontend proxy routes.
- Never display an import as an outbound ZATCA `issued` invoice.
- Preserve existing settings/onboarding behavior and Arabic localization.

## In scope

- Frontend handlers, authenticated proxy routes, templates, labels, and
  focused browserless regression tests.
- A bounded raw signed-UBL upload adapter that sends T031-compatible JSON
  evidence to the backend; parsing and cryptographic verification remain
  outside this frontend.
- List/filter and detail/review views for the T031 import lifecycle.

## Out of scope

- Backend, signed-UBL parser/service, daemon, NATS/outbox, database, or
  deployment changes.
- Automatic invoice creation, accounting, stock movement, or outbound ZATCA
  submission.
- Browser automation or live object-storage/MySQL validation.
- Changes to settings or onboarding state behavior.

## User stories

### US1 — Upload and retain a signed-UBL source

As an authenticated user, I can select a signed UBL XML file, submit it to
the authenticated import lane, and land on the durable import detail page.
The UI clearly says the item is pending review until an explicit decision is
made.

### US2 — Review durable evidence

As an authenticated reviewer, I can inspect the source hash/reference,
filename, MIME type, provenance, signature/validation evidence, canonical
data, validation errors, review decisions, and status history.

### US3 — Make explicit lifecycle decisions

As an authenticated reviewer, I can approve/request changes, post an approved
import in the import lane, or reject it with a required reason. The UI only
offers actions valid for the current state and never claims ZATCA issuance.

### US4 — See failures without false success

As a reviewer, I can distinguish pending review, failed validation, approved,
posted, and rejected states. A failure class or error-severity validation
issue must not be rendered as approved, posted, or issued.

## Functional requirements

- **FR-001**: Authenticated page routes MUST exist for import list/upload and
  import detail/review; unauthenticated requests MUST not render import data.
- **FR-002**: The upload form MUST submit a bounded signed-UBL source through
  an authenticated frontend proxy to T031 `POST /api/v2/zatca/import`.
- **FR-003**: The frontend MUST proxy tenant-scoped list/detail and review,
  post, and reject actions to the corresponding T031 endpoints.
- **FR-004**: Detail UI MUST expose source provenance, content hash/size,
  canonical/parsed data, signature/validation evidence, failure class/message,
  validation errors, review decisions, and status history when supplied.
- **FR-005**: Lifecycle status MUST be rendered from the import vocabulary
  (`review`, `approved`, `posted`, `rejected`) plus a derived `failed` view
  state for failure evidence; `review` MUST be shown as pending.
- **FR-006**: Post MUST be available only for approved imports, reject only
  for review/approved imports, and request-changes/review MUST not imply
  approval.
- **FR-007**: Rejection MUST require a non-empty reason in the browser UI;
  backend validation remains authoritative.
- **FR-008**: No import status, action, label, or fallback MUST use or claim
  outbound invoice `issued` semantics.
- **FR-009**: Existing settings/onboarding routes, templates, and labels MUST
  remain behaviorally unchanged.
- **FR-010**: Focused handler/template regression tests MUST be red before the
  implementation because the import handler/view/route is absent and green
  afterward without a browser or live backend.

## Non-functional requirements

- Use existing auth/session, proxy, template, RBAC, and localization patterns.
- Limit frontend upload requests to the backend-compatible 5 MiB source / 8 MiB
  request envelope and reject oversized files before sending.
- Escape backend evidence through Go templates; do not inject raw evidence into
  executable JavaScript.
- Keep all changes frontend-only and isolated to this feature.

## Acceptance criteria

- [x] Red focused handler/template tests fail against the clean T032 base.
- [x] Authenticated list/upload and detail/review pages render.
- [x] Authenticated API proxy routes cover create/list/detail/review/post/reject.
- [x] Pending, failed, approved, posted, and rejected states are explicit.
- [x] Provenance, validation errors, signature/validation evidence, review
      decisions, and status history are visible.
- [x] Post/reject/review actions send workflow version and preserve backend
      status semantics; no false issued state is present.
- [x] Settings/onboarding behavior and localization remain intact.
- [x] `gofmt`, targeted tests, `go test ./...`, and `git diff --check` run;
      browser/live-storage limitations are reported.
- [ ] One isolated local commit is created; no push or PR is made.

## Assumptions, constraints, risks, and unknowns

### Assumptions

- T031 routes are mounted under `/api/v2/zatca/import` and accept
  `xml_content` plus T030-shaped source/signature/validation metadata.
- A raw frontend upload is stored as review-required evidence; the separate
  T030 parser remains responsible for actual parsing and cryptographic
  verification.
- Backend JSON timestamps are RFC3339-compatible strings.

### Constraints

- Work only in `C:\ssda\chatGPT\worktrees\zatca-fix-t032-frontend` on
  `fix/zatca/m4-import-ui`, starting at `d7b5034`.
- Do not edit backend worktree `zatca-fix-t031-backend`, daemon, or deployment.
- Align request/response field names with backend commit `94aa8e2`.

### Risks

- Browserless tests cannot prove browser file APIs, CSRF middleware behavior,
  or live storage durability.
- A raw upload does not run T030 parsing in this repository; the UI must label
  the item review-required and avoid trust/issued claims.
- Template source assertions can become brittle; keep them focused on stable
  route/action/status contracts.

### Unknowns

- Production may later provide a T030 HTTP adapter; this bounded UI currently
  uses the T031-compatible raw-source adapter without inventing that service.
