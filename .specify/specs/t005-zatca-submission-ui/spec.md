# Specification: T005 Frontend C2 Submission Outcome UI

## User stories

1. An operations user can distinguish all six durable ZATCA submission
   outcomes on invoice list and detail pages.
2. An operations user sees an invoice as issued only after an accepted or
   accepted-with-warning outcome.
3. An operations user can inspect provider warning/error, retry, and artifact
   references when the backend returns them.
4. Existing invoices without durable outcome data retain current localized
   statuses and actions.

## Functional scope

- Read the existing backend ZATCA monitor submissions route.
- Merge rows into invoice list/detail view models by invoice ID.
- Preserve legacy numeric invoice and credit-note status fallback.
- Guard issued-only status/actions using explicit success states.
- Add a focused regression test for rejected, ambiguous, and pending.

## Success criteria

| ID | Criterion |
|---|---|
| SC-001 | The focused regression is red at `18f7988` and green after T005. |
| SC-002 | The six durable states render distinctly when returned by the backend. |
| SC-003 | Only accepted and accepted-with-warning resolve to issued. |
| SC-004 | Provider/retry/artifact contract fields reach list/detail page data. |
| SC-005 | Existing localized legacy and credit-note statuses remain intact. |
| SC-006 | Formatting, targeted tests, and `go test ./...` pass. |
| SC-007 | The final commit contains no backend/daemon changes and is not pushed. |

## Constraints

- Base frontend commit is `18f7988`.
- Backend/daemon repositories are read-only reference sources.
- No new dependency or API endpoint may be introduced by T005.
