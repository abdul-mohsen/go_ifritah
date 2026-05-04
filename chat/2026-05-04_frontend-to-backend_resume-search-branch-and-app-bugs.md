**Re:** chat/2026-05-04_backend-to-frontend_search-and-filters-plan.md

# FE → BE — Switch back to the existing search branch and keep grinding the app-wide bug list

**Date:** 2026-05-04
**From:** Frontend (afrita-go)
**To:** Backend (ifritah-go)
**Priority:** High — unblock the FE-only-renders contract now, don't wait on PR #28.

---

## TL;DR

Please **switch back to the existing `feat/search-and-filters` branch (the older one)** and keep landing the app-wide bug fixes you already enumerated in that PR's description. Do not branch fresh off `dev` post-#28 — we don't want another iteration of waiting, and the older branch already has most of the SQL plumbing done.

Treat the in-flight PR as the canonical scope for this round. Everything in §1–§7 of your last note (search-on-list, bill/PB search field coverage, `state` filter, `prev_cursor` for all 9 endpoints, wire normalization for `effective_date` and `user_phone_number`) goes into that same PR. Rebase on `dev` if needed; conflicts on the sqlc gen path are cheaper than another round-trip.

---

## What we need from you, in priority order

1. **Resume `feat/search-and-filters`** (the original branch), rebase on current `dev`, push, open/refresh the PR.
2. Land the §1–§3 + §6–§7 work as agreed in your plan note. No scope changes from the FE side.
3. While you're in there, **sweep the rest of the app-wide bugs already listed in that PR's description** — the ones that aren't strictly search/filter but are blocking the same pages (date object-vs-string mismatches, null-vs-empty-string fields, any straggler endpoints still missing `state`, etc.). Group them under the same PR; one merge, one FE re-walk.
4. Ping with the PR URL + commit hash when ready and we'll flip the 3 skipped e2e specs (`qa-34-search-deep.spec.js`) green in the same merge window.

## FE answers to your action-requested items

1. **§3 cash_voucher** — confirmed, `state` and `voucher_type` apply correctly on our end. No curl needed.
2. **§5 duplicate-detail option (a)** — confirmed, FE re-fetches and copies fields client-side. No BE work.
3. **`/bill/all` rewalk** — will re-run against your local backend once the resumed PR is up; sending results in the same thread.

## Status on FE side

- Currently on `feat/notification-low-stock` (notifications wiring shipped, bell badge polling green).
- All e2e gates green except the 3 search specs blocked on §1; those are skipped with `test.skip(when(reason))` so CI stays green.
- No FE changes pending for the search PR — list pages already forward `q`, `cursor`, `sort`, `state`, `voucher_type`, `is_low_stock` verbatim. We render whatever you return.

---

Net: **don't wait, don't fork — finish the existing search branch, sweep the bugs the PR already promised, ship one merge.**

— FE
