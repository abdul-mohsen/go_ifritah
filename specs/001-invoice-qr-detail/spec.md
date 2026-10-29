# Invoice Detail ZATCA QR Rendering

## Summary

Restore the invoice detail data path for persisted ZATCA QR and optional
artifact/reference values while keeping missing or pending QR data explicit and
safe.

## User Value

Users viewing an issued invoice can see the persisted ZATCA QR that belongs to
that invoice. Users viewing an invoice still awaiting ZATCA processing see a
clear localized unavailable/pending state instead of a broken or fabricated QR.

## Scope

This feature changes only the frontend invoice detail handler/template path,
its focused regression tests, and directly related localization. Monitor,
onboarding, outcome, and backend schema behavior are excluded.

## Functional Requirements

1. Pass persisted `qr_code` to the invoice detail template as `qr_code`.
2. Normalize missing, null, or non-string QR data to an empty safe value.
3. Render the existing localized QR block only for a non-empty persisted value.
4. Render an explicit localized unavailable/pending state without QR script
   initialization when the QR is empty.
5. Preserve optional persisted artifact/reference values through the detail
   template data path without adding a backend schema.
6. Preserve current invoice status/type/localization behavior.
7. Cover the missing template-data regression with a red-first handler test.

## Acceptance Scenarios

- A persisted QR value appears in the invoice detail HTML and initializes the
  existing QR display.
- An empty/pending QR produces no QR canvas or initialization script and shows
  the localized pending/unavailable message.
- Existing state/status/type content remains unchanged for the same invoice
  fixture.
- The focused regression is red before the fix and green afterward.

## Assumptions and Constraints

- Backend raw bill fields are optional and may be absent.
- Existing contextual template escaping and `helpers.SafeString` remain in use.
- No monitor/onboarding/outcome schema changes are permitted.
- The work is committed locally only; it is not pushed and no PR is created.

## Success Criteria

- 100% of focused persisted-QR and empty-QR regression cases pass.
- The invoice detail page remains renderable for invoices with no ZATCA QR.
- Existing status/type assertions remain unchanged.
- The full Go test suite passes, or limitations are documented before commit.
