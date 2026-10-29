# Design: Invoice Detail ZATCA QR Rendering

## Data Flow

1. `HandleGetInvoice` authenticates the request and fetches the raw bill.
2. `ParseBillRaw` converts common invoice fields and retains optional
   ZATCA/detail fields in `extra`.
3. The handler builds the existing template data map, adding normalized QR and
   optional artifact/reference values.
4. `invoice-detail.html` renders the persisted QR only when non-empty. When it
   is absent, it renders a localized pending/unavailable message and no QR
   initialization script.

## Component Boundaries

- **Handler**: owns backend field extraction, normalization, and status
  calculation. It does not generate QR data.
- **Parser**: owns raw bill field preservation and date normalization.
- **Template**: owns conditional presentation and localization calls.
- **Resources**: owns Arabic/English labels for the explicit QR state.

## Optional ZATCA Metadata

The handler will expose optional persisted fields using stable template keys:

- `qr_code`
- `invoice_uuid`
- `invoice_hash`
- `invoice_xml_path`

Each value is normalized with the existing safe string helper. The template
renders an artifact/reference section only when at least one value is present;
missing values do not produce blank labels or links. No new backend endpoint,
model, database column, or schema migration is introduced.

## Safety and Failure Handling

- Empty, null, or non-string QR values normalize to `""`.
- The QR script remains inside the non-empty conditional, so no client-side
  library is loaded or invoked for pending invoices.
- The QR value remains in the existing contextual escaping path.
- Existing unauthorized and backend error handling remains unchanged.
- Status/type calculation remains delegated to existing helpers.

## Localization

All new user-visible text uses `resources.L` through `L` template calls. The
existing QR label is retained; a new localized pending/unavailable label is
added beside it.

## Test Strategy

- Add a handler regression test with a mock backend returning a non-empty
  persisted QR and artifact/reference values; assert those values appear in
  rendered HTML.
- Add an empty/pending case to assert no QR initialization script or canvas is
  emitted and the localized pending label is present.
- Assert existing status output remains present in the persisted case.
- Run `gofmt`, the focused handler test, related handler/config tests, and
  `go test ./...`.

## Requirement Traceability

| Requirement | Design element |
|---|---|
| FR-001–FR-002 | Handler normalization and `qr_code` map key |
| FR-003–FR-004 | Template conditional and localized empty state |
| FR-005 | Optional metadata map and conditional artifact section |
| FR-006 | Unchanged status/type helper calls and controls |
| FR-007 | Focused `httptest` handler/template regression cases |
