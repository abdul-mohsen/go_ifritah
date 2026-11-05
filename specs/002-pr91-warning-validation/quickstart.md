# Quickstart: PR91 Warning-Only Frontend Validation

## Prerequisites

- Go 1.21 toolchain and the repository's existing dependencies.
- Node.js and the dependencies already installed for `go_ifritah`.
- Worktree: `C:\ssda\chatGPT\ifritah\frontend.git\worktree\frontend-sale-identity-2`.

## Focused checks

Run from the worktree:

```powershell
rtk go test ./handlers -run 'TestHandleCreateCreditNote|TestHandleAddCreditNoteDisplaysVerifiedSource'
rtk npm run test:js
rtk git diff --check
```

Expected outcomes:

- Invalid-but-serializable credit-note fields reach the backend test server and
  backend rejection is returned.
- A corrected credit-note retry fetches fresh source instead of using a rejected
  cached payload.
- Purchase validation returns a warning category without mutating the payload.
- Existing JavaScript tests remain green.

## Broader checks

```powershell
rtk go test ./...
rtk go vet ./...
rtk go build ./...
rtk npm run lint:go
```

The package script `npm run format:check` may have baseline Windows glob behavior
in this worktree; record the exact result rather than treating a baseline failure as
a feature regression.
