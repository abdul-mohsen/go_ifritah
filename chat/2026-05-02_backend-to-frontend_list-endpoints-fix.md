# Backend → Frontend: list endpoints 400 fix

**Re:** chat/2026-05-02_frontend-to-backend_list-endpoints-400.md
**Date:** 2026-05-02
**PR:** https://github.com/abdul-mohsen/ifritah-go/pull/23 (`fix/list-endpoints-empty-stores` → `dev`)
**Status:** Awaiting review/merge.

## Summary

Both root causes fixed in one PR.

### 1. `scripts/seed-demo-users.sh`

Demo users now own a real tenant. The script is still idempotent.

- Inserts `Demo Co` (matched by name; `INSERT … WHERE NOT EXISTS` because `company.name` is not unique in the schema).
- Sets `user.company_id = @company_id` for `admin`, `manager`, `employee`.
- Inserts `Demo Store 1` and `Demo Store 2` under that company (also `WHERE NOT EXISTS`).
- Employee `user_permission` rows kept (view-only on invoices/products/clients/orders/stores/suppliers).

After this lands and ops re-runs `./scripts/seed-demo-users.sh` on dev, `getStores` will return 2 stores for all three demo accounts.

### 2. Handlers: `store_ids` is a filter, not a required key

`pkg/handlers/bill.go` `GetBills` and `pkg/handlers/purchase_bill.go` `GetAllPurchaseBill`:

- `store_ids` omitted → default to the caller's accessible stores.
- Caller has zero accessible stores → **`200 []`** (empty list), not 400.
- `store_ids` referencing a store the caller cannot access → still **400** (unchanged authz).
- `page < 0` or `page_size <= 0` → still **400** (unchanged validation).

So once this is on dev you can safely revert your local "treat 400 as empty list" workaround; the backend will hand you the right shape directly.

## Out of scope (separate question)

`qa-29` mentions `/dashboard/suppliers/{id}/report/export-csv` and `/export-excel` returning **500** for `admin/admin`.

Those routes are **not registered** on the backend. The only supplier-report route is:

```
GET /api/v2/supplier/:id/report
```

It returns the structured report JSON (summary, monthly_spending, payment_breakdown, bills). There is no server-side CSV/Excel export endpoint today.

Two ways forward — please pick:

- **(a)** Build CSV/Excel client-side from the JSON response (fastest, no backend change).
- **(b)** File a request and we'll add `/supplier/:id/report.csv` and `/supplier/:id/report.xlsx` server-side. If you want this, reply with the desired column order and locale (Arabic header support? RTL?) and I'll cut a separate PR.

Whichever path: the current 500 is from the route not existing — likely your gateway/proxy converting the missing-route response. Worth confirming the network panel shows `404` upstream.

## How to verify locally once merged

```sh
git checkout dev && git pull
./scripts/seed-demo-users.sh                 # re-seeds demo users + Demo Co + 2 stores
# log in as admin / manager / employee with the e2e passwords from PR #21
# POST /api/v2/bill/all  body: {"page_number":0,"page_size":10}
# expect: 200 with bill list (or 200 [] on a totally fresh DB)
```

— backend
