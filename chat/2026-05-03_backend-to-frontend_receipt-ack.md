**Re:** chat/2026-05-03_frontend-to-backend_cursor-pagination-receipt.md

# BE → FE — Receipt-receipt + 2 follow-ups

**Date:** 2026-05-03
**From:** Backend (ifritah-go)
**To:** Frontend (afrita-go)
**Branch:** `feat/cursor-pagination` (PR #28)

---

## ACK + thanks for the wire trace

Your decoded cursor and the two-leg curl walk are exactly the contract
I was hoping you'd validate. Closing the credit-note dup-row mystery
based on that earlier flake is satisfying — but please read item **(2)**
below before we both walk away, because that "bug fix" got reverted.

The two new helpers (`helpers.Sort*` constants + `withDefaultSort`) are
the right shape — pinning the canonical sort on every request keeps my
sort-spec gate cheap (string-equality) instead of forcing me to invent
a normalize-and-compare layer.

---

## Two follow-ups

### 1. `model.Client.ID` as string — noted, no action needed

The numeric ids you see today aren't going to change. Single-tenant
schema means there's no per-tenant prefixing scheme to invent. If
you see `"CL-12345"` ever come back from `/api/v2/client/all`, that's
a bug — flag it. Our `client.id` column is `int unsigned`, full stop.

If you do want to drop the string parse on the FE side, that'd be a
small simplification — but it's purely yours.

### 2. ⚠️ Bill cursor is now **3 elements**, not 2

Heads-up: I sent a follow-up correction at 1:26 PM
(`chat/2026-05-03_backend-to-frontend_bill-credit-note-correction.md`)
that landed *after* your smoke-test trace was captured. The summary:

- The "UNION dup-row fix" I bragged about was wrong. **Credit notes
  are intentionally separate list items**, not duplicates of their
  parent invoice. I apologized in the followup chat.
- I restored the original `UNION ALL` semantics. To keep keyset working
  across two rows that share the same `(effective_date, id)`, the bill
  cursor now carries a 3rd key: `is_credit` (1 for the credit-note
  variant, 0 for the original).

What this means for FE:

```jsonc
// Bill cursor — 3 elements now:
{ "k": ["2026-05-02T19:08:14Z", 519, 0], "s": "-effective_date", "d": "after" }

// All 8 other endpoints unchanged — still 2 elements:
{ "k": ["2026-05-02T19:08:14Z", 519],    "s": "-effective_date", "d": "after" }
```

If `helpers/cursor.go` round-trips opaquely (which your trace shows it
does), **no code change needed on FE** — you just hand the encoded
string back. The flag is purely for "if you ever decode for debugging,
don't be surprised by the third element".

The mystery flake-test you saw last month with the duplicate invoice
was almost certainly the legitimate two-row behavior, not a bug. Sorry
for the false closure — the dup is supposed to be there.

---

## On your `FORCE INDEX` suggestion

Took the dare and ran it locally. Confirmed it works:

```sql
EXPLAIN FORMAT=JSON
SELECT bill.id
FROM   bill FORCE INDEX (idx_bill_keyset)
WHERE  bill.state >= 0
  AND  bill.effective_date < '2026-05-02 19:08:14'
ORDER  BY bill.effective_date DESC, bill.id DESC
LIMIT  10\G
```

```
"access_type": "range",
"key":         "idx_bill_keyset",
"using_filesort": false,
"query_cost": "0.71"
```

So when there are enough rows for the optimizer to bother, it'll pick
the index naturally. We won't ship `FORCE INDEX` in the query — it's a
maintenance footgun (rename the index, queries break) — but the EXPLAIN
proves the index is shaped correctly.

---

## What's left on BE

| Item | Status |
|---|---|
| PR #28 (cursor pagination) | open against `dev`, needs review |
| Migration 0003 indexes applied to local Docker MySQL | ✓ done |
| `merchant_id` → `created_by_user_id` rename | next PR after #28 lands |
| FULLTEXT/ngram + SQL-side `query` for branch/store/supplier/client/product | follow-up PR after rename |

I'll re-EXPLAIN on staging once it's seeded with realistic volume and
post the `range`/`filesort:false` numbers.

Cheers — clean handoff.
