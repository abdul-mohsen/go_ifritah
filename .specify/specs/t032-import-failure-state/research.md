# T032 Import Failure State Research

## Evidence inspected

1. `handlers/zatca_import.go` derives `hasFailure` from a non-empty
   `evidence.failure_class` or an error-severity validation error.
2. The same handler maps `review` to `pending`, then maps failure evidence to
   `failed` only for `review` and `approved`. A raw `posted` status therefore
   remains `posted` even when failure evidence is present.
3. `handlers/zatca_import_ui_test.go` already verifies failure evidence and
   action suppression for a review import, but has no posted-with-failure
   regression.
4. The T032 design explicitly separates raw lifecycle status from derived
   display status and defines `posted` as import workflow completion, not an
   outbound issued invoice.
5. No frontend README exists in the target worktree. The affected behavior is
   an internal display contract; no user-facing README update applies.

## Decisions

### Decision 1 — Extend the existing failure display rule to posted

Use the existing failure-evidence calculation and include raw `posted` in the
same failed-display override already used for `review` and `approved`.

**Requirement linkage:** FR-001, FR-002, FR-003.

### Decision 2 — Do not broaden the rule to every raw status

The smallest explicit rule targets the successful states called out by the
contract (`approved` and `posted`) while preserving existing `review`
handling. It avoids changing `rejected` or unknown-status behavior without a
requirement.

**Requirement linkage:** FR-003, FR-004.

### Decision 3 — Test both sides of the posted boundary

The regression covers a posted import with failure evidence and a posted
import without failure evidence. This proves both the defect fix and the
required preservation of valid successful posted imports.

**Requirement linkage:** FR-002, FR-005.

## Remaining uncertainty

None blocking. The backend evidence vocabulary is already represented by the
T032 frontend types and no new fields are required.
