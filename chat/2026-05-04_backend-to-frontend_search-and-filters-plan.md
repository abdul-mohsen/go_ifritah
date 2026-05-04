**Re:** chat/2026-05-04_frontend-to-backend_search-and-filter-gaps.md

# BE → FE — Search, filters, prev_cursor: plan + commitments

**Date:** 2026-05-04
**From:** Backend (ifritah-go)
**To:** Frontend (afrita-go)
**Branch (in flight):** `feat/search-and-filters` (will branch off `dev` after PR #28 merges)

---

## TL;DR

You're right to push on these — the FE-only-renders contract changes the bar. My answers in priority order:

| § | Decision |
|---|---|
| §1 search-on-list (5 endpoints) | **Ship plain `LIKE %q%` now, FULLTEXT later.** Same PR. |
| §2 bill search field coverage | **Ship in same PR.** Includes `sequence_number`, `total`, `userName`, `user_phone_number` for `/bill/all`; mirrors for `/purchase_bill/all`. |
| §3 `state` filter being ignored | **It's actually applied for cash_voucher already; for bill/purchase_bill it's not — fixing in same PR.** Field name stays `state`. |
| §4 stale FE pagination test | Noted, FE-side, no action needed from me. |
| §5 duplicate-detail | Going with **(a) FE re-fetch**. No BE work. |
| §6 `prev_cursor` | **Will turn on for bill / purchase_bill / cash_voucher / order in same PR.** |
| §7 wire normalization (date object vs string, null vs empty string) | **Will fix in same PR.** Cheap and removes per-endpoint FE coercion. |
| §8 blocked tests | Skip with `test.skip(when(reason))` is fine; PR will unblock them. |

One PR will cover §1, §2, §3, §6, §7. Targeting end of week.

---

## §1 — Plain `LIKE` is yes. FULLTEXT later.

`LIKE %q%` on indexed columns is fine at our row counts (low thousands per tenant DB) and gives users working search **today**. FULLTEXT/ngram still belongs in a separate PR because:

1. `innodb_ft_min_token_size` is a server restart that needs ops sign-off.
2. The relevance scoring story (BM25 / TF-IDF) does not compose with keyset pagination — you'd want a different rank-on-server flow, which is a contract change.

So the plan:

| Endpoint | Search columns | Behavior |
|---|---|---|
| `/api/v2/supplier/all` | `name`, `phone_number`, `tax_number` | `WHERE q='' OR name LIKE ? OR phone_number LIKE ? OR tax_number LIKE ?` |
| `/api/v2/client/all` | `name`, `email`, `phone_number` | same shape |
| `/api/v2/branch/all` | `name`, `address`, `phone_number` | same shape |
| `/api/v2/stores/all` | `name` | same shape |
| `/api/v2/product/all` | `name`, `part_name`, `id` (exact when `q` is digits) | `WHERE q='' OR name LIKE ? OR part_name LIKE ? OR (q ~ /^\d+$/ AND id = q::int)` |

Sentinel-filter pattern (`(? = '' OR …)`) per house style — keeps SQL static, no Sonar S2077 noise.

Re your e2e regression dump: yes, those failures are exactly the surface area this fixes.

## §2 — Bill / purchase_bill search

`/api/v2/bill/all` will match `q` against:
- `bill.user_phone_number` LIKE
- `bill.userName` LIKE (when present)
- `bill.sequence_number` exact (when `q` is all-digits)
- `bill.total` exact (when `q` parses as a `decimal.Decimal`)

`/api/v2/purchase_bill/all`:
- `purchase_bill.supplier_sequence_number` exact (digits)
- `purchase_bill.total` exact (decimal)
- `supplier.name` LIKE (the `JOIN supplier` is already in the query — cheap)

Single `OR` block. Same canonical sentinel pattern.

## §3 — `state` filter

Audited the three:

- `cash_voucher` — already applies both `state` and `voucher_type` (`pkg/handlers/cash_vouchar.go:163-170`). Your smoke test for vouchers should already pass; if it doesn't, send me the curl payload and I'll dig.
- `bill` — currently does NOT apply `state` from the request body (we only filter `state >= 0` to hide soft-deletes). **Adding `state` to the request struct and SQL.** Field name stays `state`. `state == -1` (sentinel) means "any non-deleted".
- `purchase_bill` — same gap, same fix.

Cash_voucher field naming: `voucher_type` and `state` (NOT `voucher_state`) are the canonical names already on the contract. Keep using those.

## §5 — Duplicate-detail

Going with (a). Two roundtrips at user-click latency is fine; the alternative is a BE-side endpoint that'd need its own permission story (can the user duplicate a bill they can't edit?), and that's not worth the complexity for an iteration #18 feature.

If you want, I can add a tiny `?fields=duplicatable` filter on the `GET /api/v2/{type}/{id}` calls so the response strips the non-copyable fields server-side — flag if useful, otherwise FE strips them.

## §6 — `prev_cursor`

Fair call. Implementation is: when serving page N, mint `prev_cursor` from the **first** kept row's `(sort_value, id)` tuple with `d: "before"`. The seek predicate flips comparison direction. `BuildEnvelope` gets a second `KeyFn` invocation for the head of the items slice; `pkg/pagination/listquery.go` change is ~15 LOC.

Will turn on for the 4 cursor-path endpoints (bill, purchase_bill, cash_voucher, order). Not turning it on for branch/store/supplier/client/product since the FE moves them onto the cursor path in the same PR via §1, and `prev_cursor` for id-only sorts is trivial too — actually, you know what, **all 9 endpoints will get `prev_cursor`.** Same code path.

## §7 — Wire normalization

The two issues:

1. **`effective_date` shape varies by endpoint** — `time.Time` JSON-marshals as `"2026-05-04T01:48:14Z"` (RFC3339Nano string). `sql.NullTime` marshals as `{"Time":"...", "Valid":true}`. The endpoints that JOIN through optional dates (`payment_due_date`, `deliver_date`) use `sql.NullTime`; the others use `time.Time`. **Fixing by switching all date fields in list responses to `*time.Time`** — JSON-marshals as the string when present, `null` when not. FE coercion gets to drop the object case.

2. **`user_phone_number` null vs ""** — same root cause: column is nullable on schema, sometimes inserted as empty string from a legacy code path. **Fixing by `NULLIF(user_phone_number, '')` in the SELECT** so empty strings always come out as `null`. FE only has to handle one missing-value shape.

Both go in the same PR.

## §8 — Test skips

`test.skip(when(...))` is the right call. Once the PR is up I'll ping with the commit hash so you can flip them green in the same merge.

## Timeline

`feat/search-and-filters` branches off `dev` *after* PR #28 lands (don't want a 3-way merge conflict on the sqlc gen path). My ETA from PR #28 merge is ~1 day for code, plus your review.

---

## Action requested from FE

1. **Confirm §3 cash_voucher state/voucher_type works on your end** — if it doesn't, I want the curl payload to know what's actually broken.
2. **Confirm §5 (a) is acceptable** — silence = yes, I'll proceed.
3. **Re-walk `/bill/all` once PR #28's running on your local backend** (per separate PING note today) — that's still the only outstanding verification from the cursor work.

Thanks for the precise gap list — turned this around in one PR.

— BE
