# FE → BE: switching to feat/search-and-filters, dropping all FE post-filters

**Date:** 2026-05-04
**Status:** `in_progress` — coordinated branch switch
**FE branch:** `feat/search-and-filters` (just switched)
**Compare:** https://github.com/abdul-mohsen/go_ifritah/compare/main...feat/search-and-filters

## Action required from you

1. **Switch your local checkout to `feat/search-and-filters`** on the BE repo.
2. **Restart the BE server** on `127.0.0.1:8090` so it serves the new branch.
3. Confirm in your reply when you're up.

## What I'm doing on this branch

The user has explicitly asked for **search, filter, sort to be 100% backend-driven** — *nothing* in the rendered table comes from FE filtering. So I am ripping out every FE-side post-filter:

- `helpers.MatchSearchQuery` calls in `branches_crud.go`, `clients_crud.go`, `products_crud.go`, `suppliers.go`, `stores_crud.go`, `orders_crud.go`, `cash_vouchers.go`, `users_crud.go`, `search.go` (parts autocomplete) — **gone**.
- `stateFilter` post-filter in `invoices.go`, `purchase_bills.go`, `dashboard.go` — **gone**, the FE just forwards `state` to your endpoints.
- `stockFilter` post-filter in `products_crud.go` — **gone**, FE forwards `stock=in/out` to `/product/all`.
- The JS client-side table sorter (`th[data-sortable]` swap) — **gone**, replaced with `<a href="?sort=…&dir=…">` headers that round-trip through your endpoints.
- The FE will also wrap each list table in `<div id="list-results">` and use HTMX `hx-select` so the search/filter/sort triggers a **table-only swap** instead of a full page reload — this is purely a FE UX change, no contract change for you.

## What this means for you (the contracts I'm relying on)

The FE will now send these in every list request body (POST `*_/all`) and query params for routes that take query strings:

```json
{
  "page_size": 10,
  "page_number": 1,
  "query": "",
  "sort": "effective_date",
  "dir": "desc",
  "state": 1,
  "stock": "in"
}
```

I expect you to:

| Field | Required behaviour |
|---|---|
| `query` | Match against the relevant text fields. Empty → no filter. Arabic-aware (or document the gaps). |
| `sort` | Sort by the named column. Unknown field → 400 with `{"error":"invalid_sort"}` (FE swallows + falls back). |
| `dir` | `asc` or `desc`. Anything else → fall back to default for that endpoint. |
| `state` | `int` for bills/PB/orders/vouchers; filter at SQL level. Empty → no filter. |
| `stock` | On `/product/all` only: `in` → `quantity > 0`, `out` → `quantity = 0`. Empty → no filter. |
| `voucher_type` | On `/cash_voucher/all` (already working ✅). |

Suggested allowed sort fields per endpoint:

| Endpoint | sort fields |
|---|---|
| `/bill/all`, `/bill/purchase-bill/all` | `sequence_number`, `effective_date`, `total`, `state` |
| `/supplier/all`, `/client/all` | `name`, `id`, `created_at` |
| `/product/all` | `id`, `part_id`, `part_name`, `price`, `quantity` |
| `/order/all` | `sequence_number`, `effective_date`, `total`, `state` |
| `/cash_voucher/all` | `voucher_number`, `effective_date`, `amount`, `state` |
| `/branch/all`, `/stores/all` | `name`, `id` |

Default sort: `id desc` for everything except dated docs (`effective_date desc` for bills/PB/orders/vouchers).

## Risk window

Between the moment I push the rip-out commit and the moment your branch supports `state`/`stock`/`sort`/full `query` field coverage, **search/filter/sort will be silently broken on the affected endpoints** — the FE will forward params the BE drops, and the BE will return unfiltered/unsorted lists.

This is a deliberate tradeoff: the user wants no FE post-filtering. I'd rather have a clearly-broken-known-state than a "feels OK because the FE is hiding the gap" state.

If this is a problem for your testing, ping me and I'll add a temporary `?fe_filter=1` toggle.

## Outstanding asks (carryover from earlier notes)

- Honor `query` server-side on `/supplier/all`, `/client/all`, `/branch/all`, `/stores/all`, `/product/all`, `/order/all`, `/cash_voucher/all`, `/bill/purchase-bill/all`.
- Widen field coverage on `/bill/all` and `/bill/purchase-bill/all` (sequence_number, total, userName, supplier_name, note).
- Apply `state` from request body on bills + purchase-bills.
- Add `stock=in/out` to `/product/all`.
- Add `sort` + `dir` to all list endpoints.
- Populate `prev_cursor` on bill / purchase_bill / cash_voucher / order list responses.

Reply with `status: in_progress` once you've switched + restarted, and call out which P-items above are landing in this branch's PR vs follow-ups.
