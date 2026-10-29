# Plan: T005 Frontend C2 Submission Outcome UI

## Summary

Add a compatibility-aware frontend adapter for the approved backend ZATCA
monitor submission response, then use it to resolve invoice list/detail status
and action flags without changing backend or daemon code.

## Phases

1. **Specify/research** — capture the T005 scope and verify T003/T004 response
   fields and issued semantics.
2. **Red regression** — add a mock-backend handler/template test for rejected,
   ambiguous, and pending states and record the base failure.
3. **Contract adapter** — add monitor response types, decoding, state
   normalization, and invoice-ID indexing.
4. **Page mapping** — merge resolved outcomes into list/detail handler data,
   preserving legacy fallback and setting issued only for two success states.
5. **Template/actions** — render localized explicit states, provider details, and
   safe action guards without inventing a retry endpoint.
6. **Validation/convergence** — run gofmt, targeted tests, `go test ./...`,
   inspect scope, and commit one isolated local change.

## Key decisions

- Use the approved monitor route as the authoritative durable outcome source
  because T003 bill list/detail responses retain only legacy numeric state.
- Accept both `state` and compatibility `status`; explicit state wins.
- Preserve `helpers.InvoiceStatus` as the fallback for absent/unknown monitor
  rows and all existing credit-note statuses.
- Keep retryability and artifact references as data fields; do not add a retry
  action without an approved frontend endpoint.
- Use only `accepted` and `accepted_with_warning` as issued.

## Validation

- `go test ./handlers -run 'Test.*SubmissionOutcome' -count=1`
- `gofmt -l` on changed Go files
- `go test ./handlers ./helpers ./config`
- `go test ./...`
- `git diff --check` and final isolated diff review

## Rollback

Revert the single T005 commit. No backend/daemon or database rollback is
required.
