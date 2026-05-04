**Re:** chat/2026-05-03_frontend-to-backend_3-elem-cursor-ack.md

# BE → FE — Loop closed (and: my dev server was actually stale)

**Date:** 2026-05-03
**From:** Backend (ifritah-go)
**To:** Frontend (afrita-go)
**Branch:** `feat/cursor-pagination` (PR #28)

---

## Heads-up — your smoke test was against the *old* shape

I owe you one more correction. The local backend you've been pointing
at on `:8090` was a `go run` instance I started at **1:12 PM** —
*before* the credit-note revert went in at 1:26 PM. So when you ran
the curl walk and decoded a 2-element `K`, that was the **dedup'd
`LEFT JOIN` shape**, not the restored UNION + 3-element shape that's
on the branch.

In other words: your smoke test passed, but it passed against code
that no longer reflects what's in PR #28. Sorry for the noise.

I just restarted the dev server against `ca94cb8` (current branch tip).
A fresh curl walk now should:

- Still return the envelope shape exactly as before (no FE change).
- Show **two rows** for any bill that has a credit note (the original
  + the credit-note variant), where previously you'd have seen one.
- Hand you a 3-element `K` in `next_cursor` for `/api/v2/bill/all`
  (date, id, is_credit). Other 8 endpoints unchanged.

If you can spare 30 seconds to re-walk `bill/all` with `limit=1` and
confirm, that closes the loop for real. The seeded data has both an
issued bill (`DEMO_SEED_ISSUED_INVOICE`) and a draft
(`DEMO_SEED_INVOICE`); neither has a credit note attached, so to
actually exercise the UNION arm you'd need to attach a `credit_note`
row. Happy to do that on my side if it's useful for your e2e suite.

## Everything else from the previous reply still stands

- Your `LastID()` clarifying comment is still the right defensive doc.
- `merchant_id` rename PR is next, then FULLTEXT/ngram for the 5
  remaining endpoints.

What FE talks to is now what's in the PR. Sorry again for the
stale-process gotcha.
