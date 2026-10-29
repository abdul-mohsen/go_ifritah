# Requirements: T005 Frontend C2 Submission Outcome UI

## Problem

Invoice list and detail pages currently infer ZATCA status from the legacy
numeric invoice state. That mapping is too generic for durable submission
outcomes: rejected, ambiguous, and pending submissions can be presented as
ordinary processing/issued states, and the available retry/provider/artifact
information is not available to the page actions.

## Goal

Use the approved backend/daemon submission outcome contract to render the real
ZATCA outcome for invoice list and detail pages without changing backend or
daemon code.

## In Scope

- Read durable submission outcome data returned by the backend monitor contract.
- Associate a submission outcome with an invoice by its invoice/bill ID.
- Preserve the existing legacy invoice and credit-note status behavior when no
  durable submission outcome is available.
- Render the six durable states:
  `pending`, `accepted`, `accepted_with_warning`, `rejected`, `ambiguous`, and
  `terminal_failure`.
- Treat only `accepted` and `accepted_with_warning` as issued for display and
  issued-only actions.
- Carry provider code, warning code/message, retryability/retry metadata, and
  request/cleared artifact references into invoice page data where present.
- Add a focused handler/template regression test covering rejected, ambiguous,
  and pending outcomes.

## Out of Scope

- Any backend, daemon, database, API route, or monitor contract changes.
- Changing legacy invoice states, credit-note states, monetary values, or
  existing non-ZATCA labels.
- Implementing a new retry API. The frontend may expose an existing safe action
  only when the contract says it is retryable; it must not invent an endpoint.
- Replacing the existing ZATCA monitor mock UI (that is a later frontend lane).

## User Stories

1. As an operations user, I can distinguish accepted, accepted-with-warning,
   rejected, pending, ambiguous, and terminal-failure invoice submissions.
2. As an operations user, I never see an invoice marked issued unless its
   durable outcome is accepted or accepted-with-warning.
3. As an operations user, I can see provider warning/error details and
   available retry/artifact references on invoice details when the backend
   returns them.
4. As an existing user, invoices without durable outcome data retain the
   current localized legacy status and actions.

## Functional Requirements

### FR-001: Contract states

The frontend MUST recognize exactly the approved durable states:
`pending`, `accepted`, `accepted_with_warning`, `rejected`, `ambiguous`, and
`terminal_failure`. Unknown or absent state values MUST fall back to the
existing legacy mapping rather than being treated as issued.

### FR-002: Issued truth

The frontend MUST set its issued view/action flag only for `accepted` and
`accepted_with_warning`. Rejected, ambiguous, pending, and terminal-failure
outcomes MUST NOT use the issued label/class or issued-only actions.

### FR-003: List mapping

The invoice list handler MUST merge the backend durable submission response
with invoice rows and pass the resolved state, localized label, CSS class,
provider details, retryability, and artifact references to the list template.

### FR-004: Detail mapping

The invoice detail handler MUST resolve the durable submission for the
requested invoice and pass the same outcome data to the detail template.

### FR-005: Actions

Invoice actions MUST be derived from the resolved outcome. Draft/edit/delete
actions remain governed by the legacy draft state. Issued-only actions and
credit-note eligibility MUST not be enabled for non-success durable outcomes.
No retry link may be rendered unless the backend response explicitly marks the
submission retryable and an existing frontend-safe endpoint is available.

### FR-006: Localization and compatibility

New status labels/messages MUST use existing localization helpers/keys where
available, preserving Arabic and English behavior. Existing invoice and
credit-note statuses MUST remain unchanged when no durable outcome overrides
them.

### FR-007: Regression coverage

A focused handler/template test MUST fail against the base implementation for
rejected, ambiguous, and pending durable outcomes because the generic mapping
would otherwise render them incorrectly, then pass after the implementation.

## Non-Functional Requirements

- Changes are limited to the frontend worktree and T005 scope.
- No backend or daemon files are modified.
- All edited Go files are gofmt-formatted.
- Targeted tests and `go test ./...` pass.
- No new dependency is required.

## Acceptance Criteria

1. A red focused test demonstrates that rejected, ambiguous, and pending
   outcomes are not rendered as generic issued/processing states.
2. The green implementation maps backend state and contract fields into both
   invoice list and detail page data.
3. Only accepted and accepted-with-warning resolve to issued.
4. Rejected, ambiguous, pending, and terminal-failure outcomes retain distinct
   state labels/classes and do not expose issued-only actions.
5. Existing localized legacy and credit-note statuses remain intact when the
   durable response is absent.
6. `gofmt -l` reports no changed Go files and targeted tests plus
   `go test ./...` pass.
7. One isolated commit contains only T005/frontend changes and required
   Spec Kit artifacts; no push or PR is created.

## Assumptions, Constraints, Risks, and Unknowns

- Assumption: the approved monitor endpoint is
  `GET /api/v2/zatca/monitor/submissions`, and its rows contain
  `invoice_id`, `state`/`status`, provider code/reference, warning fields,
  retry metadata, and request/cleared XML references.
- Assumption: the frontend's configured backend domain and authenticated
  request helper can call that endpoint with the existing session token.
- Limitation: the approved T003 monitor route is admin-scoped, so invoice
  viewers without that permission may receive legacy fallback status.
- Constraint: T005 cannot change the backend bill list/detail response or
  daemon persistence contract.
- Risk: a backend deployment may expose only the newer `state` key or only the
  compatibility `status` key; the frontend must accept both without
  fabricating an outcome.
- Risk: the endpoint may be unavailable in older deployments; invoice pages
  must preserve their current behavior rather than fail closed.
- Unknown resolved by research: the backend T003 monitor contract is the
  authoritative source for durable outcome rows; bill list/detail responses
  still expose legacy numeric state only.
- Remaining uncertainty: no retry endpoint is defined by the approved T003/T004
  contract, so T005 will carry retryability metadata but will not invent a
  retry action.
