# PR #91 Direct Contract Blockers — Research

**Baseline**: PR #91 is open at head `f14ed79876ae9f9ef6d473dc88da83d6621d5f08` (`feat/sale-identity-fields-2`) in `abdul-mohsen/go_ifritah`. The review text citing `2b96ec3` is stale. The local worktree is on `fix/pr91-blockers-95-100` at the same SHA; final push must target the existing remote PR branch explicitly.

## Findings

| Area | Evidence | Finding | Decision/status |
|---|---|---|---|
| Sale discounts | Frontend `helpers/form_helpers.go` maps product and manual discounts into `models.BillProductItem`/`BillManualItem`. Backend #105 `pkg/model/bill.go` requires `Discount`; `pkg/handlers/bill.go:addProductToBill` omits it from `AddProductToBillParams`, and `pkg/db/queries/bill.sql:AddProductToBill` omits the column. `RefreshBillTotals` uses generated line totals, so the stored discount remains zero. | Frontend sends the value; backend persistence drops it. | Minimal SQL/query/handler fix; preserve existing generated accounting formula. Backend agent implementing in PR #105 worktree. |
| Quick draft | Existing frontend `handlers/sale_identity_test.go` covers selected-store branch, cash payment method `10`, and missing-branch handling. | Current request path appears to have already replaced zero placeholders. | Preserve if focused test passes; do not claim until agent verifies. |
| Credit note | Frontend `templates/add-credit-note.html` only collects bill ID and note; `handlers/invoices.go:HandleCreateCreditNote` posts only `bill_id` and `note`. Backend #108 `CreditNoteRequest` requires `bill_id`, nonzero `branch_id`, valid `pos_id`/`bill_request_id`, reason, positive amount, and `lines` with bill-product ID, quantity, price, VAT. | Current request cannot satisfy the contract. The normal GET bill detail returns branch but omits bill-product IDs and VAT; `GET /api/v2/credit_bill/:id` returns a credit document, not a source snapshot. | Full remaining-bill only; backend read endpoint derives exact remaining lines/amount; POST remains authoritative. Backend agent implementing in PR #108 worktree. |
| Purchase import | Frontend `templates/import-purchase-bills.html` posts an array to `/api/purchase-bills/bulk-import`; frontend `main.go` registers no such path. Its payload includes sequence but defaults it to `1`, omits `cost_price` and `payment_method`. Backend `AddPurchaseBillRequest` requires supplier ID/sequence, payment method, and item cost price. | The UI path has no request handler and would be incomplete even if routed. A separate `/api/bill-import/upload` workbook flow exists and already maps cost price and `helpers.ToBackendDate`; it is not the path this page calls. | Register a protected per-item bulk proxy; validate required data, normalize date, preserve item-level errors; frontend agent implementing. |
| Effective date | Purchase template uses `Date.toISOString().split('T')[0]`; frontend `helpers/dates.go:ToBackendDate` parses date-only values in `Asia/Riyadh` and emits RFC3339 with `+03:00`; backend purchase request uses `*time.Time`. | UTC date slicing can shift a workbook business date; backend needs an RFC3339 timestamp. | Parse workbook date as a local calendar value; server-normalize date-only input with `ToBackendDate`; test midnight boundaries. |
| Sale identity | `static/js/sale-identity.js` stores the attempt ID in `sessionStorage` and clears it on reload; current JS tests explicitly expect navigation reuse and a new ID on reload. | Existing lifecycle is reversed: a distinct new-sale navigation can reuse an abandoned ID, while reload changes the same form's retry ID. | Scope identity to the browser history entry; same entry retains on reload/back/retry, new entry gets a new ID. |

## Decisions and rationale

- **Credit policy**: issue the full remaining credit only. The client must require a reason and use a verified backend snapshot; it must not guess line IDs, VAT, quantities, or amounts. Backend POST rechecks the request transactionally in case the snapshot became stale.
- **Backend scope**: the owner directed autonomous continuation. Permit the minimal sale-line discount persistence fix and an additive read-only credit-source snapshot endpoint; do not change accounting formulas or existing credit POST validation.
- **Baseline**: continue from current PR #91 head `f14ed79`; no force-push or amend.
- **Import path**: retain the existing JSON-grouped purchase import page and add its missing protected handler rather than redirecting it to a different XLSX upload contract.
- **Identity lifecycle**: bind the attempt identity to a history entry so refresh/back/resume keeps a retry stable while a newly navigated form gets a fresh ID.

## Remaining verification

- Backend #105 discount persistence test and backend #108 credit-source handler tests.
- Frontend focused tests for quick-draft, purchase request fields/date, credit request, and identity lifecycle.
- End-to-end integration, convergence, self-review, independent QA approval, and non-force PR updates remain pending.
