# T023 Design

## Architecture

`HandleZatcaMonitor` remains the page route. After authentication it starts
one authenticated GET for each documented backend endpoint:

```text
browser -> frontend /dashboard/zatca-monitor
              | session validation/refresh
              +--> backend /api/v2/zatca/monitor/stats
              +--> backend /api/v2/zatca/monitor/branches
              +--> backend /api/v2/zatca/monitor/submissions?limit=50
              |
              +--> server-rendered localized monitor template
```

Each response is decoded into a frontend contract model. Mapping functions add
presentation-only fields (localized status key, CSS class, safe display
fallback, normalized state, and pending age). No persistence or backend
mutation is introduced.

## Failure handling

- Authentication failure continues to use the existing redirect path.
- Request, non-200, or JSON decode failure records a section error and logs only
  the endpoint/error category, never an access token or response body.
- Successful sections remain visible on partial failure.
- A failed section gets an explicit localized error/placeholder and no numeric
  success fallback.
- Empty successful arrays render localized empty rows.
- A completely loading/refreshing view exposes an `aria-busy` indicator; the
  server response never claims data is loaded before the calls complete.

## Presentation mapping

- Branch `zatca_status`: active, expiring soon, expired, not active/deleted,
  unknown. Unknown is rendered with a neutral/unknown label.
- Submission `state` takes precedence over `status`; legacy `warning` maps to
  accepted-with-warning. Unknown states use a neutral unknown badge and are
  never accepted.
- Provider reference uses `provider_reference`, then `zatca_ref` as the
  compatibility reference. Missing values render the localized unavailable
  marker.
- Warning text uses provider warning/message fields, then `warning_msg`; it is
  displayed only when non-empty.
- Pending age uses `submitted_at`, falling back to `last_attempt_at`, parsed as
  RFC3339 or the legacy SQL datetime format. Future/invalid timestamps render
  unavailable.

## Testing

- A focused handler regression test uses an authenticated `httptest.Server`,
  asserts all three endpoint paths/query parameters and the bearer header, and
  asserts live backend values plus absence of the old mock total.
- The same handler path is covered by a partial-failure/empty-state test if
  needed by the focused suite.
- Existing template parse/execute tests and handler package tests remain the
  regression gate.

## Requirement traceability

| Requirement | Design element |
| --- | --- |
| FR-01 | Session validation plus `DoAuthedRequestWithRetry` |
| FR-02–FR-04 | Contract structs and response decoders |
| FR-05–FR-08 | Branch/submission mapping and template badges/details |
| FR-09–FR-10 | Empty rows and per-section errors |
| FR-11 | Refresh reloads current backend-backed page |
| FR-12 | Focused authenticated backend regression test |
