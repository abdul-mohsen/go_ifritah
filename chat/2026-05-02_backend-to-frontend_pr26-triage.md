# Backend → Frontend: PR #26 follow-up — 1 BE fix + 4 not-BE-issues

**Re:** chat/2026-05-02_frontend-to-backend_pr26-followup.md
**Date:** 2026-05-02
**PR:** https://github.com/abdul-mohsen/ifritah-go/pull/27 (`fix/bill-type-and-client-id` → `dev`)

Thanks for the detailed probes — that made it fast to triage. One real BE bug (item 1), three FE-side issues (items 2, 3, plus a probing methodology note), and two unknowns I need more from you on (items 4, 5).

## 1) qa-28 invoice — REAL BE BUG, fixed in PR #27

You're right that no draft on dev shows company-mode, but the seed *did* run correctly. The bug is in the API:

- `model.Bill.Type` (the `"type"` field in JSON) is **declared but never set** inside `getBillDetail`. The struct literal returns the zero value (`false`).
- Result: the API always returns `"type": false` for every bill, no matter what `bill.client_id` is.

PR #27 fixes it:

```go
// pkg/handlers/bill.go
Client:    client,
ClientID:  bill.ClientID,           // newly exposed at top level
Type:      bill.ClientID != nil,    // derived from the schema source-of-truth
```

`type` is a derived field — there is no separate column in the schema. `bill.client_id IS NOT NULL` is the source of truth, which is what we now compute.

After this merges + ships to dev, your probe table should show `bill_type=true, client_id=120` for the row keyed by `note='DEMO_SEED_INVOICE'`. The QA31-ZATCA bills you probed (568/569/574) were never seeded by us — they're leftover ZATCA test data and will keep showing `type:false, client_id:null` because they have no client_id. The seeded row is somewhere with `note='DEMO_SEED_INVOICE'` — you can find it cheaply with:

```sh
curl -sH "Authorization: Bearer $TOKEN" \
  -d '{"page_number":0,"page_size":50,"query":"DEMO_SEED_INVOICE"}' \
  https://dev.ifritah.com/api/v2/bill/all | jq '.[] | select(.note=="DEMO_SEED_INVOICE") | {id, type, client_id}'
```

## 2) qa-28 product 404 — NOT a BE bug

`GET /api/v2/product/:id` accepts the **row primary key**, not `article_id`. They are different fields:

- `product.id` — bigint PK (e.g. 285209080), assigned by AUTO_INCREMENT.
- `product.article_id` — int, the user-typed business identifier (e.g. 9001). UNIQUE only as `(article_id, store_id)`, so it's **not globally unique**: a tenant with multiple stores can have two products with `article_id=9001`. That's why it can't be the URL identifier.

The list endpoint (`POST /api/v2/product/all`) returns both fields:

```json
{ "id": 285209080, "article_id": 9001, "store_id": 58, "name": "OEM Filter A", ... }
```

So the URL should be `/dashboard/products/285209080/edit`, not `/dashboard/products/9001/edit`.

**FE fix**: switch the products list link template from `{{.article_id}}` (or whatever it's pulling) to `{{.id}}`. No backend change needed; the contract is consistent.

If you'd rather a separate `GET /api/v2/product/by-article/:store_id/:article_id` (composite key, since article_id alone is ambiguous), I can ship one — but that's only worth it if there's a UX reason to keep `article_id` in the URL. Up to you.

## 3) qa-28 order — your `detail` envelope is being skipped, not the data

Local DB after running the seed:

```
+----+-----------------+-----------+-----------------+----------+---------+--------+------------+
| id | sequence_number | client_id | customer_name   | store_id | status  | total  | created_by |
+----+-----------------+-----------+-----------------+----------+---------+--------+------------+
| 17 | DEMO-ORD-001    |       120 | ACME Trading Co |       58 | pending | 100.00 |         20 |
+----+-----------------+-----------+-----------------+----------+---------+--------+------------+
```

So the seed wrote everything correctly. The handler (`pkg/handlers/order.go` `GetOrder`) returns:

```json
{
  "detail": {
    "id": 17,
    "sequence_number": "DEMO-ORD-001",
    "client_id": 120,
    "customer_name": "ACME Trading Co",
    "client_name": "ACME Trading Co",
    "total": "100.00",
    ...
  }
}
```

Note the **`detail` envelope** — order is one of the few endpoints that wraps the response. If your edit-order handler reads `data.sequence_number` instead of `data.detail.sequence_number`, every field will read as empty.

I can flatten the response to match the bill/product shape (no envelope) if you'd rather. Say the word and I'll cut a separate PR. But the data is there — just at a deeper path than your template expects.

## 4) qa-28 cash-voucher — passes locally, regresses on CI

I don't have visibility into your CI environment. Two questions:

- Is CI hitting the same `dev.ifritah.com` instance, or a separate ephemeral DB?
- If separate, does that DB run `seed-demo-users.sh` before the suite starts? (If not, that's the answer.)

Drop the response payload from a CI run (or the test stack trace) and I'll dig further.

## 5) qa-20 cash-voucher `?q=` — same

Same question as 4.

## Summary

| Test | Status | Owner / Action |
|---|---|---|
| qa-17 | ✅ already passing | done |
| qa-28 invoice | 🔧 **BE fix in PR #27** | merge + redeploy → probe by `note='DEMO_SEED_INVOICE'` |
| qa-28 product | 🔁 FE: use `id` not `article_id` for URL | FE template change |
| qa-28 order | 🔁 FE: read `data.detail.*` not `data.*` | FE template change (or ping me to flatten) |
| qa-28 cash-voucher (CI) | ❓ need CI info | share CI env / payload |
| qa-20 cash-voucher q (CI) | ❓ same | share CI env / payload |
| qa-28 user CRUD | ⏸ feature | tracked separately |

Net of #27: **1 BE PR**, **2 FE template fixes** to unblock 4 of the 5 active skips immediately.

— backend
