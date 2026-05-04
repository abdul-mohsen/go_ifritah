# FE → BE: live search shipped, what we still need from you

**Date:** 2026-05-04
**FE branch:** `feat/notification-low-stock` (commits `71a39bc..5af7011`)
**Compare:** https://github.com/abdul-mohsen/go_ifritah/compare/main...feat/notification-low-stock

## What changed on FE since the last note

| Commit | What |
|---|---|
| `5af7011` | Auto-submit on every `<select>` change inside list forms (state, stock, role, voucher type, per-page). Resets `page=0` on every change. |
| `0ae85fa` | Live debounced search on `input[name="q"]` (400ms after last keystroke; ESC clears + submits empty). Resets `page=0`. |
| `f3d87a5` | Notifications now deep-link to the resource (product/bill/order/cash voucher/client/supplier) — no more "click → mark read → re-find the thing". |
| `b140642` | Fixed silent bug: orders list `sequence_number` / `phone` / `status` arrived as JSON numbers, the FE assumed strings → matcher rejected them. Routed every field through `coerceString`. |
| `4491947`, `4eb7e2a` | Invoice/PB product autocomplete (`/api/products/search`) now Arabic-aware (harakat, alef, ta-marbuta, indic digits). |
| `71a39bc..d901d7c..0cfed4b` | New `helpers.MatchSearchQuery` + `normalizeSearchText` Arabic-aware matcher. Wired into branches, clients, products, suppliers, stores, orders, cash vouchers, users — replaces every `ContainsInsensitive` on a list page. 23 new test cases prove harakat / alef / ta-marbuta / indic digits / multi-token AND. |

Net effect: a user who types "إطار خارجي" in the parts search now matches a product named "اطار خارجي ٩٠٠٤" with no harakat in the DB. Same query in any list. Same filters auto-apply on dropdown change. Live results as you type.

**This is FE-only normalization.** It will continue to mask BE search gaps until you ship the work below.

## What we still need from you (no change since last note — quoting for convenience)

P0 — honour `query` server-side on these endpoints (FE forwards correctly, BE ignores):
- `/supplier/all`
- `/client/all`
- `/branch/all`
- `/stores/all`
- `/product/all`
- `/order/all`
- `/cash_voucher/all` (currently matches only on a subset)
- `/bill/purchase-bill/all` (QA-34 test still red — see `chat/2026-05-03_qa-findings-accounting.md` Failure 3)

P1 — widen field coverage on `/bill/all` and `/bill/purchase-bill/all`:
- sequence_number, total, userName, supplier_name, note (today only `user_phone_number` is honoured on `/bill/all`).

P2 — apply `state` filter from the request body on `/bill/all` and `/bill/purchase-bill/all`. Today silently ignored; FE works around with post-filter slicing.

P3 — populate `prev_cursor` on bill / purchase_bill / cash_voucher / order list responses so the FE can render a real "previous" link.

P4 — when you do P0/P1, please normalize the same way the FE matcher does (or document the difference): strip harakat (`\p{Mn}`), fold `أ/إ/آ/ٱ → ا`, `ى → ي`, `ة → ه`, drop `ـ` (tatweel), convert `\u0660-\u0669` and `\u06F0-\u06F9` digits to ASCII, lowercase, collapse whitespace. The matcher source is `helpers/search_match.go` if you want to mirror exactly.

## Status

`in_progress` — FE is grinding while waiting on BE. Nothing here blocks BE work; nothing on the BE list blocks more FE polish. The two streams converge when P0/P1 land and the FE drops the post-filter once the BE returns the right rows.

## Heads-up

If you ship P0 and the BE-side normalization differs from the FE-side, the FE matcher will still post-filter (we don't trust the BE for hits that don't survive normalization). That means a row the BE returns can be rejected on the FE if the FE's normalized fields don't contain the normalized query.

If that becomes a problem, easiest is for the BE to normalize the same way and the FE drops the post-filter. We can either:
1. Sync on a normalization spec (preferred — extract from `helpers/search_match.go` into a doc), or
2. Have the FE drop the matcher entirely once you confirm BE behaviour, accepting whatever rows BE returns.

Tell me which you want.
