# 2026-05-04 — search-and-filters loop closed (FE ↔ BE)

Summary of the peer-dev mailbox exchange on thread `search-and-filters`
(messages 1–9 in `shared-chat/search-and-filters/`). Both sides shipped
and closed the thread; this doc captures the contract that's now live on
`feat/search-and-filters` so the next agent has the full record without
needing the mailbox.

**FE branch:** `feat/search-and-filters` (head `50f3dc1`)
**BE branch:** `feat/search-and-filters` (head `ef3f89c`, PR #29)
**FE compare:** https://github.com/abdul-mohsen/go_ifritah/compare/dev...feat/search-and-filters
**BE compare:** https://github.com/abdul-mohsen/ifritah-go/compare/dev...feat/search-and-filters

## Round 1 — BE shipped (msg #1)

- **Search-on-list, 5 endpoints**: top-level `query` (`*string`) on the
  POST body; sentinel-filter pattern, Sonar-clean static SQL.
  - `/supplier/all` → `name`, `phone_number`, `vat_number`
  - `/client/all` → `name`, `email`, `phone`
  - `/branch/all` → `name`, `address`, `phone`
  - `/stores/all` → `name` (in-memory; ≤10 rows)
  - `/product/all` → `name`, `shelf_number`, plus exact `id` when q
    is all-digits. (`part_name` lives on `bill_product` /
    `order_items`, not the catalog table, so BE used `shelf_number`.)
- **Bill / purchase_bill search**:
  - `/bill/all` matches `userName`, `user_phone_number`, plus exact
    `sequence_number` for digit queries.
  - `/purchase_bill/all` matches `supplier.name` and exact
    `supplier_sequence_number` for digit queries.
  - **Deferred**: `total` exact-match — sqlc v1.31 emits
    non-nullable `decimal.Decimal` for an `sqlc.narg` against a
    NOT-NULL decimal column; CONCAT / CAST AS CHAR didn't help.
    Tracked as a known limitation; revisit with
    `decimal.NullDecimal` migration later.
- **`state` filter** added on `/bill/all`, `/purchase_bill/all`,
  `/cash_voucher/all`. `null`/missing/`-1` = any non-deleted; `0/1/2/3`
  = exact match.

## Round 1 reply — FE (msg #2)

FE acknowledged §1–§3 and went further than asked: **ripped out every
FE post-filter / sort block** and switched to 100% backend-driven
search/filter/sort with table-only HTMX swap.

Key changes:
- `helpers/list_fetch.go` (new) — `ListOpts {Page, PerPage, Query,
  Sort, Dir, State, Stock, VoucherType, Role}`; `FetchBranchesList`,
  `FetchClientsList`, `FetchSuppliersList`, `FetchStoresList`,
  `FetchProductsList`, `FetchOrdersList`. No caching on these.
- `helpers/api_helpers.go` — `FetchInvoicesAll(token, page, query,
  state, sort, dir)` and `FetchPurchaseBillsAll(token, page, query,
  state, sort, dir)`. Cache only on the default zero-arg call.
- `helpers/cash_voucher_helpers.go` — `FetchCashVouchers(token, page,
  perPage, query, voucherType, sort, dir)`.
- `handlers/{branches,clients,stores,suppliers,products,orders,
  invoices,purchase_bills,cash_vouchers,users,dashboard}.go`,
  `handlers/search.go` — every `MatchSearchQuery` /
  `stateFilter` / `stockFilter` block deleted; params forwarded to
  BE; templates get `sort`/`dir`.
- `templates/*.html` (10 list pages) — HTMX
  `hx-get / hx-target=#list-results / hx-select=#list-results /
  hx-swap=outerHTML / hx-push-url=true` on the form so only the
  table region swaps; hidden `<input name="sort">` /
  `<input name="dir">`; per-`<th>` `data-sort-key` +
  server-rendered `data-sort-current`.
- `static/js/script.js` — live-search IIFE skips HTMX-controlled
  forms; sort IIFE cycles `none → asc → desc → none`, sets the hidden
  inputs, calls `form.requestSubmit()` (HTMX intercepts); rebinds on
  `htmx:afterSwap`.
- Tests updated: `branches_crud_test.go` and
  `products_search_json_test.go` mock BE now honors `query`
  (Arabic-aware via `helpers.MatchSearchQuery`), proving FE→BE
  forwarding instead of FE post-filtering.

FE asked BE for: `sort`/`dir` on all 9 endpoints, `stock`
(`in`/`out`/`low`) on `/product/all`, `role` on `/users/all` (when
that lands), wider `query` on bill (`client.name`) and purchase_bill
(exact `id`).

## Round 2 — BE shipped (msg #3, PR #29 head `8f296b2`)

- **`sort` + `dir` on every list endpoint**, accepted in the JSON
  body. Default `dir` is `asc`; unknown keys silently ignored (no
  500); empty `sort` keeps canonical seek order. Implementation: in-
  memory stable sort over the page; strings compare case-insensitive;
  decimals parse via `decimal` so `"9" < "10"` is correct.

  | endpoint | accepted sort keys |
  |---|---|
  | `/branch/all` | `name`, `address` |
  | `/client/all` | `name`, `company_name`, `email`, `phone` |
  | `/stores/all` | `id`, `name` |
  | `/supplier/all` | `name`, `email`, `phone_number`, `address`, `vat_number` |
  | `/order/all` | `sequence_number`, `client`, `total`, `status` |
  | `/product/all` | `id`, `part_name` (alias of `name`), `price`, `quantity`, `status` |
  | `/bill/all` | `sequence_number`, `total`, `effective_date`, `type`, `state` |
  | `/purchase_bill/all` | `supplier_sequence_number`, `id`, `total`, `effective_date`, `state` (no `type` column on schema — sent key is a no-op) |
  | `/cash_voucher/all` | `voucher_number`, `voucher_type`, `recipient_name`, `amount`, `effective_date`, `state` |

- **`stock` filter on `/product/all`**: `in` → `quantity > 0`,
  `out` → `quantity = 0`, `low` → `quantity <= product.min_stock`.
  Combines with `query`.
- **`MaxLimit` bumped 100 → 10000**, unlocking the FE one-shot
  fetch-all mode.
- **`/bill/all` `query` widened** to also LIKE `client.name` (B2B).
- **`/purchase_bill/all` `query` widened**: digits also match `b.id`
  exact.
- Wire fields added: `pkg/model/pagination.go` `PaginationRequest`
  → `Dir`, `Stock`; `pkg/model/bill.go` `BillRequestFilter` → `Dir`;
  `pkg/pagination/listquery.go` `ListRequest` → `Dir` (+ allows
  non-canonical sort keys when no cursor, so FE-flavored keys don't
  400).

## Smoke probe — FE (msg #6)

FE walked the four asks against BE on `:8090`. Results:

- ✅ `/api/v2/branch/all`, `/api/v2/client/all`, `/api/v2/supplier/all`,
  `/api/v2/order/all`, `/api/v2/bill/all`,
  `/api/v2/purchase_bill/all`, `/api/v2/cash_voucher/all` all 200.
- ❌ **`/api/v2/product/all` 500** on every variant (baseline,
  stock=*, sort=*, query=*).
- ❌ **`/api/v2/stores/all` 404 on POST**, 200 on GET — wire
  mismatch with the round-2 message that implied POST.

Smoke 1 (clients sort=phone), Smoke 3 (bills state=1 sort=total
dir=desc, items=7, totals descending, all state=1), Smoke 4
(purchase_bill query=99101 → 1 item, supplier_sequence_number=99101,
id=394) all passed.

## Round 3 — BE fixes (msg #7, head `ef3f89c`)

- **Products 500**: MySQL 1052 ambiguous `is_deleted`. The
  `GetAllProduct` SQL joined `user JOIN store JOIN product` and the
  WHERE clause's `is_deleted = False` was unqualified; round-1's
  OR-tree extension changed join resolution and tripped MySQL 8's
  ambiguity guard. Fix: qualify to `p.is_deleted = False` in
  `GetProduct` and `GetAllProduct`. Verified locally.
- **`/stores/all` wire**: BE picked **both**. GET stays canonical
  (FE already uses it); POST now also routed to the same handler.
  Either method accepts query string `?query=&sort=&dir=` (GET) or
  JSON body (POST). Query-string overlay wins when both present.

## Final smoke + close (msgs #8, #9)

FE re-walked all 8 list pages on `:8000` against BE on `:8090`:

| Page | URL params | Result |
|---|---|---|
| Clients | `?sort=phone&dir=desc` | ✅ 200, sort indicator + hidden inputs match |
| Products | `?stock=low&sort=quantity&dir=asc` | ✅ 200 |
| Invoices | `?state=1&sort=total&dir=desc` | ✅ 200 |
| Purchase-bills | `?q=99101` | ✅ 200 |
| Orders | `?sort=total&dir=desc` | ✅ 200 |
| Cash-vouchers | `?sort=amount&dir=asc` | ✅ 200 |
| Branches | `?sort=name&dir=asc` | ✅ 200 |
| Suppliers | `?sort=name&dir=asc` | ✅ 200 |
| Stores | `?sort=name&dir=asc` | ✅ 200 |

Also verified an HTMX-style request (`HX-Request: true`) on
`/dashboard/products`: server returns the full page; FE
`hx-select="#list-results"` extracts only the table region — no
full-page reload semantics on the client. `data-sort-current` is
rendered server-side from the URL params.

BE marked **done**; FE marked **done**; thread closed by FE.

## Post-close polish

- **Animation flicker on reload** (commit `50f3dc1`): the body-end
  IIFE that restores collapsed sidebar sections from localStorage
  ran after first paint, so adding `.closed` triggered a live
  `max-height 500px → 0` transition (visible flicker). Fixed by
  adding a `preload-no-anim` class on `<html>` from an inline
  `<head>` script; a scoped `<style>` disables all transitions /
  animations while present; the restore IIFE removes the class
  after two `requestAnimationFrame` calls so user-initiated
  toggles still animate normally.

## Deferred to follow-up PRs

| Item | Why deferred |
|---|---|
| `total` exact-match for bill / purchase_bill | sqlc v1.31 nullable-decimal narg limitation; revisit with `decimal.NullDecimal`. |
| `prev_cursor` for true backward paging (§6) | meaningful change to every cursor handler; separate PR `feat/list-prev-cursor`. |
| Wire normalization §7 (`*time.Time`, `NULLIF(user_phone_number, '')`) | FYI-only on FE side; piggyback on the §6 PR. |
| `/users/all` role filter | endpoint not yet implemented; FE still mocks. |
| Arabic-aware case-insensitive matching (harakat, alef, ta-marbuta, indic digits) | needs collation work on BE; separate ticket. |

## Files touched on FE this round

- `helpers/list_fetch.go` (new)
- `helpers/api_helpers.go`, `helpers/cash_voucher_helpers.go`
- `handlers/{branches_crud, clients_crud, stores_crud, suppliers,
  products_crud, orders_crud, invoices, purchase_bills,
  cash_vouchers, users_crud, dashboard, search}.go`
- `templates/{branches, clients, stores, suppliers, orders,
  products, invoices, purchase-bills, users, cash-vouchers}.html`
- `templates/layouts/base.html` (animation flicker fix)
- `static/js/script.js`
- `handlers/branches_crud_test.go`,
  `handlers/products_search_json_test.go` (mock BE now honors
  `query`)

— FE
