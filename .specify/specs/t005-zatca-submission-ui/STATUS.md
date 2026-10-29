# Status: T005 Frontend C2 Submission Outcome UI

## Completed

- T005-REQ: repository baseline and approved T003/T004 contract researched.
- T005-REQ: requirements, research, design, tasks, and status artifacts created.
- T005-DOC: no root `README.md` exists in the frontend repository; the
  user-facing behavior is documented in this feature artifact and no unrelated
  README will be created.
- T005-RED: added `TestHandleInvoicesUsesDurableSubmissionOutcomeForStatusAndActions`.
- T005-RED: confirmed red at base `18f7988`; the handler ignored monitor rows,
  so the test could not find the rejected durable label and would have kept
  issued-only credit actions.
- T005-MODEL/T005-CLIENT: added the frontend monitor adapter, six-state
  normalization, compatibility aliases, and invoice-ID outcome index.
- T005-HANDLERS: merged durable outcome fields into invoice list/detail data;
  only accepted and accepted-with-warning set `is_issued`.
- T005-TEMPLATE: added localized durable labels, warning/detail metadata, and
  non-success credit-action guards.
- T005-GREEN: focused list/detail tests and `go test ./...` pass; `gofmt -l`
  reports no output for changed Go files.
- T005-CONVERGE: final review found no additional T005 implementation work;
  backend/daemon files are not in the frontend diff.
- T005-DOC: no repository root `README.md` exists; feature behavior and the
  contract limitation report are documented in this scoped Spec Kit directory.

## In Progress

- None.

## Blocked

- None.

## Backlog

- None.

## Risks

- Older backend deployments may not expose the monitor route; implementation
  must preserve legacy invoice rendering on monitor-read failure.
- The approved contract does not define a frontend retry endpoint, so retry
  metadata can be displayed but must not create an invented action.
- T003's monitor route is admin-only and the bill list/detail contract still
  exposes only legacy numeric state; non-admin invoice viewers or older
  deployments therefore fall back to legacy status until the backend exposes
  tenant-authorized invoice outcome data.

## Next Actions

- None; T005 is complete in the local isolated commit. Do not push or create a
  PR.
