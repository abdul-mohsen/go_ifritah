# T032 Import Failure State Requirements

## Problem

The signed-UBL import page can receive a T031-valid import whose lifecycle
status is `posted` while its durable evidence still records a failure. The
frontend currently displays that item as successfully posted, contradicting
the evidence shown to the reviewer.

## Goals

- Prevent failure evidence from being presented as a successful `posted` or
  `approved` display state.
- Preserve the existing display of a successful `posted` import with no
  failure evidence.
- Preserve existing localized status labels and all other import lifecycle
  behavior.

## In scope

- The frontend import-view status derivation.
- A focused browserless regression test for posted imports with and without
  failure evidence.
- Spec Kit artifacts and validation of the affected Go package.

## Out of scope

- T031 backend persistence or lifecycle transitions.
- Changes to evidence fields, failure detection semantics, actions, routes,
  templates, or localization resources unless required to preserve the
  existing contract.
- Browser automation, live storage, or deployment changes.

## User scenarios

### US1 — Failed posted import is visibly failed

As an import reviewer, when a posted import carries failure evidence, I see
the import as failed rather than as a successful posted import.

### US2 — Successful posted import remains posted

As an import reviewer, when a posted import has no failure evidence, I see the
existing posted state and its existing localized label.

### US3 — Existing failure behavior remains stable

As an import reviewer, review or approved imports with existing failure
evidence continue to use the failed display state and existing action gating.

## Functional requirements

- **FR-001**: An import with failure evidence MUST NOT render with a
  successful `posted` or `approved` display state.
- **FR-002**: A `posted` import with no failure evidence MUST retain the
  existing `posted` display state.
- **FR-003**: The raw lifecycle status and existing action permissions MUST
  remain unchanged by display-state derivation.
- **FR-004**: Existing localized status labels and template contracts MUST
  remain unchanged.
- **FR-005**: A focused regression test MUST fail against commit `afc6b71`
  and pass after the defect is fixed.

## Non-functional requirements

- Keep the change limited to the smallest explicit frontend rule that
  distinguishes failure evidence from successful posted display state.
- Do not add a new localization key or expose backend evidence differently.
- Preserve Go formatting and repository test compatibility.

## Acceptance criteria

- [x] A posted import carrying the existing failure evidence recognized by the
  view is rendered with display state `failed`.
- [x] A posted import without failure evidence remains display state `posted`.
- [x] Existing review/approved failure behavior and action gating remain
  unchanged.
- [x] Existing localization is unchanged.
- [x] The focused regression is red on `afc6b71` before the implementation and
  green afterward.
- [x] Required formatting, targeted tests, full tests, diff check, todo
  update, self-review, and one local commit complete without a blocker.

## Assumptions, constraints, risks, and unknowns

### Assumptions

- The existing `hasFailure` calculation is the T032 contract for failure
  evidence: a non-empty failure class or an error-severity validation issue.
- T031 may legitimately return `posted` together with failure evidence, so the
  frontend must treat display status as evidence-aware.

### Constraints

- Work only in
  `C:\ssda\chatGPT\worktrees\zatca-fix-t032-failure-state-frontend`.
- Do not modify the existing T032 worktree, push, or create a pull request.
- Base the change on commit `afc6b71`.

### Risks

- A broad failure override could change rejected or unknown status rendering;
  the rule must be limited to successful posted/approved states plus the
  existing review behavior.
- Browserless tests cannot validate live backend state or browser rendering.

### Unknowns

- None blocking. The existing T032 failure-evidence definition and status
  vocabulary provide the required contract.
