# Warning-Only Validation Requirements Checklist

**Purpose**: Review the quality and completeness of the warning-only validation requirements
**Created**: 2026-11-05
**Feature**: [spec.md](../spec.md)

**Ownership**: Reviewer-owned requirements checklist. `[x]` means the requirement
quality criterion is approved; it does not mean implementation is complete.

## Requirement Completeness

- [ ] CHK001 Are warning-only requirements defined separately for purchase imports and credit notes? [Completeness, Spec §FR-001, §FR-004]
- [ ] CHK002 Are the hard-failure conditions that may remain blocking explicitly bounded? [Completeness, Spec §FR-006]
- [ ] CHK003 Are backend rejection and partial-success outcomes specified for every warning-only submission path? [Coverage, Spec §FR-003, §FR-005]

## Requirement Clarity

- [ ] CHK004 Is "serializable request" defined clearly enough to distinguish a business-invalid value from an impossible or unsafe request? [Clarity, Spec §FR-001, §FR-006]
- [ ] CHK005 Is "unchanged payload" defined for both user-entered values and generated request identity? [Clarity, Spec §FR-002, §FR-007]
- [ ] CHK006 Is the difference between a user-visible warning and a backend rejection unambiguous? [Clarity, Spec §SC-005]

## Requirement Consistency

- [ ] CHK007 Are warning-only rules consistent with the requirement that backend validation remains authoritative? [Consistency, Spec §FR-003 through §FR-006]
- [ ] CHK008 Are recovery requirements consistent with preserving retry identity and clearing only definitive rejected attempts? [Consistency, Spec §FR-007, §SC-003]

## Acceptance Criteria Quality

- [ ] CHK009 Can the purchase warning outcome be objectively distinguished from a structural preview failure? [Measurability, Spec §AC-1]
- [ ] CHK010 Can the credit-note outcome prove that the backend was reached before a rejection was shown? [Measurability, Spec §AC-2]
- [ ] CHK011 Can payload preservation be measured without relying only on the warning text? [Measurability, Spec §AC-3]

## Scenario and Edge-Case Coverage

- [ ] CHK012 Are blank, over-limit, malformed-identity, corrected-retry, and source-unavailable credit-note cases covered? [Coverage, Spec §User Story 2, §User Story 3]
- [ ] CHK013 Are mixed valid/invalid purchase rows and partial backend success covered? [Coverage, Spec §User Story 1]
- [ ] CHK014 Are malformed upload, missing mapping, request-size, authentication, and CSRF cases intentionally separated from business warnings? [Edge Case, Spec §FR-006]

## Non-Functional Requirements

- [ ] CHK015 Are warning accessibility and Arabic/English localization requirements explicit for every new warning surface? [Completeness, Spec §Non-functional requirements]
- [ ] CHK016 Are retry and control-state requirements clear enough to rule out a permanently disabled submission control? [Reliability, Spec §Non-functional requirements]

## Dependencies and Assumptions

- [ ] CHK017 Are backend response-shape assumptions documented and linked to the paths that render those responses? [Dependency, Spec §Assumptions]
- [ ] CHK018 Is the exclusion of sales-import gating and durable purchase-import identity explicit enough to prevent scope creep? [Scope, Spec §Out of Scope]

## Notes

- This checklist tests requirements quality, not implementation behavior.
- The repository has no README.md; the feature artifacts and `STATUS.md` record the
  user-facing behavior for this focused revision.
