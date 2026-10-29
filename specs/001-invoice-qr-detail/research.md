# Research: Invoice Detail ZATCA QR Rendering

## Repository Evidence

1. `handlers/invoices.go` fetches the complete raw bill with
   `helpers.FetchBillRaw`, parses it with `helpers.ParseBillRaw`, and builds the
   `invoice-detail` template map.
2. `helpers.ParseBillRaw` already copies `qr_code` into its `extra` map. The
   handler maps other optional values such as `url`, `note`, and dates, but
   does not map `extra["qr_code"]` to `data["qr_code"]`.
3. `templates/invoice-detail.html` already has a localized QR block guarded by
   `{{ if .qr_code }}` and embeds the value only inside the existing
   `html/template`-escaped template context.
4. `resources/labels.go` contains the localized
   `tpl.invoice.qr_code` label. No QR generation belongs in the frontend.
5. Backend code used by adjacent task worktrees persists `qr_code` on the bill
   and also has invoice UUID/hash/XML-path columns in the storage layer; the
   frontend must treat those optional fields as pass-through data and must not
   change backend schema.
6. `helpers.InvoiceStatus` and `helpers.InvoiceTypeLabel` are independent of
   the QR value and must not be changed for T021.

## Resolved Decisions

| Requirement | Evidence | Decision |
|---|---|---|
| FR-001 | `ParseBillRaw` contains `qr_code`; handler data map omits it | Add the normalized QR value to the handler map |
| FR-002 | `helpers.SafeString` returns empty for nil/non-string values | Use the existing helper rather than type assertions in the template |
| FR-003 | QR markup and localized label already exist | Keep the current QR block and feed it the persisted value |
| FR-004 | Empty QR currently skips the block | Add explicit localized unavailable/pending presentation while retaining the conditional script |
| FR-005 | Raw response is map-based and optional | Preserve related persisted artifact/reference values as optional detail data; no model/schema change |
| FR-006 | Status helpers are separate | Do not touch status/type calculation |
| FR-007 | Existing handler render tests use `httptest` backends and session cookies | Add a focused handler regression test with a persisted QR fixture |

## Alternatives Considered

- **Generate a new QR in the frontend**: rejected; the persisted ZATCA QR is
  authoritative and the frontend has no signing/submission responsibility.
- **Change `models.Invoice` to include ZATCA fields**: rejected for T021;
  `ParseBillRaw` already supports optional raw values and schema changes are
  explicitly out of scope.
- **Fetch a second ZATCA endpoint**: rejected; it would add latency and could
  disagree with the persisted bill detail response.
- **Remove the QR block when empty**: rejected; an explicit localized
  unavailable/pending state is required, while still omitting executable QR
  initialization.

## Remaining Uncertainty

Backend deployments may expose different subsets of artifact/reference fields.
The implementation will pass through only fields actually present and will
render optional values conditionally, so missing fields remain harmless.
