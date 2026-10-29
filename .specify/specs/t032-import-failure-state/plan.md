# T032 Import Failure State Plan

## Implementation approach

1. Create and validate the scoped requirements, research, design, task, and
   status artifacts.
2. Add the focused posted failure-state regression without changing
   production code.
3. Run that exact regression on commit `afc6b71` and record the red result.
4. Extend the existing display-state failure override to include `posted`.
5. Run gofmt on touched Go files, targeted handler/template tests, the full Go
   suite, and `git diff --check`.
6. Reconcile artifacts, update the SQL todo, and create one local commit with
   the required co-author trailer.

## Scope and trade-offs

- Keep the existing evidence definition and localization.
- Change one conditional rather than introducing a new abstraction.
- Do not alter action gating or raw backend status.

## Validation gate

The implementation is ready to commit only when the focused regression,
targeted tests, full `go test ./...`, formatting, diff check, todo update, and
self-review all pass.
