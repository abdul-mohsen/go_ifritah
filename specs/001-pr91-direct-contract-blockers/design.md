# Design: PR #91 Direct Contract Blockers

## Architecture

Keep the existing frontend form, handler, and backend API boundaries. Change only the paths needed to satisfy the already-defined contracts:

1. **Sale discounts**: the frontend already serializes product and manual line discounts. Backend #105 must pass the request value into the existing `bill_product.discount` column; generated line totals and bill-total formulas remain unchanged.
2. **Quick drafts**: retain the existing selected-store branch and cash payment behavior when the focused request test confirms it; fail closed when required context is absent.
3. **Credit notes**: backend #108 adds an authenticated read-only `GET /api/v2/bill/{id}/credit-source` that returns the source bill's exact full remaining credit snapshot. It uses existing source-line and credit-history queries/calculations and checks bill state/store access. The frontend credit form displays that snapshot; the submit handler fetches it again and builds the POST request server-side. The existing credit POST revalidates transactionally, so a stale snapshot cannot over-credit a bill.
4. **Purchase import**: register a protected frontend `POST /api/purchase-bills/bulk-import` for the JSON array already submitted by the import page. Validate each item, normalize its effective date with the existing Riyadh helper, and proxy each purchase bill to backend `/api/v2/purchase_bill`. Return per-item success/failure; do not report success for rejected bills. Require a supplier sequence, payment method, and cost price rather than sending zero placeholders or silently inventing values.
5. **Effective date**: treat a workbook date as a calendar date, not a UTC instant. Parse it without `toISOString()` day slicing, then normalize date-only input on the server with `helpers.ToBackendDate` to RFC3339 carrying Riyadh `+03:00`.
6. **Sale identity**: bind the sale-attempt ID to the browser history entry. Reload/back/resume/retry of the same form entry preserves it; a new form navigation entry receives a fresh ID. Keep the existing POS identity behavior.

## Interfaces and data flow

### Sale

`BuildBillPayload` -> frontend sale handler -> backend #105 `AddBillRequest` -> `addProductToBill` -> `bill_product.discount` -> existing generated totals. No change to discount meaning or formula.

### Credit note

Credit form GET -> frontend requests the verified credit-source snapshot -> render amount and remaining lines. Credit form POST -> frontend fetches a fresh snapshot -> create `{bill_id, branch_id, pos_id, bill_request_id, reason, amount, lines}` -> backend #108 credit POST. The browser does not submit amount/line values as authority. An unavailable or inconsistent snapshot is a visible failure.

### Purchase import

Import page -> protected frontend bulk handler -> per-item validation/date normalization -> backend purchase-bill API -> per-item result. The request keeps the existing JSON array shape. An invalid item is reported as failed and does not become a success-shaped response.

## Persistence and migration

- No schema migration is expected: `bill_product.discount` already exists and feeds generated totals.
- The credit-source endpoint is read-only and additive; it does not persist data or alter the existing credit POST contract.
- Purchase imports continue to persist through the existing backend purchase-bill endpoint.
- Sale identity changes affect browser history state only; no server or database migration.

## Failure handling

- Missing sale branch/payment context, supplier sequence, payment method, cost price, credit reason, credit source metadata, or unprovable credit remainder must stop submission and show an actionable error.
- Purchase import returns one result per item, preserving backend rejection details without claiming success.
- Credit POST remains the authority for concurrency and over-credit protection; if the snapshot becomes stale, the backend rejection is surfaced and the user may reload to obtain a new snapshot.
- Do not substitute zero values, guessed line IDs, guessed VAT, or default supplier sequence values.

## Security and performance

- Preserve existing route RBAC and CSRF protection for the frontend bulk and credit form routes.
- The backend credit-source endpoint uses authentication and source-bill store access checks, returns only the fields needed to create a note, and does not expose unrelated customer data.
- Backend POST validation remains authoritative; the read snapshot is not trusted as an authorization decision.
- Purchase imports use one backend request per bill, matching the response's per-item result semantics.

## Test strategy

- Backend #105: focused handler/query regression proves non-zero line discount reaches the existing insert; verify generated totals formula is untouched.
- Backend #108: handler tests cover remaining lines and amount, partial prior credits, fully credited/draft/inaccessible bills, and missing/unreconcilable source data.
- Frontend: test quick-draft fields, the actual serialized purchase bulk request and error results, Riyadh date boundaries, credit-note request construction from the source snapshot, and history-entry identity behavior.
- Run focused tests first, then repository Go/JS checks for each touched worktree. Review final diffs and Spec Kit convergence before QA.

## Rollout and rollback

Merge backend #105 before stacked backend #108, then merge/update frontend PR #91. Until the additive backend credit-source route is available, the frontend credit-note form must fail visibly rather than fall back to incomplete data. Roll back by reverting the relevant backend/frontend commits; no data migration or generated-data repair is needed.

## Requirement traceability

| Requirement | Design element |
|---|---|
| FR-001 | Sale line discount persistence through the existing query |
| FR-002 | Valid quick-draft context and fail-closed behavior |
| FR-003 | Full-remaining credit-source snapshot plus server-side request construction |
| FR-004 | Protected bulk import proxy with required fields and per-item results |
| FR-005 | Calendar-date parsing and Riyadh RFC3339 normalization |
| FR-006 | History-entry-scoped request identity |
| NFR-001 | Focused request/persistence tests across the actual paths |
| NFR-002 | No accounting formula or credit POST behavior change |
