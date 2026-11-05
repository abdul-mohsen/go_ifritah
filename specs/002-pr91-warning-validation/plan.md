# Plan: PR91 Warning-Only Frontend Validation

## Summary

Adjust the frontend validation boundary in PR #91 so business-invalid but
serializable purchase-import and credit-note inputs are reported as warnings and
sent through the existing backend-authoritative paths.

## Technical approach

1. Add a localized inline warning area to the purchase import preview and change
   the existing row validator from a gate into warning collection.
2. Keep the constructed purchase payload unchanged and preserve structured backend
   per-row results.
3. Remove the credit-note handler's business-field preflight rejection and allow
   source-backed, serializable values to reach the backend.
4. Remove native reason validation that prevents the request before HTMX submission;
   show guidance without changing the submitted value.
5. Update focused tests and run repository checks.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Invalid rows are submitted | Backend per-row validation remains authoritative and visible |
| User input is changed silently | Warning path does not mutate payload or identity |
| Unsafe source request is forwarded | Source retrieval and verification remain hard failures |
| Existing valid flow regresses | Focused Go/JavaScript tests plus `npm run check` |

## Rollback

Revert only the warning-boundary changes; no schema or persistent data change is
introduced.

## Traceability

See `requirements.md`, `design.md`, and `tasks.md`. T004-T006 cover purchase
warnings; T007-T009 cover credit-note warnings; T010-T011 preserve hard failures;
T014-T016 are the final validation and QA gates.
