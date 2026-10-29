# T012 Plan

## Implementation sequence

1. Extend the frontend ZATCA response model and settings-page data map with
   T010/T011 onboarding metadata.
2. Add focused handler/template regression tests and run them red against
   commit `55a5024`.
3. Add localized labels and update the settings template to render durable
   progress/failure/retry/completion state, protect active status, and stop
   terminal polling.
4. Run gofmt, targeted tests, full Go tests, diff/scope checks, and self-review.
5. Update status/convergence artifacts and create one isolated local commit.

## Decisions

- Retain the numeric status map for compatibility; add a separate onboarding
  metadata map.
- Use durable state as the source of truth when it is present.
- Prefer localized error-code labels over provider text.
- Do not edit T010/T011 repositories or add browser dependencies.

