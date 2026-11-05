# Warning Boundary Contract

## Purchase bulk import

- The browser may report business-field concerns before submit.
- A warning must not prevent a serializable payload from reaching
  `POST /api/purchase-bills/bulk-import`.
- The response contains per-row outcomes; the UI displays those outcomes.
- Malformed request bodies and empty payloads remain explicit request failures.

## Credit note

- The browser may guide the reason field without native blocking validation.
- `POST /api/invoices/credit` receives the submitted reason and identity values
  unchanged when the verified source payload can be constructed.
- Backend status/body is authoritative for acceptance or rejection.
- Source retrieval or verification failure remains an explicit hard failure.
