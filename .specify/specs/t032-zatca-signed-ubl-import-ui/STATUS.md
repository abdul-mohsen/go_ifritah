# T032 Status

## Completed

- T032-REQ: scoped requirements, research, design, plan, spec, and tasks
  created from frontend `d7b5034` and backend T031 `94aa8e2`.
- Repository scope confirmed: frontend worktree only; backend worktree is
  read-only reference.
- T032-RED: focused handler/template tests failed before implementation and
  pass after the import handlers/views were added.
- T032-HANDLER: authenticated list/detail and T031 create/list/detail/review/
  post/reject proxy handlers implemented.
- T032-TEMPLATES: upload/list/detail/status/evidence/review templates and
  localized labels implemented with explicit pending/failed/rejected/posted
  states and no issued-state mapping.
- T032-ROUTES: singular/plural authenticated page and API aliases registered.
- T032-GREEN: focused handlers/config template tests and `go test ./...`
  pass.
- No frontend README existed at baseline; no existing README target applies.

## In Progress

- T032-COMMIT: creating the isolated local commit; no push or PR.

## Blocked

- None currently.

## Backlog

- None.

## Risks

- Browser File API, CSRF middleware, and live MySQL/object-storage durability
  cannot be proven by local browserless tests.
- Raw XML upload is persisted as review-required evidence; T030 parsing and
  cryptographic verification remain outside this frontend.

## Next Actions

1. Commit the validated frontend-only T032 changes locally; do not push or
   create a PR.
