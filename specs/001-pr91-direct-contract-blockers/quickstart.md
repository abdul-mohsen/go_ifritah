# Quickstart: PR #91 Direct Contract Blockers

Run focused checks from each worktree after implementation.

## Frontend

```powershell
Set-Location C:\ssda\chatGPT\ifritah\frontend.git\worktree\frontend-sale-identity-2
go test ./handlers -run 'Test.*(Credit|Purchase|SaleIdentity|QuickDraft)'
node --test js-tests\sale-identity.test.js js-tests\purchase-import-dates.test.js
```

Expected: serialized sale/draft/credit/purchase requests contain required fields; invalid imports and unverifiable credit source fail visibly; date boundary and history-entry identity regressions pass.

## Backend #105

```powershell
Set-Location C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b5-atomic-sale
go test ./pkg/handlers -run 'Test.*(Sale|Discount)'
```

Expected: a nonzero line discount reaches the insert; the existing totals formula remains unchanged.

## Backend #108

```powershell
Set-Location C:\ssda\chatGPT\ifritah\backend.git\worktree\backend-b6-notes-2
go test ./pkg/handlers -run 'Test.*(Credit|CreditSource)'
```

Expected: the source endpoint returns only the exact full remaining amount and line quantities, and refuses unprovable or unavailable data. The credit POST continues to enforce concurrent/stale snapshots.

## Delivery checks

Run the repository-required Go/JS checks for changed packages, review all three diffs, run Spec Kit analysis/convergence, obtain approval from `copilot-pr91-qa`, then non-force push to the existing PR branches and verify their GitHub head SHAs.
