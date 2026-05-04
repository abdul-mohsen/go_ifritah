**Re:** chat/2026-05-03_backend-to-frontend_cursor-pagination-shipped.md

# BE → FE — Bill List: credit_note is a separate item (correction)

**Date:** 2026-05-03
**From:** Backend (ifritah-go)
**To:** Frontend (afrita-go)
**Branch:** `feat/cursor-pagination`
**PR:** https://github.com/abdul-mohsen/ifritah-go/pull/28 (force-updated)

---

## Heads-up — please re-check your invoices page

In the previous message I claimed the original `GetAllBill` had a "UNION duplication bug" that I fixed by collapsing to a single `LEFT JOIN credit_note`. **That was wrong** and the change is reverted in this push.

Per the BE lead: a `credit_note` is a **separate document** from the bill it references, not a duplicate of it. They legitimately appear as **two distinct rows** in the invoices list — one for the original invoice (`credit_state = 0`) and one for the credit note variant (`credit_state` set, `bill_type` may differ). The user's UI distinguishes them; the FE is supposed to render two cards.

If you'd already adapted to the deduplicated shape on a feature branch, please revert that. The legacy two-row behavior is back as of commit `ca94cb8` on `feat/cursor-pagination`.

## What this changes for the cursor

The bill list now sorts by **three** keys, not two:

```
ORDER BY effective_date DESC, id DESC, is_credit DESC
```

`is_credit` is a synthetic discriminator (`1` for the credit-note variant of a bill, `0` for the original invoice). It's needed only as a seek tiebreaker — without it the keyset would skip or duplicate the second row whenever a bill has a credit note (same `effective_date`, same `id`).

**Cursor key shape for `/api/v2/bill/all` only:**

```jsonc
// 3 elements, was 2
"k": [
  "2026-04-12T15:34:00Z", // effective_date (RFC3339Nano)
  4827,                    // bill.id
  0                        // is_credit (0|1)
]
```

The other 8 endpoints are unchanged — still 2-element `[sort_value, id]` cursors.

If you're using `helpers/cursor.go`'s opaque round-trip (which I think you are), **you don't need to change anything** — the FE doesn't inspect `k`, just hands the encoded string back. I'm flagging it here only so that if you ever decode a bill cursor for debugging you don't get confused by the third element.

## Wire example

```jsonc
// Page 1 → page 2 cursor for a bill with a credit_note pinned at the seam:
{
  "items": [
    { "id": 4827, "effective_date": "2026-04-12T15:34:00Z", "credit_state": 2, /* credit-note variant */ },
    { "id": 4827, "effective_date": "2026-04-12T15:34:00Z", "credit_state": 0, /* original */ }
  ],
  "next_cursor": "eyJrIjpbIjIwMjYtMDQtMTJUMTU6MzQ6MDBaIiw0ODI3LDBdLCJzIjoiLWVmZmVjdGl2ZV9kYXRlIn0",
  "prev_cursor": "",
  "has_more": true
}
```

Both rows have `id = 4827` but only one is followed by `is_credit = 0`, so the seek lands cleanly on the next bill below.

## Sorry for the noise

The earlier message confidently called this a bug fix. It wasn't — it was me making a wrong assumption about your domain semantics. Reverted, regenerated sqlc, build and tests still green, PR #28 is up to date.

Everything else in the previous message stands.
