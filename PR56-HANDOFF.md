# PR #56 — Handoff Notes

**Branch:** `feat/purchase-bill-selling-price`
**PR:** https://github.com/abdul-mohsen/go_ifritah/pull/56
**Status as of 2026-07-21:** Latest CI run had 5 failures. Two fixes were pushed in commit `87b98de` — CI has not re-run yet.

---

## What this PR does

Adds a **Selling Price** field to the add/edit purchase-bill forms:

- Pre-filled from the product catalog when an existing store product is selected
- Admin/manager can edit it; other roles see it read-only
- Role enforcement lives in the backend (ifritah-go PR #60); the frontend degrades gracefully without it
- Adds an optional `product_id` column to the Excel/CSV import template so imported rows can fetch shelf/selling price for review
- Rows imported without a `product_id` but whose name matches an existing catalog item get a non-blocking warning instead of silently duplicating the entry
- Clarifies "Purchase Price vs Cost Price" wording with hint text

**Depends on:** `ifritah-go` backend PR #60 for the admin/manager override enforcement. The feature works without it — the backend just ignores unauthorized overrides.

---

## Key files changed in this PR

| File | What changed |
|---|---|
| `templates/add-purchase-bill.html` | Added `products_selling_price` input per row; JS to pre-fill on product select; `canEditSellingPrice` window flag; import resolution JS |
| `templates/edit-purchase-bill.html` | Same selling-price additions for the edit form |
| `templates/components/purchase-bill-item-row.html` | **New.** Shared JS for store-product search extracted here to avoid SonarCloud duplication gate |
| `handlers/purchase_bills.go` | Passes `pb_pdf_required` setting; state filter is now BE-driven; removed client-side state filter |
| `handlers/purchase_bill_import.go` | CSV parser hardened (`FieldsPerRecord = -1`) for short/trimmed rows; new `resolveImportedItemLookup` endpoint |
| `handlers/search.go` | `HandleProductsSearchJSON` returns `selling_price` (= `price`) in the JSON payload |
| `handlers/products_crud.go` | Create payload now sends both `"part_name"` and `"name"` (fix for qa-39) |
| `helpers/auth_tokens.go` | Per-session mutex on `RefreshTokenIfNeeded` to prevent concurrent-refresh race (fix for parallel test failures) |
| `e2e/tests/qa-39-purchase-bill-selling-price-and-import-lookup.spec.js` | **New.** End-to-end tests for the two frontend behaviors |
| `e2e/tests/qa-30-purchase-bill-stock-price-labels.spec.js` | Updated: stock rows now assert purchase + cost + selling price columns |

---

## CI failures that were fixed (commit `87b98de`)

The last CI run before `87b98de` had **5 failures**:

### Failures 3–5 — `qa-39` tests: `createStoreProduct` not found after creation

**Symptom:**
```
Error: store product "QA39-Sell-..." did not appear in search-json after creation
```
The `createStoreProduct` helper in `qa-39` POSTs to `/dashboard/products/create` (succeeds), then polls `/api/products/search-json` 10 times over 5 seconds — the product never appears.

**Root cause:**
`HandleCreateProduct` was sending `"name": partName` to the backend's `POST /api/v2/product`. The backend's text search indexes products by the `part_name` field, not `name`. Product was created but invisible to search.

**Fix (`handlers/products_crud.go`):**
Send both `"part_name": partName` and `"name": partName` in the create payload.

---

### Failures 1–2 — `purchase-bills.spec.js` list/delimiter tests: redirect to login

**Symptom:**
```
Error: expect(page).toHaveURL(expected)
Expected pattern: /purchase-bills/
Received string:  "http://localhost:8001/"
```
After `login(page)`, navigating to `/dashboard/purchase-bills` redirected to `/` (the login page) instead of loading the list.

**Root cause:**
`GetTokenOrRedirect` calls `RefreshTokenIfNeeded` on every request. With 4 parallel workers and 3 new qa-39 tests each making 11+ API calls, multiple goroutines simultaneously detected the session token near-expiry and all raced to `POST /api/v2/refresh` using the **same single-use** refresh token. The first caller got a new token; all others got a 401. On 401, `RefreshTokenIfNeeded` returned `false`, causing `GetTokenOrRedirect` to redirect to `/`.

**Fix (`helpers/auth_tokens.go`):**
Added a `sync.Map` of per-session `sync.Mutex` values. Only one goroutine per session calls the backend to refresh at a time. The others block, then re-check `ShouldRefreshToken` after the lock is released — the token is now fresh so they skip the refresh entirely.

---

## What still needs to happen before merge

1. **CI must pass green** — push `87b98de` triggered a new CI run; wait for results. If it passes, the PR is ready to review/merge.

2. **Backend PR #60 must be merged first** (or merged simultaneously) — the frontend sends `products_selling_price` in the purchase-bill payload. Without the backend PR, the field is accepted but silently dropped. The UI will still work, just without role enforcement.

3. **Review the `canEditSellingPrice` window flag** — it's set in `add-purchase-bill.html` via a Go template expression. Confirm the role detection logic matches the backend's role list for the production environment.

4. **SonarCloud gate** — the component extraction in `1e6ea0a` was specifically to fix a duplicated-lines gate. Confirm the gate passes after the CI run.

---

## Local setup to continue work

```bash
# The worktree is already checked out at:
cd C:\ssda\chatGPT\worktrees\afrita-go-selling-price

# Run Go unit tests
go test ./...

# Build
go build -o tmp/afrita.exe .

# Run the server locally (needs .env with BACKEND_DOMAIN etc.)
PORT=8000 BACKEND_DOMAIN=https://dev.ifritah.com GODEBUG=jstmpllitinterp=1 ./tmp/afrita.exe

# Run the new e2e spec only (assumes server is running on :8000)
cd e2e
npx playwright test qa-39 --project=parallel
```

**Important env flag:** `GODEBUG=jstmpllitinterp=1`
Go 1.21+ rejects `{{ L "..." }}` calls inside JS template literals (backticks) at execution time. This flag restores pre-1.21 behaviour. It is set in `e2e.yml` for CI and must be set locally when running the server against templates that use `L` inside backtick strings.

---

## Branch/commit map

```
origin/main
    └── ... (base)
         ├── 233d9d9  feat: multi-supplier ledger export          (merged from main)
         ├── 8769e73  fix(ci): serialize e2e runs                 (merged from main, then reverted)
         ├── e48562c  Revert serialize e2e runs (#57)             (merged from main)
         ├── 46fad3e  feat: surface selling price (core feature)
         ├── d4e54f5  fix: e2e/CI failures for selling-price
         ├── 1e6ea0a  refactor: dedupe item-row JS (SonarCloud)
         └── 87b98de  fix: qa-39 + parallel token-refresh race    ← HEAD (latest push)
```

---

## Relevant test files

| Test | What it covers |
|---|---|
| `e2e/tests/qa-39-...spec.js` | Selling price pre-fill on product select; import with product_id fetches shelf+price; import without id warns on name match |
| `e2e/tests/qa-30-...spec.js` | Purchase-bill stock row has purchase + cost + selling price columns |
| `e2e/tests/purchase-bills.spec.js` | List page loads; no leaked template delimiters; add form loads; supplier combobox; duplicate check |
| `handlers/purchase_bill_import_test.go` | CSV parser handles short rows (missing product_id column) |
| `handlers/products_search_json_test.go` | Search JSON endpoint returns correct products for Arabic/English queries |
| `helpers/auth_tokens_test.go` | `ShouldRefreshToken` window; token refresh round-trip |
