# Handoff Notes — All Active PRs (2026-07-21)

Two repos are in play:
- **go_ifritah** (Go frontend) — https://github.com/abdul-mohsen/go_ifritah
- **ifritah-go** (backend) — https://github.com/abdul-mohsen/ifritah-go

Worktrees are checked out under `C:\ssda\chatGPT\worktrees\`.

---

## Quick Status Overview

| PR | Repo | Title | Branch | E2E | Other checks |
|---|---|---|---|---|---|
| [#56](https://github.com/abdul-mohsen/go_ifritah/pull/56) | frontend | Selling price on purchase bills | `feat/purchase-bill-selling-price` | **pending** (new fixes pushed) | all pass |
| [#55](https://github.com/abdul-mohsen/go_ifritah/pull/55) | frontend | Unify bill import/export + supplier ledger | `feature/bill-import-export-ledger` | **fail** | all pass |
| [#53](https://github.com/abdul-mohsen/go_ifritah/pull/53) | frontend | Fix qa-15 oversell flake | `fix/qa15-stock-enforcement-flake` | no e2e run | all pass |
| [#49](https://github.com/abdul-mohsen/go_ifritah/pull/49) | frontend | Fix purchase-bill date timezone shift | `fix/purchase-bill-date-shift` | **pending** | pending |
| [#48](https://github.com/abdul-mohsen/go_ifritah/pull/48) | frontend | SonarCloud fixes (cookies/log/SRI/a11y) | `fix/sonar-findings` | no e2e run | all pass |
| [#47](https://github.com/abdul-mohsen/go_ifritah/pull/47) | frontend | CI: add golangci-lint validate workflow | `ci/add-validate-workflow` | **fail** | all pass |
| [#61](https://github.com/abdul-mohsen/ifritah-go/pull/61) | backend | Add GET /api/v2/supplier/ledger | `feat/supplier-general-ledger` | n/a | — |

---

## PR #56 — Selling price on purchase bills
**Worktree:** `afrita-go-selling-price`
**Branch:** `feat/purchase-bill-selling-price`
**Backend dependency:** ifritah-go PR #60 (already **merged**)

### What it does
- Adds a `Selling Price` field per item row on add/edit purchase-bill forms
- Pre-fills from the catalog when an existing store product is selected
- Admin/manager can edit it; other roles see it read-only (enforced by backend)
- Adds optional `product_id` column to the Excel/CSV import template so imported rows fetch shelf/selling price for review
- Rows imported without a `product_id` but whose name matches a catalog item get a non-blocking warning (instead of silent duplicate)
- Clarifies "Purchase Price vs Cost Price" wording with hint text

### Key files
| File | What changed |
|---|---|
| `templates/add-purchase-bill.html` | `products_selling_price` input; JS pre-fill on product select; `canEditSellingPrice` flag; import resolution JS |
| `templates/edit-purchase-bill.html` | Same additions for the edit form |
| `templates/components/purchase-bill-item-row.html` | **New.** Shared product-search JS (extracted to pass SonarCloud duplication gate) |
| `handlers/purchase_bill_import.go` | CSV parser hardened for short rows; `resolveImportedItemLookup` endpoint |
| `handlers/search.go` | `HandleProductsSearchJSON` now returns `selling_price` in JSON |
| `handlers/products_crud.go` | Create payload sends both `"part_name"` and `"name"` (bug fix) |
| `helpers/auth_tokens.go` | Per-session mutex on `RefreshTokenIfNeeded` (race condition fix) |
| `e2e/tests/qa-39-...spec.js` | **New.** End-to-end coverage for both frontend behaviors |

### CI fixes pushed in `87b98de` (CI re-run triggered, not yet complete)

**Fix 1 — qa-39 product not found after creation:**
`HandleCreateProduct` was sending `"name": partName` to `POST /api/v2/product`. The backend indexes by `part_name`. Product was created but invisible to text search. Fixed by sending both `"part_name"` and `"name"`.

**Fix 2 — purchase-bills tests redirect to login:**
With 4 parallel workers and 3 qa-39 tests each making 11+ API calls, multiple goroutines simultaneously hit `RefreshTokenIfNeeded` with the same single-use refresh token. First wins; rest get 401 and redirect to `/`. Fixed with a `sync.Map` of per-session `sync.Mutex` — only one goroutine refreshes per session at a time.

### What still needs to happen
1. Wait for CI to go green on the fixes pushed today
2. Review `canEditSellingPrice` window flag role logic matches production backend roles
3. Confirm SonarCloud gate passes (component extraction was specifically to fix duplication gate)
4. Merge to `dev`

---

## PR #55 — Unify bill import/export + supplier ledger
**Worktree:** `afrita-go-bill-import-ledger`
**Branch:** `feature/bill-import-export-ledger`
**E2E status: FAILING**

### What it does
- Shared XLSX import flow for both sales invoices and purchase bills (gated by `RequireBillImportPermission`)
- Matching XLSX export endpoints for both bill types
- New `/dashboard/supplier-ledger` page showing multi-supplier ledger statement
- e2e coverage: `qa-37`, `qa-38`

### E2E failure
Last CI run failed E2E (19 min run). The worktree has commits that tried to fix e2e timeouts and a language assumption on bulk export/import tests (`e7734a0`). Need to check whether the failure is the same issue or something new — push a re-run trigger or look at the CI log.

### Worktree commit tip
```
e7734a0  fix: e2e timeouts and a language assumption on bulk export/import tests
e7bd428  fix: bill-import e2e failure (0-of-2 imported) and Sonar duplication
5b61496  fix: address SonarCloud findings and an accidental Arabic->English header swap
96b5814  feat: unify bill import/export (sales+purchase) and add supplier ledger page
```

### What still needs to happen
1. Diagnose and fix the E2E failure
2. Verify SonarCloud passes (previous commit fixed duplication findings)
3. Merge to `dev`

---

## PR #53 — Fix qa-15 oversell-dialog flake
**Worktree:** `afrita-go-e2e-unified-items-fix`  
**Branch:** `fix/qa15-stock-enforcement-flake`
**CI status: all checks pass** (no e2e run — presumably not needed or was skipped)

### What it does
After PR #52 merged, `qa-15-stock-enforcement-behavior.spec.js` started intermittently failing with `must show a confirm/alert dialog. Got: []`. Root cause: `setSetting()` retries confirming the settings page reflects the new value but doesn't guarantee the *next* add-invoice render has caught up — a timing race, not an app bug.

Fix: added `attemptOversellExpectingDialog()` that re-applies the setting and re-attempts the oversell up to 3 times before asserting. No assertions weakened.

### What still needs to happen
- Looks ready to merge. No E2E gate is blocking it. Confirm with author whether it needs a re-run or is approved.

---

## PR #49 — Fix purchase-bill date timezone shift
**Worktree:** (maps to `fix/purchase-bill-date-shift` — no matching worktree directory found locally, may need to be checked out)
**Branch:** `fix/purchase-bill-date-shift`
**CI status: all checks pending** (checks just triggered)

### What it does
Dates on purchase-bill list/detail/edit pages could shift back one calendar day. Root cause: `extractDateField` and inline date logic used naive `string[:10]` slicing instead of `helpers.ToDisplayDate`. For UTC-offset timestamps, slicing the raw string gives the wrong calendar date before 03:00 Riyadh time. Fix: route all purchase-bill dates through `helpers.ToDisplayDate` (same as `invoices.go` already did).

### Tests added
- `TestExtractDateFieldReLocalizesUTCToRiyadh`
- `TestEditPurchaseBillDatePreservesCalendarDay`
- `TestPurchaseBillDetailDatePreservesCalendarDay`

All three verified to fail before the fix and pass after.

### What still needs to happen
1. Wait for CI to complete (checks just triggered)
2. If green, ready to merge

---

## PR #48 — SonarCloud findings (cookies, log injection, SRI, a11y, lockfile)
**Worktree:** (no matching worktree — `fix/sonar-findings` branch)
**Branch:** `fix/sonar-findings`
**CI status: all checks pass** (no e2e run)

### What it does
Fixes 39 SonarCloud bugs/vulnerabilities across 6 categories:
- **Cookie Secure flag** — logout/refresh/CSRF cookies missing `Secure`/`SameSite`. Added `config.IsLocalhost()` helper; also fixed a latent bug where the old private helper read `os.Getenv("APP_DOMAIN")` directly instead of the resolved `config.AppDomain`
- **Log injection** — request-derived values logged unsanitized. Added `helpers.SanitizeForLog`, applied across auth/middleware/products/orders/purchase-bills/zatca handlers
- **Missing lockfile** — `package-lock.json` was in `.gitignore`. Removed exclusion and committed a fresh lockfile (0 vulnerabilities)
- **Keyboard accessibility** — 5 click-only elements now have `role="button"`, `tabindex="0"`, and `onkeydown` (Enter/Space)
- **Missing table headers** — 5 import-preview tables populated entirely by JS now have static `<thead>` placeholders
- **Subresource Integrity** — 4 CDN `<script>` tags (htmx ×2, chart.js, qrious.js) now have SHA-384 `integrity` + `crossorigin="anonymous"`. Cairo font self-hosted (was Google Fonts) with 3 variable-font woff2 files + `fonts.css`

### What still needs to happen
- Looks ready to merge. No E2E gate blocking it. All other checks pass.

---

## PR #47 — CI: add golangci-lint validate workflow
**Worktree:** (no matching worktree)
**Branch:** `ci/add-validate-workflow`
**E2E status: FAILING** (but this PR adds zero Go code)

### What it does
Adds `.github/workflows/validate.yml` running `golangci-lint-action@v6` (pinned to v1.64.8 — the repo's `.golangci.yml` is v1-format; v7+ requires v2 config) with `only-new-issues: true` so pre-existing debt doesn't block new work.

### E2E failure
The E2E failure is **pre-existing on `dev`** — this PR's branch was cut from a point where dev already had intermittent failures. The PR itself adds no application code. The E2E failure is unrelated to the lint workflow change.

### What still needs to happen
- E2E failure needs to be resolved on `dev` first, or this PR needs to be rebased onto a clean `dev` tip
- Once E2E is clean, this is a trivial merge

---

## PR #61 (backend) — Add GET /api/v2/supplier/ledger
**Worktree:** `ifritah-backend-supplier-ledger`
**Branch:** `feat/supplier-general-ledger` (ifritah-go repo)
**Depends on:** Frontend PR #55 (supplier ledger page) to be useful end-to-end

### What it does
Adds a new `GET /api/v2/supplier/ledger` backend endpoint that returns a supplier's general ledger (purchases, payments, balances). This is the backend half of the supplier ledger statement feature.

### Worktree commit tip
```
2876b72  feat: add GET /api/v2/supplier/ledger (supplier general ledger)
```

### What still needs to happen
1. Backend tests / CI review
2. Coordinate merge with frontend PR #55

---

## Worktrees not tied to open PRs

These local worktrees exist but their branches are **not open PRs** — they appear to be intermediate work or were superseded:

| Worktree | Branch | Notes |
|---|---|---|
| `afrita-go-e2e-unified-items-fix` | `fix/e2e-unified-items-stale-selectors` | Upstream is `origin/dev` — work from before PR #52 merged, likely superseded |
| `afrita-go-i18n-fix` | `fix/unify-localization` | At commit `04c434f` (same as `dev` at that point) — appears stale/abandoned |
| `afrita-go-ledger-multi-supplier` | `feature/ledger-statement-multi-supplier` | No remote upstream. Likely superseded by what shipped in `dev` as commit `233d9d9`. Contains perf improvement (`b8995a8`) for multi-supplier report using combined endpoint |
| `afrita-go-revert-e2e-concurrency` | `revert/e2e-shared-backend-concurrency` | A revert of the "serialize e2e runs" commit — this revert is already on `dev` as `e48562c` |
| `ifritah-backend-multi-supplier-report` | `feat/multi-supplier-report` | Backend PR #59 already **merged**. Worktree is stale |
| `ifritah-backend-purchase-bills` | `feat/purchase-bill-line-fields` | No remote upstream. May be a staging area for backend work |
| `ifritah-backend-selling-price` | `feat/purchase-bill-selling-price` | Backend PR #60 already **merged**. Worktree is stale |

The stale worktrees can be removed if not needed:
```bash
git worktree remove C:\ssda\chatGPT\worktrees\afrita-go-i18n-fix
git worktree remove C:\ssda\chatGPT\worktrees\afrita-go-revert-e2e-concurrency
git worktree remove C:\ssda\chatGPT\worktrees\ifritah-backend-multi-supplier-report
git worktree remove C:\ssda\chatGPT\worktrees\ifritah-backend-selling-price
```

---

## Merge order / dependency chain

The PRs are not strictly ordered but here is the recommended sequence:

```
#48 (SonarCloud fixes)        → merge first, no dependencies, all checks pass
#53 (qa-15 flake fix)         → merge second, all checks pass
#47 (lint CI workflow)        → needs clean dev E2E first; trivial after
#49 (date timezone fix)       → waiting for CI; no dependencies
#56 (selling price)           → wait for CI re-run after today's fixes
#61 backend (supplier ledger) → coordinate with #55
#55 (bill import + ledger)    → needs E2E fixed; depends on #61 for full end-to-end
```

---

## Shared infrastructure notes

### `GODEBUG=jstmpllitinterp=1`
Required whenever running the Go server with templates that call `{{ L "..." }}` inside JS backtick string literals. Go 1.21 made this an execution error. The flag restores pre-1.21 behaviour. It is set in `e2e.yml` for CI — **must also be set locally**.

### Token refresh race (fixed in `87b98de`)
`helpers.RefreshTokenIfNeeded` now uses a per-session `sync.Mutex` (stored in a `sync.Map`). Before this fix, any parallel test load that caused multiple goroutines to see the token near-expiry at the same moment would corrupt the session — each goroutine tried to refresh using the same single-use refresh token. This was the root cause of the intermittent auth-redirect failures in the parallel e2e project.

### E2E auth model
- All tests load `.auth/storageState.json` (created by `e2e/global-setup.js` at the start of the run)
- The stored state contains the `session_id` cookie which maps to a server-side session in `config.SessionTokens`
- The Go server is what holds the session in memory — not the browser. The cookie is just an ID.
- If the server restarts between global-setup and the tests, all sessions are lost (unless token files on disk are loaded at startup via `AFRITA_TOKEN_DIR`)

### Local worktree paths
```
C:\ssda\chatGPT\worktrees\
  afrita-go-bill-import-ledger       ← PR #55
  afrita-go-selling-price            ← PR #56 (this file lives here)
  ifritah-backend-supplier-ledger    ← PR #61 (backend)
  
  afrita-go-e2e-unified-items-fix    ← stale
  afrita-go-i18n-fix                 ← stale
  afrita-go-ledger-multi-supplier    ← stale (may have useful perf commit)
  afrita-go-revert-e2e-concurrency   ← stale
  ifritah-backend-multi-supplier-report  ← stale (PR #59 merged)
  ifritah-backend-purchase-bills     ← stale (no open PR)
  ifritah-backend-selling-price      ← stale (PR #60 merged)
```
