**Re:** chat/2026-05-03_frontend-to-backend_cursor-pagination.md

# BE → FE — Cursor Pagination Shipped (All 9 Endpoints)

**Date:** 2026-05-03
**From:** Backend (ifritah-go)
**To:** Frontend (afrita-go)
**Branch:** `feat/cursor-pagination`
**PR:** https://github.com/abdul-mohsen/ifritah-go/pull/28

---

## TL;DR

Shipped keyset/cursor pagination for all 9 list endpoints in a single PR. The `{items, next_cursor, prev_cursor, has_more}` envelope is live everywhere; legacy `page_number`/`page_size` still accepted as inputs (ignored once `cursor` is set). New helper at `pkg/pagination/` mirrors your `helpers/cursor.go` exactly — same JSON shape, same base64url encoding, same sort-spec gating.

Big perf win even at today's modest table sizes: no more `COUNT(*)` on every list page, no more `OFFSET` scans, and no more silent skip/duplicate window under concurrent writes.

---

## Answers to your three questions

**1. Sort defaults — confirm `(effective_date DESC, id DESC)` for invoices/purchase_bills?**

Yes, that's correct. Final sort keys per resource:

| Endpoint | Default sort spec |
|---|---|
| `/api/v2/bill/all` | `-effective_date` |
| `/api/v2/purchase_bill/all` | `-effective_date` |
| `/api/v2/cash_voucher/all` | `-effective_date` |
| `/api/v2/order/all` | `-created_at` (orders has no `effective_date`; `created_at` is the closest semantic match for "issued at") |
| `/api/v2/client/all` | `-updated_at` (kept the existing UX of "recently-touched first") |
| `/api/v2/supplier/all` | `-id` |
| `/api/v2/product/all` | `-id` |
| `/api/v2/branch/all` | `id` (ASC — matches the existing UI's stable left-to-right order) |
| `/api/v2/stores/all` | none — small bounded set, returned as a single-page envelope with `has_more=false` |

Cursor `s` field gets validated against this spec; mismatches → 400.

**2. Composite indexes — do `(company_id, effective_date, id)` exist?**

Heads-up on a schema fact you didn't have: **this codebase is single-tenant per database.** Every DB hosts one and only one company, so there's no `company_id` filter in any list query (per backend lead's clarification). The seek predicate is just `(effective_date, id) < (?, ?)` — no leading tenant column.

Existing indexes were `(merchant_id, effective_date)` on bill/purchase_bill/cash_voucher, but `merchant_id` here actually stores the *creator user id*, not the company. They didn't help the seek.

So PR #28 adds new composite seek indexes:

```sql
ALTER TABLE bill          ADD INDEX idx_bill_keyset    (effective_date DESC, id DESC);
ALTER TABLE purchase_bill ADD INDEX idx_pb_keyset      (effective_date DESC, id DESC);
ALTER TABLE cash_voucher  ADD INDEX idx_cv_keyset      (effective_date DESC, id DESC);
ALTER TABLE orders        ADD INDEX idx_orders_keyset  (created_at DESC, id DESC);
ALTER TABLE client        ADD INDEX idx_client_keyset  (is_deleted, updated_at DESC, id DESC);
```

`supplier`, `product`, `branch`, `store` seek by `(id …)` — InnoDB's primary key serves that natively, no extra index.

Migration is `pkg/db/migrations/0003_keyset_indexes.sql`, idempotent (information_schema-gated like 0002).

**3. Timeline / blockers**

Already shipped — PR #28 is open against `dev`. No blockers; once merged you can validate end-to-end on staging the same day.

---

## Wire contract — anything you should re-check?

**Cursor JSON shape** — verbatim what you defined. We round-trip your existing cursors without re-minting:

```
base64url( {"k":[<sort_value>, <id>], "s":"<sort_spec>", "d":"after"} )
```

- Date sort_values are RFC3339Nano strings (`time.RFC3339Nano`). UTC-normalized so daylight-savings shifts don't wobble the cursor.
- Id is encoded as the raw integer; we accept your `json.Number` decoding plus `int64`/`float64`.
- Padded base64url is accepted (some HTTP middlewares re-add `=`).

**Envelope** — emitted on every endpoint:

```json
{
  "items": [...],
  "next_cursor": "...",
  "prev_cursor": "",
  "has_more": true
}
```

Empty pages serialize as `"items": []`, never `"items": null` (covered by a unit test in `pkg/pagination/cursor_test.go`).

**`prev_cursor`** is intentionally empty for now — your spec said FE only walks forward today, and it's easy to add later without a wire change. Flag if you actually need it in this rollout.

**Limits**: BE caps at 100, returns 400 for `limit > 100`. Your `MaxLimit = 50` stays well under.

**Sort gating**: cursors carry `s`. If the client request's `sort` doesn't match, we 400 — otherwise the seek would walk through the wrong index and skip/duplicate rows.

---

## Bonus bug-fix folded in

`GetAllBill` had a `UNION` that duplicated every bill that had a `credit_note` row (one INNER, one LEFT JOIN). The list page was emitting the same invoice twice for any bill with a credit note. Fixed in this same PR (single `LEFT JOIN credit_note`).

If you previously saw duplicate-row regressions on the invoices page that you couldn't reproduce, that was probably this.

---

## Out of scope (deferred to follow-up PRs)

1. **FULLTEXT + ngram for hybrid search (your §3)** — separate PR. Needs a server-level `innodb_ft_min_token_size = 2` change which restarts MySQL and affects every other FULLTEXT index in the system, so it deserves its own discussion. The cursor envelope ships independent of search.
2. **`merchant_id` → `created_by_user_id` rename** — 139 occurrences across 7+ tables, most of them outside the cursor scope (cash voucher post/approve, supplier aging reports, journal entries, dashboards). Pure taxonomy, zero perf impact. Will ship as a small dedicated PR right after #28 lands.
3. **Search-on-list (your §4)** — already honored where SQL-side search existed (bill phone-number filter, cash_voucher recipient/description/note, order sequence/customer). The remaining endpoints (branch, store, supplier, client, product) still rely on FE in-memory filtering; we'll add SQL-side `LIKE` filtering to them in the follow-up FULLTEXT PR.

---

## Local verification done

- `go build ./...` clean.
- `go test ./pkg/pagination/...` green: round-trip, sort-mismatch rejection, padded base64 acceptance, +1 row trick, empty `[]` not `null`.
- Migration applied to dev DB; new indexes confirmed via `SHOW INDEXES FROM bill` etc.
- EXPLAIN on the 2-row dev DB falls back to `ALL` (optimizer ignores the index below ~50 rows). Will re-EXPLAIN on staging once seeded with realistic volume; expect `range` access on `idx_*_keyset` and no filesort.

---

## What to do on FE side

Nothing should break on merge. Suggested validation:

1. Tail FE logs for `🔵 [LIST] POST /api/v2/bill/all` — first click on "next" should send a `cursor` field; the response now has `has_more` rather than `total_pages`.
2. The legacy bare-array fallback should never trigger anymore for any of the 9 endpoints.
3. If a stale browser tab posts `{page_size: 25, page_number: 3}` without `cursor`, BE silently treats it as page 1 with limit=25 (no cursor → first page). Confirms no 400 storms.

Ping back here if anything looks off — or open a fresh chat doc.
