# Design: T005 Frontend C2 Submission Outcome UI

## Overview

Add a small frontend-side adapter that reads the authenticated ZATCA monitor
submission list, indexes rows by invoice ID, and resolves a view model for
invoice list/detail handlers. The adapter is additive: it preserves the
existing numeric invoice/credit-note status mapping when no durable row exists.

## Components and boundaries

### Contract model

Add frontend-only Go structs for the monitor response row and the resolved
invoice submission view. JSON fields match the approved backend contract,
including nullable provider/retry/artifact fields. The model accepts both
`state` and compatibility `status`; explicit `state` wins.

### Backend client

Add a helper that performs an authenticated `GET` request to
`/api/v2/zatca/monitor/submissions?limit=200`, decodes either a JSON array or
an envelope with a submissions/items/data field, and returns a map keyed by
`invoice_id`. A failed or unauthorized monitor request is non-fatal for invoice
page rendering: callers retain legacy status behavior, while unauthorized
requests continue to use existing session handling where appropriate.

The approved T003 route is admin-scoped. Invoice viewers without that backend
permission therefore receive the documented legacy fallback until a
tenant-authorized bill outcome response is available.

### Outcome resolution

Resolve a row in this order:

1. If no row or state is empty/unknown, use `helpers.InvoiceStatus` and existing
   localized status behavior.
2. If state is one of the six approved values, use a localized durable label
   and outcome CSS class.
3. Set `is_issued` only for `accepted` and `accepted_with_warning`.
4. Copy provider code, warning code/message, provider reference, retry count,
   next retry time, and request/cleared artifact references into the page map.
5. Set `is_retryable` only from an explicit backend boolean or an explicit
   retryable field when present; never infer a retry endpoint.

Legacy numeric state remains available to templates for draft/edit/delete
compatibility. Credit-note status behavior remains the first precedence path.

## Data flow

```text
GET /api/v2/bill/all ───────────────┐
                                    ├─ merge by invoice_id ──► list view model
GET /api/v2/zatca/monitor/submissions ┘

GET /api/v2/bill/:id ───────────────┐
                                    ├─ lookup invoice_id ───► detail view model
GET /api/v2/zatca/monitor/submissions ┘
```

The list fetch uses one bounded monitor call per page request. The detail fetch
uses the same bounded call and selects the requested invoice. Existing API
cache behavior is not changed; monitor data is not persisted in the frontend.

## Status presentation

| Durable state | Localized meaning | Issued? | Class intent |
|---|---|---:|---|
| `pending` | pending submission | no | pending/neutral |
| `accepted` | issued/accepted | yes | issued/success |
| `accepted_with_warning` | issued with warning | yes | warning/success |
| `rejected` | rejected | no | danger |
| `ambiguous` | outcome uncertain | no | warning |
| `terminal_failure` | terminal failure | no | danger |

Existing legacy statuses are unchanged when no durable row overrides them.

## Action behavior

- Draft edit/delete/submit actions continue to use the numeric draft state.
- No durable non-success state may render as issued.
- Credit-note action is enabled only when the resolved durable state is not
  present or is an issued-success state; rejected, ambiguous, pending, and
  terminal-failure rows do not receive it.
- PDF/view/print remain normal invoice actions and are not classified as
  issued-only.
- No retry button is added because T003/T004 define retry metadata but no
  frontend retry endpoint.

## Failure handling

- Empty, malformed, unknown, or unavailable monitor responses log a diagnostic
  and fall back to the existing invoice status mapping.
- A monitor response cannot elevate an invoice to issued from a legacy state;
  only explicit accepted states do so.
- The adapter must not overwrite invoice monetary or legacy business fields.

## Test strategy

- Add a focused handler regression using a mock backend for bill list/detail and
  monitor submissions.
- Assert rejected, ambiguous, and pending render distinct explicit labels and do
  not render issued/credit-note action markup.
- Keep existing pagination and page-render tests green.
- Run `gofmt -l`, the focused test, the handlers package tests, and `go test ./...`.

## Requirement traceability

| Requirement | Design element |
|---|---|
| FR-001 | Explicit six-state resolver with unknown fallback |
| FR-002 | `is_issued` derived only from accepted states |
| FR-003 | List handler monitor merge and list view fields |
| FR-004 | Detail handler monitor lookup |
| FR-005 | Durable-state-aware credit/issued action guards |
| FR-006 | Existing resource localization and legacy fallback |
| FR-007 | Mock backend handler/template regression |

## Rollout and rollback

The change is additive and uses the existing monitor route. If a deployment
does not yet expose the route, pages retain their prior rendering. Rollback is
one isolated commit revert; no database or backend rollback is required.
