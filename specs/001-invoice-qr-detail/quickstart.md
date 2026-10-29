# Quickstart Validation: Invoice Detail ZATCA QR Rendering

## Prerequisites

- Go 1.21 toolchain available.
- Worktree is `C:\ssda\chatGPT\worktrees\zatca-fix-t021-frontend`.
- Branch is `fix/zatca/h7-invoice-qr-detail`.

## Red-First Regression

From the repository root, run the focused test immediately after adding the
test and before the handler change:

```powershell
rtk go test ./handlers -run TestHandleGetInvoiceRendersPersistedZATCAQR -count=1
```

Expected result: failure because the handler does not yet include `qr_code` in
the template data.

## Green Validation

Format changed Go files and rerun the focused test:

```powershell
rtk gofmt -w handlers\invoice_detail_qr_test.go handlers\invoices.go
rtk go test ./handlers -run TestHandleGetInvoiceRendersPersistedZATCAQR -count=1
```

Expected result: persisted QR/artifact assertions pass; empty QR does not emit
QR initialization.

Run related and full validation:

```powershell
rtk go test ./handlers ./config -count=1
rtk go test ./...
```

Expected result: all commands pass, or any environment limitation is recorded
in `STATUS.md` and the final report.

## Review Checks

- `rtk git diff --check`
- Confirm no monitor, onboarding, or outcome schema files changed.
- Confirm no push or pull request was performed.
