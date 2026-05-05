# QA suite report (parallel run, 211.5s wall)

| Suite | Exit | Seconds | Result |
|---|---|---|---|
| playwright-e2e | 1 | 87.3 | 2 failed test(s) |
| go-test | 0 | 47 | 0 failed test(s) |
| go-vet | 0 | 36.5 | 0 vet warning(s) |
| pa11y | 0 | 70 | 13 route(s) with issues |
| lighthouse | 0 | 124 | 4 route(s) below 90 |
| htmlvalidate | 0 | 1.3 | 13 route(s) with issues |
| animation-flash | 0 | 16.6 | 0 route(s) with flicker |


## playwright-e2e (exit=1)
- log: `quality-gate/logs/playwright-e2e.log`
- 2 failed test(s)

- ✘  127 [parallel] › tests\zatca-settings.spec.js:28:1 › ZATCA connect button is disabled when fields are empty (9.3s)
- [parallel] › tests\zatca-settings.spec.js:28:1 › ZATCA connect button is disabled when fields are empty

## go-test (exit=0)
- log: `quality-gate/logs/go-test.log`
- 0 failed test(s)

## go-vet (exit=0)
- log: `quality-gate/logs/go-vet.log`
- 0 vet warning(s)

## pa11y (exit=0)
- log: `quality-gate/logs/pa11y.log`
- 13 route(s) with issues

- home → 4 error(s)
- login → 4 error(s)
- dashboard → 31 error(s)
- invoices → 11 error(s)
- products → 11 error(s)
- clients → 8 error(s)
- suppliers → 9 error(s)
- branches → 6 error(s)
- stores → 6 error(s)
- orders → 8 error(s)
- purchasebills → 12 error(s)
- cashvouchers → 12 error(s)
- settings → 77 error(s)

## lighthouse (exit=0)
- log: `quality-gate/logs/lighthouse.log`
- 4 route(s) below 90

- dashboard → a11y=87
- invoices → perf=84
- purchasebills → perf=87
- settings → a11y=82

## htmlvalidate (exit=0)
- log: `quality-gate/logs/htmlvalidate.log`
- 13 route(s) with issues

- home → 8 error(s)
- login → 8 error(s)
- dashboard → 269 error(s)
- products → 61 error(s)
- clients → 56 error(s)
- suppliers → 59 error(s)
- branches → 57 error(s)
- stores → 57 error(s)
- orders → 57 error(s)
- purchasebills → 65 error(s)
- cashvouchers → 61 error(s)
- notifications → 58 error(s)
- settings → 135 error(s)

## animation-flash (exit=0)
- log: `quality-gate/logs/animation-flash.log`
- 0 route(s) with flicker