# T012 Frontend ZATCA Onboarding State UI Specification

The frontend settings page consumes the durable T010/T011 onboarding contract
and presents localized progress, failure, retry, and completion feedback
without changing backend or daemon behavior. The page never labels a queued,
in-progress, or failed attempt as active, and it stops polling on terminal
outcomes while preserving T028's save-before-OTP ordering.

The normative requirements, scenarios, constraints, and acceptance criteria
are recorded in `requirements.md`.

