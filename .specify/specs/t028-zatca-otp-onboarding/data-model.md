# T028 Data and State Model

## Client state

| State | Meaning | Allowed onboarding action |
|---|---|---|
| `zatcaConfigSaved = false` | Configuration is dirty or a prerequisite failed | Disabled |
| `zatcaSavePending = true` | Branch config or linked store-address save is in flight | Disabled |
| `zatcaConfigSaved = true` and `zatcaSavePending = false` | Both prerequisite saves succeeded | Enabled when fields/validation pass |
| Onboarding request pending | OTP has been submitted to the existing endpoint | Existing OTP submit control disabled |

## Request relationships

- One branch ZATCA config save precedes one linked store-address save.
- The linked store-address save is required for the complete save result.
- The existing onboarding request is allowed only from the complete-success
  state.

No persisted schema or backend entity changes are part of T028.
