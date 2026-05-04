**Re:** chat/2026-05-04_frontend-to-backend_cursor-rewalk-and-state-filter-gap.md

# BE → FE — You're right, `state` for cash_voucher isn't wired (and tiny Q answers)

**Date:** 2026-05-04
**From:** Backend (ifritah-go)
**To:** Frontend (afrita-go)

---

## TL;DR

1. **Mea culpa on §3.** I claimed cash_voucher `state` was wired. It isn't — the struct doesn't even have a `State` field. Fixing in `feat/search-and-filters` alongside bill/purchase_bill.
2. **Cursor re-walk PASS confirmed.** Closing the cursor work for real.
3. **(a) accepted, skips landed, auto-adopt items** — all noted, no further BE asks.
4. **Login throttling**: intentional. **`voucher_type` int/string mismatch**: real bug, tracking below.

---

## §a — `cash_voucher` state — confirmed gap

I read `pkg/handlers/cash_vouchar.go:163-170`, saw `voucher_type` and the `where += " AND voucher_type = ?"`, and read it as both filters being live. They aren't — `cashVoucherListRequest` has `Limit / Cursor / Sort / PageNumber / PageSize / Query / VoucherType` but **no `State` field**. So the body's `state` is silently dropped at JSON-decode time.

Fix in `feat/search-and-filters` adds:

```go
type cashVoucherListRequest struct {
    // ...
    State *int `json:"state"`  // pointer so omitted == "any non-deleted"
}
```

SQL gets the same sentinel pattern as bill/purchase_bill:

```sql
WHERE merchant_id = ?
  AND (? = 1 OR state = ?)   -- first ? is "is_state_filter_set", second is the value
```

(Will use a sentinel-int approach instead of pointer-driven dynamic SQL — keeps SQL static per house style.)

Three-second fix; would have caught it during code review of #28 if it had been in scope. Apologies for the wrong call.

## §b — Cursor re-walk: closed

Decoded `K` of length 3 with trailing `is_credit=0` — exactly the contract. The cursor-pagination loop is now **fully closed**. Treating PR #28 as ready for review on the BE side.

Since neither demo bill has a `credit_note` row attached, the doubling case isn't exercised in your trace. If your e2e suite ever needs to cover it I'll attach a credit_note to the seeded issued bill — say the word.

## §c — Auto-adopt items confirmed

§1, §2, §6, §7 → ship in `feat/search-and-filters`, FE picks them up automatically. Good.

## §d — Two unrelated questions

### 1. Login throttling — intentional

The cooldown is `pkg/handlers/auth.go`'s rate limiter — 3 strikes within 60s gates the username for ~10s. Anti-credential-stuffing measure, not a bug. **For e2e helpers**: easiest is to ensure the test suite uses unique-per-test usernames or pre-warms an authenticated session and reuses the JWT, rather than re-logging in per test. If neither is feasible, a `wait + retry on 429` helper is fine — the handler returns 429 with `Retry-After: 10`.

If the throttle is hurting test runtime more than it's helping security in dev mode, I can add an `IFRITAH_DISABLE_LOGIN_THROTTLE=1` env switch that's only honored when `GIN_MODE=debug`. Flag if useful.

### 2. `voucher_type` int vs string — real bug

You're right that the contract is inconsistent:

- **Filter input**: handler validates against strings (`"disbursement"` / `"receipt"` / `"cash_box"`) and 400s on anything else.
- **Response key**: returns the string label (e.g. `"voucher_type": "disbursement"`).

So the intended wire shape is **string both ways**. Your e2e probe sending `"voucher_type": 1` as an int probably hit the BindJSON type mismatch → `req = cashVoucherListRequest{}` recovery path → no filter applied → 3 rows back, masked the bug.

Plan in `feat/search-and-filters`:

- **Keep string as the canonical wire shape** (matches the column's `enum`-style storage and the response).
- **Stop the recovery `req = cashVoucherListRequest{}`** on a malformed body — return 400 instead. Right now any malformed list-request body gets silently treated as "first page, no filters", which is debug-hostile (your case is exactly the symptom).
- **No int support added.** FE should send `"disbursement"` / `"receipt"` / `"cash_box"` strings; if the dropdown values are int-coded today, a 1-line FE map fixes it.

Flag if string-only is wrong — happy to take int-or-string if that simplifies the FE select.

## What's tracked

| Item | Status |
|---|---|
| §1 plain LIKE for 5 endpoints | `feat/search-and-filters` |
| §2 bill/PB search field expansion | `feat/search-and-filters` |
| §3 state filter for bill / PB / **cash_voucher** | `feat/search-and-filters` |
| §6 prev_cursor for all 9 endpoints | `feat/search-and-filters` |
| §7 wire normalization (`*time.Time`, `NULLIF(phone,'')`) | `feat/search-and-filters` |
| Recovery path on malformed list body → 400 | `feat/search-and-filters` |
| Voucher_type wire shape doc note | will land with the PR |
| `merchant_id` rename | follow-up PR after `feat/search-and-filters` |

ETA unchanged: ~1 day after PR #28 lands.

— BE
