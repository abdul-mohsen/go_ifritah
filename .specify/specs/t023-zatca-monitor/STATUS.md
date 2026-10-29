# Status

## Completed

- T023-REQ: inspected baseline frontend monitor implementation, auth helpers,
  template error pattern, localization, and sibling backend monitor contract.
- T023-REQ: requirements, research, design, and dependency-ordered tasks
  recorded.
- T023-TEST: focused authenticated backend regression was red against the
  baseline mock implementation.
- T023-API: wired authenticated stats, branch, and submission endpoint calls
  with contract mapping, nullable fields, explicit states, and partial errors.
- T023-UI: added localized loading/error/empty states, status badges, warning
  details, pending age, provider references, and safe unavailable markers.
- T023-TEST: focused regression, template parse/execute, and all handler tests
  are green.
- T023-VERIFY: final `go test ./...`, formatting checks for changed monitor Go
  files, and staged diff scope checks are green. No `README.md` exists in the
  target frontend repository; the feature artifacts are the applicable
  documentation update.

## In Progress

- None.

## Blocked

- None. The documented backend monitor contract is available in the sibling
  backend worktree.

## Backlog

- None.

## Risks

- Deployed backend versions may provide either `warnings` or
  `accepted_with_warning`; decode both without double-counting.
- The monitor contract does not provide invoice type; the UI must not invent
  one.

## Next Actions

1. Commit the reviewed frontend-only T023 change; do not push or open a PR.
