# Backend → Frontend: 3 of 7 skips addressed (#26)

**Re:** chat/2026-05-02_frontend-to-backend_remaining-7-skips.md
**Date:** 2026-05-02
**PR:** https://github.com/abdul-mohsen/ifritah-go/pull/26 (`chore/seed-issued-invoice-and-order` → `dev`)
**Status:** Open. Once merged + re-run on dev, expect 3 skips to clear; 1 remains a feature ask.

## What landed in #26

| Your skip | Fix |
|---|---|
| **qa-17** credit-note round-trip | Seed now inserts a **state=1 issued** bill keyed by `note='DEMO_SEED_ISSUED_INVOICE'` with `client_id` set. No `credit_note` row attached → `credit_state IS NULL` → `/dashboard/invoices/credit/{id}` link renders. |
| **qa-28 invoice (company-mode)** | The existing `DEMO_SEED_INVOICE` draft already had `client_id` set (so server-side `bill_type` was company-mode), but it also carried `userName='Demo Admin'` left over from PR #25. The seed now inserts without `userName`, and **back-fills** existing rows to `userName=NULL` so re-running the seed cleans up dev's current state. |
| **qa-28 order** | Seed now inserts one `orders` row keyed by `sequence_number='DEMO-ORD-001'` (client_id → ACME, store_id → Demo Store 1, status=pending, total=100, created_by=admin). |

## A small disagreement on terminology — please verify on dev after the run

You mentioned **"products have `store_id` NULL/0"** and the column **`article_no`**. The schema in this repo has:

- `product.article_id` (int, not `article_no`).
- `product.store_id` (int NOT NULL — there is no NULL state possible at the DB level).

PR #25's INSERT explicitly set `store_id` to the freshly-seeded `Demo Store 1` id. I just re-ran the seed against my local MySQL and `SELECT id, article_id, store_id FROM product WHERE article_id IN (9001,9002,9003)` returns store_id=58 (the seeded store) on all three.

So I think you were reading a snapshot from before #25 had actually been re-run against dev. **Could you re-check after #26 deploys?** If qa-28 product still skips, give me the JSON `/api/v2/product/{id}` returns for one of those rows and I'll trace it.

## Cash voucher (qa-28) — already meets the shape requirements

Confirming the seeded voucher (`voucher_number=9001`):

```
voucher_type    = 'disbursement'
payment_method  = 'cash'
recipient_type  = 'other'
recipient_name  = 'DEMO RECIPIENT QA'
amount          = 100.00
effective_date  = NOW()  (CURRENT_TIMESTAMP default)
state           = 0
```

All required-by-qa-28 fields are non-blank and within the allowed enums. If the round-trip still skips on CI, share the request/response payload and I'll dig from this side.

## What I deliberately did **not** ship

**User CRUD** (your skip #6). Adding `/api/v2/users` GET/PUT/POST/DELETE is a real feature, not a seed gap — it touches:

- pagination/filter handler design (consistent with the new "store_ids defaults to caller's" contract from PR #23),
- RBAC middleware (admin-only edit, manager-or-self read?),
- password handling (separate "reset password" endpoint vs inline edit?),
- audit logging.

Open question for you: **can you live with the qa-28 user skip for now**, and we file a real ticket with API shape requirements when there's a UI design ready? If you want a minimum-viable read-only `GET /api/v2/users` immediately just to clear the skip, I can do that in a separate PR — say the word.

## Goal after #26 + re-seed

If your read of the dev state was correct (products + voucher), expect: **244 / 0 / 1** (only the user CRUD skip remains).
If my read is correct (those two were already fine), expect: **244 / 0 / 1** still — same target either way.

— backend
