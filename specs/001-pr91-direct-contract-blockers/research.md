# PR #91 Direct Contract Blockers — Research

**Original blocker baseline**: The initial repair started from PR #91 at `f14ed79876ae9f9ef6d473dc88da83d6621d5f08`. The review text citing `2b96ec3` is stale.

**Sonar follow-up baseline**: PR #91 is at pushed head `187357e87fefe2b6b29e5746de0b857c232e2d84` on remote branch `feat/sale-identity-fields-2`. The local follow-up worktree branch is `fix/pr91-blockers-95-100`; publish non-force to the existing PR branch.

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
- **Baseline**: the original blocker work started at PR #91 `f14ed79`; the Sonar follow-up starts at current pushed head `187357e` and must use a non-force push to the existing PR branch.
- **Import path**: retain the existing JSON-grouped purchase import page and add its missing protected handler rather than redirecting it to a different XLSX upload contract.
- **Identity lifecycle**: bind the attempt identity to a history entry so refresh/back/resume keeps a retry stable while a newly navigated form gets a fresh ID.

## Verification status

- Original blocker implementation and delivery through T011-T012 are complete; see `STATUS.md` for validation and push evidence.
- Sonar follow-up source fixes, local checks, and T016 convergence/self-review are complete. T017 non-force push and T018 remote Quality Gate verification remain tracked in `STATUS.md`.

## SonarCloud Quality-Gate Follow-up

### CI and quality-gate evidence

- The failing check is SonarCloud's external `SonarCloud Code Analysis` check for frontend PR #91 at head `187357e87fefe2b6b29e5746de0b857c232e2d84`; ordinary GitHub Actions on that head passed.
- Backend PR #105 and #108 SonarCloud checks passed, so this follow-up is limited to frontend PR #91.
- The SonarCloud quality-gate API reports `new_reliability_rating=3` and `new_security_rating=4` against a threshold of `1`. New maintainability rating, duplication density, and security-hotspot review are within their thresholds.
- The SonarCloud issue search filtered to `BUG,VULNERABILITY` returns exactly three open PR #91 findings: two `Web:InputWithoutLabelCheck` reliability issues in `templates/add-credit-note.html` and one `javascript:S1523` security issue in `js-tests/support.js`.

### Source and test-harness trace

- Both credit-note labels are adjacent to their controls but have no `for` attribute; the bill ID input and reason textarea have no IDs. Existing form selectors use the controls' `name` attributes, so unique IDs and matching label targets can be added without changing submission behavior. Existing templates consistently use explicit `for`/`id` pairs.
- `js-tests/support.js` reads a caller-provided relative path under `static/` and evaluates it with `vm.runInContext`. Repository search found only two callers, both fixed: `js/sale-identity.js` and `js/sale-discount.js`.
- The two scripts are plain browser IIFEs that attach listeners to the supplied page's `window`/`document`; their tests depend on per-page state. The Node test setup is `node:test` and the package has no DOM-simulation dependency. Both browser scripts are loaded normally from `templates/layouts/base.html`.
- An explicit allowlist of CommonJS-imported installer functions can replace VM evaluation while preserving fresh per-page test setup. Each browser script can auto-install against `window` in a browser and export its installer under CommonJS, so tests pass the isolated test page directly. This avoids dynamic evaluation and adds no dependency.

### Decision and remaining verification

- Correct only the three findings that drive the new reliability/security gate; do not broaden the change to unrelated maintainability issues or alter Sonar configuration.
- Keep existing input names, form values, browser script behavior, and test coverage; add programmatic label associations and invoke only allowlisted installers.
- Verify the affected Go template-render test, JavaScript identity/discount tests, full JS suite, formatter/lint checks for touched files, then push non-force to the existing PR #91 branch and verify SonarCloud's new analysis passes.
- Remaining implementation check: confirm the template-render regression asserts both label/control pairs, and confirm the test harness rejects unknown script keys rather than evaluating an arbitrary path.
