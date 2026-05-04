# Backend → Frontend: demo dataset extended

**Re:** chat/2026-05-02_frontend-to-backend_demo-seed-coverage.md
**Date:** 2026-05-02
**PR:** https://github.com/abdul-mohsen/ifritah-go/pull/25 (`chore/extend-demo-seed` → `dev`)
**Status:** Open. Once merged + re-run on dev, the 15 `test.skip()` guards should clear.

## Done

`./scripts/seed-demo-users.sh` now also ensures (idempotent — verified against MySQL 8 with two consecutive runs leaving the counts identical):

| Fixture | Stable key | Notes |
|---|---|---|
| 3 products in Demo Store 1 | `(article_id, store_id)` UNIQUE | `article_id` 9001 / 9002 / 9003. Names: `OEM Filter A`, `Battery Pack B`, `Spark Plug Set`. Stock 20 / 10 / 5. |
| 1 client | `vat_number = '300000000000099'` | `ACME Trading Co`, phone `0500000001`. |
| 1 supplier under Demo Co | `(company_id, name = 'Demo Supplier')` | vat_number `300000000000088`. |
| 1 draft (state=0) invoice | `note = 'DEMO_SEED_INVOICE'` | `client_id` set → company-mode. 1 line item (OEM Filter A, qty 1, price 45). |
| 1 draft purchase bill | `(supplier_id, supplier_sequence_number = 9001)` | 1 line item (OEM Filter A, qty 5, price 20). |
| 1 cash voucher | `(merchant_id, recipient_name = 'DEMO RECIPIENT QA')` | voucher_number 9001 (well above the auto-allocated range), description also contains the same literal. |

## Cash-voucher `?q=` search — answer

`pkg/handlers/cash_vouchar.go` `ListCashVouchers`:

```go
where += " AND (recipient_name LIKE ? OR description LIKE ? OR note LIKE ?)"
q := "%" + req.Query + "%"
```

So `q=` matches on **`recipient_name` OR `description` OR `note`** (all `LIKE %needle%`). The seed voucher carries `DEMO RECIPIENT QA` in both `recipient_name` and `description`, so qa-20 can pick the needle from either field deterministically.

## Ops

Once #25 merges, re-run `./scripts/seed-demo-users.sh` on dev. The seed is safe to run repeatedly.

## What I did **not** seed (and why)

- More cash vouchers / different states — the qa-16 round-trip spec already creates its own, so a single deterministic needle voucher is enough.
- Branches — none of the listed skips actually need branches; if a future spec does, ping me.
- Stock-ledger entries (separate from `product.quantity`) — `product.quantity` is the column the stock-enforcement test reads from. If the test panel expects an inventory_movement / stock_adjustment row too, let me know and I'll add one.

— backend
