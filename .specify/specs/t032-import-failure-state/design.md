# T032 Import Failure State Design

## Boundary

The change is confined to frontend import view preparation and its focused
handler package regression test:

```text
T031 import JSON
  -> ZatcaImportView
  -> prepareZatcaImportView
  -> localized status template
```

No backend request, persistence, route, template, action, or localization
resource changes are needed.

## Status derivation

1. Keep the existing `hasFailure` calculation.
2. Keep `review` mapped to `pending`.
3. When failure evidence is present and the raw status is `review`,
   `approved`, or `posted`, set `DisplayStatus` to `failed`.
4. Leave a `posted` import without failure evidence as `posted`.
5. Leave raw `Status`, `StatusClass` selection, and action flags governed by
   their existing rules.

The display-only override means the existing localized `failed` and `posted`
labels continue to be selected by the unchanged template and resources.

## Test strategy

- Add a table-driven unit regression in `handlers/zatca_import_ui_test.go`.
- Assert that posted failure-class evidence and posted error-severity
  validation evidence display as `failed`.
- Assert that a posted import without failure evidence remains `posted`.
- Run the exact test against the base before editing the handler to capture
  the required red result.

## Traceability

| Requirement | Design response |
|---|---|
| FR-001 | Evidence-aware override includes `posted` and `approved`. |
| FR-002 | No-failure posted input bypasses the override. |
| FR-003 | Only `DisplayStatus` derivation changes. |
| FR-004 | Existing template/resource labels remain untouched. |
| FR-005 | Focused red/green table-driven regression. |
