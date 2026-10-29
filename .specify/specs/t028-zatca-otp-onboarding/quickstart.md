# T028 Validation Guide

Run commands from `C:\ssda\chatGPT\worktrees\zatca-fix-t028-frontend`.

## Baseline regression

1. Add the focused test from task `T028-RED`.
2. Run:

   ```text
   go test ./config -run T028
   ```

3. Confirm it fails because the baseline template fires the store save and
   dirty-config save without awaiting them.

## Production validation

1. Apply the template change.
2. Format changed Go files:

   ```text
   gofmt -w config/settings_zatca_flow_test.go
   ```

3. Run the focused test and template parse tests:

   ```text
   go test ./config -run T028
   go test ./config
   ```

4. Run the full suite when practical:

   ```text
   go test ./...
   ```

5. Review:

   ```text
   git diff --check
   git status --short
   ```

Expected result: the focused regression and relevant Go tests pass, and the
diff contains only T028 frontend/template/test/spec artifacts.

## Recorded results

- Baseline focused regression: failed as expected before the production edit
  because the pending/awaited-save contract was absent.
- Focused regression after the edit: passed.
- `go test ./config ./handlers`: passed (272 tests).
- `go test ./...`: passed (398 tests across 7 packages).
- `go vet ./...`: passed.
- `npx prettier --check templates/settings.html`: passed.
- The package-level `npm run format:check` command is not portable in this
  Windows shell because its single-quoted glob is passed literally; the
  changed template was checked directly instead.
- No browser/runtime automation was run; the requested regression is
  browserless and Go/template based.
