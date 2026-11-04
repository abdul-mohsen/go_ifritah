# Data Model: PR #91 Direct Contract Blockers

## Sale line

| Field | Type/contract | Validation/source |
|---|---|---|
| `product_id` | nullable product identifier | Existing linked/manual product model |
| `price` | decimal value | Existing sale request |
| `quantity` | positive decimal | Backend sale validation |
| `discount` | decimal amount per backend #105 | Must be sent and persisted; no formula change |

Sale and draft totals continue to be calculated by the backend from persisted line values.

## Quick draft context

| Field | Type/contract | Validation/source |
|---|---|---|
| `store_id` | selected store identifier | Existing form selection |
| `branch_id` | positive branch identifier belonging to the selected store | Required backend context; no zero placeholder |
| `payment_method` | valid payment method identifier | Existing cash method is `10`; no zero placeholder |

## Credit source snapshot

| Field | Type/contract | Validation/source |
|---|---|---|
| `bill_id` | positive integer | Source final sale |
| `branch_id` | positive integer | Source bill's verified branch |
| `amount` | exact decimal string | Sum of all remaining source line totals including VAT |
| `lines[]` | non-empty array | Full remaining line quantities only |
| `lines[].bill_product_id` | positive integer | Existing source line primary key |
| `lines[].quantity` | positive decimal string | Source quantity minus quantities already credited |
| `lines[].price` | exact decimal string | Original source-line price |
| `lines[].vat` | decimal percentage string | Original source-line VAT rate |

The backend snapshot is read-only and non-authoritative for posting; the credit POST repeats validation transactionally. Missing metadata or an unreconciled amount produces an error, not a partial/default snapshot.

## Credit-note request

| Field | Type/contract | Validation/source |
|---|---|---|
| `bill_id` | positive integer | Route/source bill |
| `branch_id` | positive integer | Credit-source snapshot |
| `pos_id` | valid stable POS identity | Existing identity helper |
| `bill_request_id` | UUID/request identity | Stable for retry of the same credit-form history entry |
| `reason` | required trimmed text | Explicit user input |
| `amount` | positive decimal string | Credit-source snapshot |
| `lines[]` | full remaining lines | Credit-source snapshot |

## Purchase import bill and item

| Field | Type/contract | Validation/source |
|---|---|---|
| `store_id` | valid selected store | Existing import form |
| `supplier_id` | positive identifier | Supplier mapping |
| `supplier_sequence_number` | positive integer | Explicit workbook mapping; no fallback to `1` |
| `effective_date` | RFC3339 timestamp with `+03:00` | Date-only workbook value normalized in Riyadh |
| `payment_method` | valid nonzero identifier | Existing purchase payment method control/pattern |
| `manual_products[].name` | required product name | Workbook mapping |
| `manual_products[].price` | decimal | Workbook mapping |
| `manual_products[].cost_price` | decimal | Explicit cost mapping or explicitly verified same source field |
| `manual_products[].quantity` | positive decimal | Workbook mapping |

Import result reports total bills and one success/error result for each submitted item.

## Sale request identity

| Field | Type/contract | Lifecycle |
|---|---|---|
| `pos_id` | stable browser POS identifier | Existing localStorage behavior |
| `bill_request_id` | per-sale-attempt UUID | Browser history-entry scoped; stable for reload/back/resume/retry; fresh for a new entry |
