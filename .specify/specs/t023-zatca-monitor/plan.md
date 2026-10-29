# T023 Implementation Plan

1. Add the red authenticated handler/template regression test.
2. Replace fixed monitor data with concurrent authenticated calls to the three
   documented backend endpoints.
3. Map nullable contract values and explicit statuses into safe template data.
4. Update the localized template for loading/error/empty states, warnings,
   pending age, and provider references.
5. Run formatting and focused/full Go tests, converge against the artifacts,
   self-review the frontend-only diff, and commit without pushing.
