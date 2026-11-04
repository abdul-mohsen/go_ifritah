# Credit Source API Contract

## Backend read endpoint

`GET /api/v2/bill/{bill_id}/credit-source`

Requires the existing authenticated API session and access to the source bill's store. The bill must be final, have a valid branch, and have a provable remaining credit.

Success response:

```json
{
  "bill_id": 42,
  "branch_id": 7,
  "amount": "11.50",
  "lines": [
    {
      "bill_product_id": 5,
      "quantity": "1",
      "price": "10.00",
      "vat": "15.00"
    }
  ]
}
```

`amount` is the exact sum of the full remaining source-line quantities with the backend's existing discount/VAT rounding rules. The endpoint returns a non-success status when required source metadata is missing, the bill is a draft/deleted/inaccessible, no creditable lines remain, or the remaining amount cannot be reconciled.

This is a read-only snapshot. It can become stale; the credit POST revalidates all line/amount constraints transactionally.

## Backend credit POST

`POST /api/v2/bill/credit`

The frontend server constructs the request from the latest credit-source response and explicit user reason:

```json
{
  "bill_id": 42,
  "branch_id": 7,
  "pos_id": "web",
  "bill_request_id": "5e1a7c2d-9b3f-4c8e-a6d1-2f0b9c4e7a13",
  "reason": "Returned goods",
  "amount": "11.50",
  "lines": [
    {
      "bill_product_id": 5,
      "quantity": "1",
      "price": "10.00",
      "vat": "15.00"
    }
  ]
}
```

The exact stable `pos_id` value follows the existing frontend identity validator. The browser does not provide authoritative amount or line values.
