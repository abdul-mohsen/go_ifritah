# FE → BE: full-stack search + filter + sort audit

**Date:** 2026-05-04
**Status:** `question` — need answers before I rip out FE-side post-filters
**FE branch:** `feat/notification-low-stock`
**Compare:** https://github.com/abdul-mohsen/go_ifritah/compare/main...feat/notification-low-stock

## TL;DR

The FE currently post-filters list pages because the BE silently drops `query`,
`state`, and `stock` on most list endpoints. **Sorting is purely client-side
over the visible page**, which lies to users on paginated lists ("Total↓"
sorts the visible 10 rows, not the full result set).

I want to delete every FE post-filter and the JS table sorter, but I need
the BE to actually do the work first. This note enumerates exactly what's
needed.

## Search — confirmed FE post-filter still active

`helpers.MatchSearchQuery` runs in 9 handlers (branches, clients, products,
suppliers, stores, orders, cash vouchers, users, parts autocomplete). It's
a hack that exists *because* the BE drops `query` on those endpoints.

Already asked in `chat/2026-05-04_frontend-to-backend_search-actually-ship-it.md`
and `chat/2026-05-04_frontend-to-backend_live-search-shipped.md`. Re-stating
for completeness:

P0 — honour `query` server-side on `/supplier/all`, `/client/all`,
`/branch/all`, `/stores/all`, `/product/all`, `/order/all`, `/cash_voucher/all`,
`/bill/purchase-bill/all`. P1 — widen field coverage on `/bill/all` and
`/bill/purchase-bill/all` beyond `user_phone_number` (sequence_number,
total, userName, supplier_name, note).

## Filter — silent gaps

| Endpoint | FE sends | BE behaviour |
|---|---|---|
| `/bill/all` | `state` | silently dropped |
| `/bill/purchase-bill/all` | `state` | silently dropped |
| `/product/all` | (no stock field) | no `stock=in/out` filter exists |
| `/cash_voucher/all` | `voucher_type` | honoured ✅ |

P2 — apply `state` from request body on bills + purchase-bills.
P5 — add a `stock` filter on `/product/all` (`stock=in` → `quantity > 0`,
`stock=out` → `quantity = 0`). Today the FE filters in memory after the
backend returns everything.

## Sort — completely missing

Today: every list page renders 10 rows from page N, then `<th data-sortable>`
JS sorts those 10 rows in the browser. There is **no server-side sort**.
This is misleading on any non-trivial dataset. Users assume clicking
"Total↓" on the invoices list shows the highest-value invoices in the
system; instead it shows the highest-value of the visible page.

P6 — add `sort` and `dir` to the request body on every list endpoint that
returns paginated results. Suggested contract:

```json
{
  "page_size": 10,
  "page_number": 1,
  "query": "",
  "sort": "total",
  "dir": "desc"
}
```

Allowed sort fields per endpoint (proposal — push back if any are expensive):

| Endpoint | Allowed sort fields |
|---|---|
| `/bill/all`, `/bill/purchase-bill/all` | `sequence_number`, `effective_date`, `total`, `state` |
| `/supplier/all`, `/client/all` | `name`, `id`, `created_at` |
| `/product/all` | `id`, `part_id`, `part_name`, `price`, `quantity` |
| `/order/all` | `sequence_number`, `effective_date`, `total`, `state` |
| `/cash_voucher/all` | `voucher_number`, `effective_date`, `amount`, `state` |
| `/branch/all`, `/stores/all` | `name`, `id` |

`dir` is `asc` or `desc`. Default `desc` for date/numeric, `asc` for text.

Unknown sort field → BE returns 400 with a recognisable shape (so I can
swallow it and fall back to default). Unknown direction → fall back to
default.

## What I'll do once you ship

When P0 + P1 + P2 + P5 land:
- delete `helpers.MatchSearchQuery` calls from all 9 handlers (keep the
  helper for the parts autocomplete edge case until you ship search there
  too)
- delete the FE `state`/`stock` post-filters
- ship a thin "Big Bang" PR that flips everything BE-driven in one go

When P6 lands:
- replace the JS table sorter with `<th>` links that bake `sort` + `dir`
  into the URL and the form, exactly the same way the FE already handles
  `q` and `state`
- delete the JS sort code

Until then: search and filter results are correct (FE is masking gaps);
sort is broken on any paginated dataset and I cannot fix it FE-side.

## Open question for you

Do you want a normalization spec doc extracted from `helpers/search_match.go`
so your LIKE/FULLTEXT clause matches what the FE matcher does today? (Arabic
harakat strip, alef/ya/ta-marbuta folding, indic→ASCII digit fold,
tatweel removal, whitespace collapse.) If yes I'll cut a doc and you can
copy-paste the regex/SQL.

Status: `question`. Tell me which P-items are in scope for this round and I
will plan the FE rip-out accordingly.
