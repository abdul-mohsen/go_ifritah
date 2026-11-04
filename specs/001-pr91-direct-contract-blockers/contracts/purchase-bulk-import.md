# Purchase Bulk Import Contract

## Frontend endpoint

`POST /api/purchase-bills/bulk-import`

Requires the existing purchase-bill add permission and CSRF protection. The body remains the array already emitted by the import page. Each bill must include:

- `store_id`
- `supplier_id`
- positive `supplier_sequence_number`
- date-only or RFC3339 `effective_date` parseable by the Riyadh helper
- nonzero valid `payment_method`
- `manual_products` with non-empty name, valid price, positive quantity, and `cost_price`

The frontend handler normalizes date-only values to RFC3339 using the application's `Asia/Riyadh` location, then posts each valid bill to backend `POST /api/v2/purchase_bill`. It returns one indexed result per bill. A backend failure is reported as a failed item; an invalid item is not posted.

Example response:

```json
{
  "total": 1,
  "success": 1,
  "failed": 0,
  "results": [
    {"index": 0, "ok": true}
  ]
}
```

Exact field names remain those of `models.PurchaseBillPayload` and backend `AddPurchaseBillRequest`; generated arithmetic or supplier sequence must not be silently defaulted.
