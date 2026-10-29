# T032 Import Failure State

## Summary

Ensure imports with durable failure evidence cannot be displayed as
successfully posted or approved, while successful posted imports and existing
localization remain unchanged.

## User scenarios

### Scenario 1: Posted import has failure evidence

Given a posted import with a non-empty failure class or error-severity
validation evidence, the import display state is `failed`.

### Scenario 2: Posted import succeeds

Given a posted import with no failure evidence, the import display state
remains `posted` and uses the existing localized label.

## Requirements

- Failure evidence MUST override successful posted/approved display states.
- A valid posted import without failure evidence MUST remain posted.
- Raw lifecycle status, action permissions, templates, and localization MUST
  remain unchanged.
- A focused regression MUST be red on the base commit and green after the fix.

## Out of scope

Backend behavior, routes, persistence, evidence schema, localization changes,
and browser/live-storage validation.
