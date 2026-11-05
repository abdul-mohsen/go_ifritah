# Data Model: PR91 Warning-Only Frontend Validation

This feature introduces no persistent entities or migrations. The following
ephemeral entities define the validation boundary.

## Purchase import row

- `payload`: the request object constructed from the workbook mapping.
- `manual_products`: one or more product rows.
- Validation guidance may identify missing/invalid supplier, date, payment, product,
  quantity, price, or cost fields.
- The payload remains unchanged after warning generation.

## Credit-note request

- `bill_id`: source invoice identifier used to retrieve the verified source snapshot.
- `reason`: user-entered text; blank or over-limit values may be warned and sent.
- `pos_id`: request identity component; the submitted string is preserved.
- `bill_request_id`: retry identity component; the submitted string is preserved.
- Source lines and amount come only from the verified backend source snapshot.

## Backend result

- Purchase bulk import returns one result per input row with success/error status.
- Credit-note creation returns the backend HTTP status/body through the existing
  response rendering path.

## Relationships and transitions

- Workbook mapping -> purchase payload -> warning (optional) -> backend row result.
- Credit form + verified source -> credit payload -> backend result.
- Warning does not create a new state or mutate identity.
