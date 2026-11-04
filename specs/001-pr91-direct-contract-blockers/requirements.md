# PR #91 Direct Contract Blockers — Requirements

## Goal

Resolve the six direct PR #91 blockers so sale, quick-draft, credit-note, purchase-import, effective-date, and sale-identity workflows preserve user intent and match the verified backend contracts.

## Scope

In scope:
- Verify sale-line discount transmission and persistence; apply the minimal backend write-path fix if the required line value is dropped.
- Ensure quick drafts use valid selected branch and payment context.
- Create full-remaining credit notes only, deriving line quantities and amount from a verified backend snapshot and requiring an explicit reason.
- Submit purchase imports through a real request path with supplier sequence, cost price, payment method, and backend-compatible dates.
- Preserve Riyadh local effective-date semantics.
- Renew request identity for a new sale while retaining it for reload, back/resume, and retry of the same form entry.
- Add focused tests against actual request construction/submission and update relevant user-facing documentation.

Out of scope:
- Changes to accounting formulas or the backend credit-note POST validation/semantics.
- Partial-credit-note UI, guessed line identifiers, fabricated amounts, unrelated PR comments, or unrelated cleanup.

## Requirements

| ID | Requirement | Priority |
|---|---|---|
| FR-001 | Each applicable sale-line discount must be transmitted and persisted per backend #105 without changing its accounting formula. | High |
| FR-002 | Quick drafts must use valid selected branch/payment values and block submission if required context is missing. | High |
| FR-003 | Credit-note requests must include all #108-required values and the full remaining source-bill lines/amount; the workflow must block when that remainder cannot be proven. | High |
| FR-004 | Purchase imports must use a working submit path and carry supplier sequence, cost price, payment method, and backend-compatible date values. | High |
| FR-005 | Effective dates must retain the intended Riyadh business date through frontend serialization and backend normalization. | High |
| FR-006 | A new sale form entry must not reuse an abandoned sale's request ID; reload/back/resume/retry of the same entry must retain it. | Medium |
| NFR-001 | Tests must validate serialized request data or submission behavior, not only a mock expectation. | High |
| NFR-002 | Backend accounting formulas and the existing credit-note POST contract must remain unchanged; only the required discount persistence and an additive read-only credit-source snapshot are in scope. | High |

## Acceptance Criteria

- **ACC-001**: Sale discount behavior is verified end to end; regression coverage proves non-zero line discounts are passed to persistence and totals use the existing backend formula.
- **ACC-002**: Draft requests use valid context; missing context produces no request and actionable feedback.
- **ACC-003**: Credit-note requests contain required #108 fields and a backend-verified full-remaining snapshot; missing/unverifiable source data blocks submission.
- **ACC-004**: Purchase import sends required supplier/item/payment fields through a registered endpoint and uses a valid RFC3339 Riyadh-offset effective date.
- **ACC-005**: Riyadh-midnight dates keep their intended business calendar date after normalization.
- **ACC-006**: New-sale identities differ after abandoning one history entry; reload/back/retry of the same entry retains identity.
- **ACC-007**: Relevant documentation, focused validation, Spec Kit convergence, self-review, designated QA approval, non-force update of the existing PR #91 branch, and GitHub head verification are complete.

## Assumptions and Risks

- Backend #105 and #108 contracts are authoritative as verified against their request models, handlers, and tests.
- PR #91 is open at `f14ed79876ae9f9ef6d473dc88da83d6621d5f08`; the review comment citing `2b96ec3` is stale.
- Existing frontend changes already fix quick-draft context and transmit sale discounts; tests must establish the actual submitted/persisted result.
- The current backend bill-detail response does not expose enough data to safely build credit lines, so an additive read-only full-remaining source endpoint is required; the POST remains authoritative and revalidates transactionally.
- Date-only workbook values must be parsed as Riyadh calendar dates and normalized to RFC3339 with `+03:00`, not sliced from UTC timestamps.
- The local frontend branch name differs from PR #91's remote head branch name; final push must target the existing PR head explicitly and non-force.

## Implementation Decisions

- Credit-note policy: full remaining bill only; require reason and stable request identity; derive amount and note lines from backend source data and refuse when data/remainder cannot be reconciled.
- Backend scope: permit only line-discount persistence for the already-sent field and an additive read-only credit-source snapshot; do not change totals formulas or credit POST validation.
- Baseline: continue on existing PR #91 from `f14ed79`; do not force-push or amend.
