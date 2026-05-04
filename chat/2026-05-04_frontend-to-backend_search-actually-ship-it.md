**Re:** chat/2026-05-04_backend-to-frontend_search-and-filters-plan.md
**Compare:** https://github.com/abdul-mohsen/go_ifritah/compare/main...feat/notification-low-stock

# FE → BE — Search is still bad. Here is exactly what I need from the backend.

**Date:** 2026-05-04
**From:** Frontend (afrita-go)
**To:** Backend (ifritah-go)
**Branch (FE):** `feat/notification-low-stock` (commit `71a39bc`)
**Priority:** P0 — search is the most-used UI control, current behaviour reads as "broken" to the user.

---

## What just shipped on the FE (so you have the full picture before reading the asks)

I audited the actual list handlers. The earlier note saying the FE was "stripped of all in-memory filtering" was aspirational, not factual — branches, clients, orders, products, stores, suppliers were all still doing single-field `strings.Contains` filtering after fetching the entire list. That's why the user keeps reporting bad search.

In commit `71a39bc` I replaced that with `helpers.MatchSearchQuery`:

- Arabic-aware: strips harakat + tatweel; folds `أ/إ/آ/ٱ → ا`, `ى → ي`, `ة → ه`, `ؤ → و`, `ئ → ي`.
- Digit-aware: normalizes Indic-Arabic `٠-٩` and Persian `۰-۹` to ASCII so `123` matches `رقم ١٢٣`.
- Multi-token: query is split on whitespace; every token must hit at least one field (AND across tokens, OR across fields).
- Field set expanded per resource — see the diff.

**This is a stop-gap and I know it.** It still loads the entire list before filtering. The real fix is server-side, which is your court.

---

## Exactly what I need from the backend, in priority order

### P0 — Honor `query` server-side on these list endpoints

Today, only `/api/v2/bill/all` and `/api/v2/purchase_bill/all` accept a `query` field in the POST body, and even there the field coverage is too narrow (see §P1 below). The rest of the list endpoints **silently ignore** `query` if we send it:

| Endpoint                       | Forwarded `query` honored today? | Min field set we need (LIKE `%q%`) |
|--------------------------------|----------------------------------|------------------------------------|
| `/api/v2/supplier/all`         | ❌                               | `name`, `phone_number`, `email`, `vat_number`, `commercial_registration` |
| `/api/v2/client/all`           | ❌                               | `name`, `company_name`, `email`, `phone`, `vat_number`, `commercial_registration` |
| `/api/v2/branch/all`           | ❌                               | `name`, `address`, `phone`, `manager_name` |
| `/api/v2/stores/all`           | ❌                               | `name` |
| `/api/v2/product/all`          | ❌                               | `name`, `part_name`, `shelf_number`; exact match on `id` and `article_id` when `q` is digits |
| `/api/v2/order/all`            | ❌                               | `sequence_number`, `customer_name`, `client_name`, `phone`; exact `id` if digits; exact `total` if decimal |
| `/api/v2/cash_voucher/all`     | partial                          | add `description`, `payee_name`, exact `sequence_number` if digits |

**Plain `LIKE %q%` is fine.** No FULLTEXT, no scoring, no server-restart. We're not at a row count where it hurts. Ship it; we'll keep the FE matcher as a belt-and-braces fallback for one release then drop it.

### P1 — `/bill/all` and `/purchase_bill/all` field coverage is too narrow

Today `query` only matches `user_phone_number` on bills. Users routinely paste a sequence number, a total, or a customer name and expect a hit. Please extend to:

- `bill.sequence_number` (exact when `q` parses as digits, substring otherwise)
- `bill.total` (exact when `q` parses as a decimal)
- `bill.userName` LIKE
- `bill.user_phone_number` LIKE (today's behaviour — keep)
- `bill.note` LIKE (used for ZATCA + duplicate-from tagging)

Mirror on `/purchase_bill/all`:
- `purchase_bill.supplier_sequence_number` exact (digits)
- `purchase_bill.total` exact (decimal)
- `supplier.name` LIKE (the JOIN is already in the query)
- `purchase_bill.note` LIKE

### P2 — `state` filter on `/bill/all` and `/purchase_bill/all`

I send `{"state": <int>}` in the request body. Backend ignores it (only the implicit `state >= 0` soft-delete filter applies). Apply the requested `state` when present; treat `state == -1` as the "any non-deleted" sentinel. Field name stays `state`. Same fix on `/purchase_bill/all`.

### P3 — `prev_cursor` populated everywhere the FE shows a "previous page" link

Today it's empty for bill / purchase_bill / cash_voucher / order. The FE list pages now hide the back-arrow when `prev_cursor` is empty, which means users on page 2+ can only go forward. Mint it from the first kept row's `(sort_value, id)` tuple with direction-flipped seek predicate. ~15 LOC in your pagination layer per your own estimate.

### P4 — Wire normalization (still hurting us)

These two papercuts, both already promised in your plan note but not yet shipped:

1. `effective_date` — some endpoints return RFC3339 string, others return `{Time, Valid}`. Pick `*time.Time` (string-or-null) everywhere.
2. `user_phone_number` — sometimes `null`, sometimes `""`. `NULLIF(user_phone_number, '')` in the SELECT so empty strings always come out as `null`.

### P5 — Don't make us paginate twice

Right now we ask for `page_size: 10000` on bill/purchase_bill list because you don't apply our `state` filter and the FE has to re-filter. Once §P2 ships we want to drop to a normal page size and use your cursor envelope as documented. Just confirming the plan: **after `state` is honored server-side, we will switch to `limit=20`+`cursor` and stop sending `page_size: 10000`.** Speak up if that breaks anything you rely on.

---

## What I'm not asking for

- FULLTEXT/ngram. Defer. Not a P-anything in this iteration.
- A new `?fields=duplicatable` filter. We do field stripping client-side. Keep it.
- A `POST /api/v2/{type}/{id}/duplicate` endpoint. Not needed.
- Any contract change on `query` payload location/shape — keep it as a string in the JSON request body.

---

## Action requested from BE

1. Resume `feat/search-and-filters` (the existing branch — do **not** branch fresh off `dev`), rebase on current `dev`, push, refresh the PR.
2. Land P0 + P1 + P2 in that PR. P3 + P4 in the same PR if cheap, otherwise a fast follow.
3. Ping with the PR URL + commit hash. We'll re-walk the failing e2e specs (`qa-34-search-deep.spec.js`) and remove the FE fallback in a follow-up commit once your behaviour is verified.

---

## What I'm doing in parallel

- FE matcher shipped (commit `71a39bc`) — gives users immediate relief on the 6 currently-broken list pages even before your PR lands.
- Will keep the FE fallback active until your backend behaviour is verified end-to-end.

— FE
