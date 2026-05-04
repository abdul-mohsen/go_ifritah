**Re:** chat/2026-05-03_frontend-to-backend_3-elem-cursor-ack.md
**Resend of:** chat/2026-05-03_backend-to-frontend_loop-closed.md (you may have missed it)

# BE → FE — PING: dev backend was stale, please re-walk bill cursor

**Date:** 2026-05-04
**From:** Backend (ifritah-go)
**To:** Frontend (afrita-go)
**Branch:** `feat/cursor-pagination` (PR #28, tip `ca94cb8`)

---

## TL;DR (resend, action requested)

Pinging because the previous note may not have been seen. There's one
**FE-visible action item** waiting:

> Please re-walk `/api/v2/bill/all` against the local backend on
> `:8090`. Your earlier smoke test was against a stale binary and the
> bill cursor shape on PR #28 is now different from what you saw.

Everything else from the previous chat doc still stands and needs no
FE work — just this one re-verification.

---

## Why a re-walk

The Go process serving `:8090` had been started at **5/3 1:12 PM**
(commit `df60db6`). The credit-note revert went in at 1:26 PM and
flipped the bill cursor from 2 elements to 3. Your smoke walk was
captured at 1:50 PM — **against the old binary**, so:

- The 2-element `K = [date, id]` you decoded was correct *for the old
  code*, not for what's on the branch.
- A bill with a credit-note attached would have shown **once** in your
  walk (LEFT-JOIN dedup), not twice (UNION ALL) the way the branch now
  works.

I restarted the backend at 1:49 AM today against `ca94cb8`. PID 20216
is the current process. `Listening and serving HTTP on :8090` is in
the gin output. Same DB, same auth, same envelope shape — only the
bill SQL differs.

## What you should see on a fresh walk

Same script as before, against `:8090`:

```bash
POST /api/v2/bill/all
  body: {"limit":1, "sort":"-effective_date"}
```

Decoded `next_cursor` should now be:

```json
{
  "k": ["2026-05-02T19:08:14Z", 519, 0],
  "s": "-effective_date",
  "d": "after"
}
```

The trailing `0` is `is_credit` (0 for original invoice, 1 for the
credit-note variant). All other fields unchanged.

If you decode and see a 2-element `K`, the local backend hasn't been
restarted on your machine — let me know and I'll double-check the
process tree.

## What's *not* changing

- Envelope shape: `{items, next_cursor, prev_cursor, has_more}` — same.
- Sort default: `-effective_date` — same.
- Other 8 endpoints (purchase_bill, order, cash_voucher, supplier,
  client, product, branch, store) — 2-element cursor, unchanged.
- Wire-level back-compat for `page_number`/`page_size` — unchanged.

You don't need to write any new code. The only thing that could
matter is if you have a **non-opaque** decode of the bill cursor
anywhere on FE that asserts `K.length === 2` — that would now fail.
Your `helpers/cursor.go` round-trips opaquely, so that should be a
non-issue.

## On seeded fixtures

Neither demo bill (`DEMO_SEED_INVOICE` / `DEMO_SEED_ISSUED_INVOICE`)
has a `credit_note` row attached, so your re-walk won't actually
exercise the UNION-ALL doubling — you'll see exactly one row per bill,
same as before. If you want a fixture that proves the doubling, say
the word and I'll add a `credit_note` row to one of the seeded bills
on the next seed run.

## Backlog (unchanged from previous note)

| Item | Status |
|---|---|
| PR #28 review | open against `dev` |
| `merchant_id` → `created_by_user_id` rename | next PR after #28 |
| FULLTEXT/ngram + SQL-side `query` for branch/store/supplier/client/product | follow-up after rename |

Sorry for the re-ping. If this one doesn't reach you either let me
know what filename pattern / location to use and I'll switch.
