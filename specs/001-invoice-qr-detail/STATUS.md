# Status

## Completed

- T021-R: repository research identified `qr_code` retained by
  `ParseBillRaw` but omitted by `HandleGetInvoice`.
- T021-R: requirements, research, design, tasks, and spec artifacts created.
- Documentation review: this frontend has no root `README.md`; no separate
  user-facing documentation update applies. Behavior is documented in these
  feature artifacts.
- T021-T: focused regression test added and confirmed red before the fix.
- T021-I1/I2/I3/I4: persisted QR, optional artifact/reference data, safe empty
  state, and localization wired through the invoice detail path.
- T021-V1: changed Go files formatted with `gofmt`.
- T021-V2: focused regression passed.
- T021-V3: `go test ./handlers ./config -count=1` passed (272 tests).
- T021-V4: `go test ./...` passed (398 tests).
- T021-C1: convergence review found no remaining implementation gaps; final
  scope review is complete and the local commit is ready.

## In Progress

- None.

## Blocked

- None.

## Backlog

- None.

## Risks

- Backend responses may omit optional artifact/reference fields; rendering must
  remain conditional.
- QR content must remain contextual-escaped and must never be fabricated by the
  frontend.

## Next Actions

- None before handoff; do not push or create a pull request.

## Validation Log

- Red-first: `go test ./handlers -run TestHandleGetInvoiceRendersPersistedZATCAQR -count=1`
  failed as expected because `persisted-zatca-qr` was absent from rendered
  invoice detail HTML.
- Green: `go test ./handlers -run TestHandleGetInvoiceRendersPersistedZATCAQR -count=1`
  passed.
- Related: `go test ./handlers ./config -count=1` passed.
- Full: `go test ./...` passed.
