# Supplier Report Performance Suggestion

Backend repo: https://github.com/abdul-mohsen/ifritah-go/tree/dev

## Conclusion

The report is slow because the frontend currently has to fall back to an expensive `1 + N + 1` backend call pattern when the aggregate supplier report endpoint fails:

- one call to load all purchase bills,
- one detail call for each purchase bill so the frontend can discover whether it belongs to the selected supplier and read its item lines,
- one call to load cash vouchers.

The backend should implement/fix one aggregate report endpoint that returns the complete filtered supplier statement in a single response. Then the frontend can render the page and generate Excel/PDF from that same report shape without calling purchase bill details one by one.

Backend implementation target:

`GET /api/v2/supplier/{supplier_id}/report?from=YYYY-MM-DD&to=YYYY-MM-DD`

This endpoint should do the supplier/date filtering and aggregation in the backend database layer. The frontend should not need to loop through every purchase bill and request every purchase bill detail.

## What The Frontend Needs

For a selected supplier and date filter, return all data needed by the visible page and downloads:

- `summary`: totals, VAT, paid/unpaid, closing balance, bill/payment counts.
- `bills`: every purchase bill for the supplier in the selected date range, including supplier bill number, totals, state, effective date, due date, received info, and item count.
- `payments`: every supplier cash voucher in the selected date range.
- `top_items`: aggregated purchased items for that supplier/filter.
- `aging`: current, 1-30, 31-60, 61-90, and 90+ buckets.
- `monthly_spending`: purchases per month, and enough payment data for the frontend to show monthly net.
- `payment_breakdown`: totals by payment method.

If this endpoint returns `200` with the full shape below, the frontend can avoid spamming the backend.

## Problem

The frontend supplier account statement page calls:

`GET /api/v2/supplier/{supplier_id}/report?from=YYYY-MM-DD&to=YYYY-MM-DD`

When this endpoint is missing, the frontend can only build the report through a legacy fallback:

1. `POST /api/v2/purchase_bill/all` with a large page size.
2. `GET /api/v2/purchase_bill/{id}` once for every purchase bill so it can discover `supplier_id`, item lines, dates, and totals.
3. `POST /api/v2/cash_voucher/all` with a large page size.
4. Filter and aggregate everything in the Go frontend process.

That is effectively `1 + N + 1` backend calls for one report page or one download. Large tenants make the page and exports slow. If the user opens the page and then downloads Excel/PDF for the same filter, the backend should not need to recompute via many detail calls.

## Proposed Endpoint

Implement or fix the aggregate endpoint in the backend:

`GET /api/v2/supplier/{supplier_id}/report?from=YYYY-MM-DD&to=YYYY-MM-DD`

The endpoint should return all data needed by the page and by PDF/Excel exports for the selected date filter in one response:

```json
{
  "summary": {
    "bill_count": 0,
    "total_spent": 0,
    "total_before_vat": 0,
    "total_vat": 0,
    "total_discount": 0,
    "total_payments": 0,
    "payment_count": 0,
    "paid_total": 0,
    "unpaid_total": 0,
    "closing_balance": 0,
    "avg_bill": 0,
    "received_count": 0
  },
  "bills": [
    {
      "id": 0,
      "sequence_number": 0,
      "supplier_sequence_number": "",
      "total": 0,
      "total_before_vat": 0,
      "total_vat": 0,
      "discount": 0,
      "state": 0,
      "effective_date": "2026-04-01T00:00:00Z",
      "payment_due_date": "2026-04-30T00:00:00Z",
      "received_at": "",
      "received_by": 0,
      "item_count": 0
    }
  ],
  "payments": [
    {
      "id": 0,
      "voucher_number": 0,
      "voucher_type": "payment",
      "effective_date": "2026-04-01T00:00:00Z",
      "amount": 0,
      "payment_method": "cash",
      "description": ""
    }
  ],
  "top_items": [
    {
      "item_name": "",
      "total_qty": 0,
      "total_value": 0,
      "avg_price": 0,
      "bill_count": 0
    }
  ],
  "aging": [
    { "bucket": "current", "bill_count": 0, "bucket_total": 0 },
    { "bucket": "1-30", "bill_count": 0, "bucket_total": 0 },
    { "bucket": "31-60", "bill_count": 0, "bucket_total": 0 },
    { "bucket": "61-90", "bill_count": 0, "bucket_total": 0 },
    { "bucket": "90+", "bill_count": 0, "bucket_total": 0 }
  ],
  "monthly_spending": [
    { "month": "2026-04", "total_spent": 0 }
  ],
  "payment_breakdown": {
    "cash_total": 0,
    "bank_transfer_total": 0
  }
}
```

## Query Semantics

- `from` and `to` are inclusive date filters.
- Use purchase bill `effective_date` for bill rows and totals.
- Use cash voucher `effective_date` for supplier payments.
- Return the full filtered dataset, not only a page of rows. The frontend page and its PDF/Excel exports must contain all rows for the selected filter.
- Do not require the frontend to call purchase bill detail endpoints to know `supplier_id` or item lines.

## Performance Notes

Recommended indexes:

- `purchase_bills(supplier_id, effective_date)`
- `cash_vouchers(recipient_type, recipient_id, effective_date)`
- `purchase_bill_items(bill_id)` or equivalent item-line table index
- Any manual purchase bill item table should also be indexed by `bill_id`

Expected behavior:

- One backend request should render the report page.
- One backend request should generate the full Excel/PDF export when the frontend cache is cold.
- Backend `5xx` errors should be fixed at the aggregate endpoint rather than relying on frontend fallback.
- The endpoint should remain stable for long date ranges such as a full year.

## Acceptance Criteria

- `GET /api/v2/supplier/{id}/report?from=&to=` returns `200` with the response shape above.
- Response includes all bills, payments, top items, aging, monthly totals, and summary values for the selected filter.
- No frontend N+1 calls are required for normal operation.
- Excel/PDF generated from the frontend contains the same filtered report sections shown on the page.