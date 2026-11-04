# Implementation Plan: PR #91 Direct Contract Blockers

## Technical Context

- **Application**: Go/Mux/HTMX frontend with browser JavaScript; backend Go/Gin API.
- **Frontend branch/worktree**: local `fix/pr91-blockers-95-100`, initial baseline SHA `f14ed79876ae9f9ef6d473dc88da83d6621d5f08`; current Sonar remediation starts from PR #91 head `187357e87fefe2b6b29e5746de0b857c232e2d84` on existing remote branch `feat/sale-identity-fields-2`.
- **Backend branches**: PR #105 `feat/atomic-sale`; PR #108 `feat/b6-credit-debit-2`, stacked on #105.
- **Contracts**: backend sale line discount is a decimal value; purchase effective date is a JSON `time.Time`; credit-note POST requires branch, idempotency identity, reason, positive amount, and source line data.
- **Date behavior**: application helper uses `Asia/Riyadh`, serializing to RFC3339 with `+03:00`.
- **Persistence**: no migration planned; the sale line discount column exists. The repair passes the existing field through the insert.
- **Dependencies**: Go modules and existing sqlc generation in the backend; browser JS tests use the repository's existing Node test setup.
- **Unknowns**: the SonarCloud-specific source findings and test-helper constraints are resolved in `research.md`; remaining implementation assumptions must be validated by focused tests.

## Constitution Check

No `.specify/memory/constitution.md` or project-level Spec Kit scripts/templates are present in this worktree. The feature uses the established `specs/001-pr91-direct-contract-blockers/` location recorded by `.specify/feature.json`. Scope is narrow, preserves accounting formulas, adds no schema migration, and requires explicit error reporting and regression tests. No constitution violation is identified.

## Design and quality gates

- Requirements and contract research are recorded in `requirements.md` and `research.md`.
- Technical design, data model, API contracts, and runnable checks are in `design.md`, `data-model.md`, `contracts/`, and `quickstart.md`.
- Dependency-ordered implementation tasks are in `tasks.md`.
- `STATUS.md` tracks implementation, risks, convergence, QA, and PR delivery.
- The repository-specific `.specify/scripts/bash/setup-plan.sh` and plan template are absent; artifacts are maintained in the existing feature directory without initializing or overwriting unrelated repository scaffolding.

## Implementation waves

1. **Parallel backend work**: persist sale-line discount in PR #105; add read-only full-remaining credit-source endpoint in PR #108.
2. **Frontend work**: verify draft behavior; register purchase bulk import and correct payload/date; integrate full-remaining credit source; fix sale identity lifecycle.
3. **Sonar remediation**: fix the two label associations and remove unsafe dynamic execution from the JS test harness without changing the Sonar workflow or thresholds.
4. **Convergence and self-review**: run targeted checks, review diffs/artifacts, update applicable documentation, and resolve any remaining gaps.
5. **Delivery**: non-force push to the existing PR #91 branch, verify the new SonarCloud analysis passes, then hand the PR back to the owner for review.

## Rollback

Revert the associated commits if any request path or compatibility check fails. No database migration or backfill is required.
