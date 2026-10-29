# T032 Design

## Boundary

```text
Authenticated browser
  ├─ GET /dashboard/zatca/imports
  │    └─ frontend handler -> GET /api/v2/zatca/import -> list/upload template
  ├─ GET /dashboard/zatca/imports/:id
  │    └─ frontend handler -> GET /api/v2/zatca/import/:id -> detail template
  └─ /api/zatca/import...
       └─ authenticated frontend proxy -> T031 import API
```

No backend, parser, daemon, outbox, deployment, or outbound ZATCA submission
component is changed.

## Components

- `handlers/zatca_import.go`: typed response views, backend request helper,
  authenticated list/detail page handlers, and JSON proxy handlers.
- `templates/zatca-imports.html`: upload form, status filter, and lifecycle
  list.
- `templates/zatca-import-detail.html`: evidence/provenance/review/actions.
- `templates/components/zatca-import-status.html`: shared status badge.
- `main.go`: authenticated page and API route registration.
- `config/config.go`, `config/template_render_test.go`: template registration
  and parse/execute coverage.
- `resources/labels.go`, `static/js/i18n.js`: Arabic/English UI labels.
- `handlers/zatca_import_ui_test.go`: browserless handler/template contract
  regressions.

## Data flow

1. The page handler validates the session, fetches tenant-scoped T031 JSON,
   derives display status, and renders escaped template data.
2. The upload form reads a selected XML file, enforces the 5 MiB client limit,
   and posts `xml_content`, source metadata, unverified signature evidence,
   review-required validation metadata, and an idempotency key to the frontend
   proxy.
3. The proxy forwards JSON with the current refreshed backend token and copies
   backend status/body/content type back to the browser.
4. Detail actions send the current `workflow_version`, then reload the detail
   page after a successful response.

## Status and action policy

| Raw T031 status/evidence | Display | Actions |
|---|---|---|
| `review`, no failure evidence | pending | approve, request changes, reject |
| `review` plus failure/error evidence | failed | request changes, reject |
| `approved` | approved | post, request changes, reject |
| `posted` | posted | none |
| `rejected` | rejected | none |

The string `issued` is not used by the import UI. The posted label explicitly
means import workflow completion, not a provider-issued invoice.

## Failure handling and security

- Missing session returns JSON 401 for proxy calls and redirects page calls.
- Backend 401/refresh failure is surfaced as 401; other failures preserve
  backend status/body where safe, with generic gateway errors for transport
  failures.
- Upload body is capped before forwarding; browser rejects files over 5 MiB.
- Backend evidence is rendered through `html/template`; raw canonical/parsed
  JSON is displayed in `<pre>` and never evaluated as script.
- Action buttons are gated by raw lifecycle status, not derived display state,
  preventing a failed review from being posted.

## Test strategy

- Add a focused handler regression that uses an `httptest` backend to verify
  authenticated list/detail rendering and evidence/action/status contracts.
- Add template-source assertions for all lifecycle/action routes and the
  absence of issued semantics.
- Run the focused test red before implementation, then green after.
- Run `gofmt`, package tests, full `go test ./...`, and `git diff --check`.

## Requirement traceability

| Requirement | Design response |
|---|---|
| FR-001–FR-003 | Authenticated page/API proxy routes |
| FR-004 | Detail view for evidence, errors, reviews, history |
| FR-005, FR-008 | Derived statuses and no issued vocabulary |
| FR-006–FR-007 | Raw-status action gating and required reject reason |
| FR-009 | Additive route/template/label changes only |
| FR-010 | Red/green browserless regression |
