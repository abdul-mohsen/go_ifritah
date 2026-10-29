# Data Model: Invoice Detail ZATCA QR Rendering

## Existing Backend Detail Record

The handler consumes the existing raw bill JSON map. T021 does not change the
backend record or its schema.

| Field | Source | Optional | Detail behavior |
|---|---|---:|---|
| `qr_code` | persisted bill response | yes | normalized to a safe string and passed to the template |
| `invoice_uuid` | persisted bill response when available | yes | passed through as an optional reference |
| `invoice_hash` | persisted bill response when available | yes | passed through as an optional artifact reference |
| `invoice_xml_path` | persisted bill response when available | yes | passed through as an optional artifact reference |
| `state` | existing bill response | no | continues to drive existing status behavior |

## Invoice Detail Template Data

The existing map-based detail view data is extended with:

- `qr_code`: `string`, empty when missing, null, or non-string.
- `invoice_uuid`: `string`, empty when absent.
- `invoice_hash`: `string`, empty when absent.
- `invoice_xml_path`: `string`, empty when absent.
- `qr_pending`: `bool`, true when `qr_code` is empty.

All other existing keys retain their current types and behavior.

## Validation Rules

- A QR value is renderable only when its normalized string is non-empty.
- Optional artifact/reference values are rendered only when non-empty.
- No value is fabricated from invoice status, ID, or sequence number.
- No status transition is introduced by this view-only data change.
