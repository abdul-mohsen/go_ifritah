# Implementation Plan: Invoice Detail ZATCA QR Rendering

**Branch**: `fix/zatca/h7-invoice-qr-detail` | **Date**: 2026-10-29 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/001-invoice-qr-detail/spec.md`

## Summary

Restore the missing `qr_code` handler-to-template mapping, pass through optional
persisted ZATCA artifact/reference values, and make the empty QR state explicit
without emitting QR initialization code. Keep existing status helpers,
localization, authentication, and backend error handling unchanged.

## Technical Context

**Language/Version**: Go 1.21

**Primary Dependencies**: Go `html/template`, Gorilla Mux, existing `helpers`
and `resources` packages

**Storage**: Backend-owned invoice persistence accessed through the existing
authenticated bill-detail request

**Testing**: `go test`, `gofmt`, existing handler/config template tests

**Target Platform**: Server-rendered web frontend

**Project Type**: Go web application

**Performance Goals**: No additional backend request; preserve the existing
single detail fetch path

**Constraints**: No backend schema/API changes, no monitor/onboarding/outcome
work, preserve contextual escaping and localization

**Scale/Scope**: One invoice detail handler, one page template, one localization
label, and focused regression coverage

## Constitution Check

The generated constitution still contains only placeholders and therefore has
no project-specific normative gates. Repository instructions require red-first
testing, gofmt, targeted tests, full Go tests, isolated changes, and no push or
PR; these gates are included in `tasks.md`.

## Phase 0: Research

- Confirmed `helpers.ParseBillRaw` already retains `qr_code` in `extra`.
- Confirmed `HandleGetInvoice` omits `qr_code` from the template map.
- Confirmed the template already conditionally initializes QR rendering and uses
  the localized `tpl.invoice.qr_code` label.
- Confirmed optional artifact/reference fields can be passed through without
  changing frontend models or backend schemas.

## Phase 1: Design

### Data flow

`FetchBillRaw` → `ParseBillRaw` → `HandleGetInvoice` template map →
`invoice-detail.html`. The handler normalizes optional strings with
`helpers.SafeString`; the template conditionally renders the QR and optional
metadata.

### Error and safety behavior

Missing, null, or non-string QR values become empty strings. The QR script stays
inside the non-empty conditional. Existing authentication, backend error, status
and type behavior remains unchanged.

### Artifacts

- [data-model.md](data-model.md) defines the optional view data.
- [quickstart.md](quickstart.md) defines red-first and final validation.
- No external contract file is required because no API contract changes.

## Project Structure

### Documentation

```text
specs/001-invoice-qr-detail/
├── requirements.md
├── research.md
├── design.md
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── tasks.md
├── STATUS.md
└── checklists/requirements.md
```

### Source Code

```text
handlers/
├── invoices.go
└── invoice_detail_qr_test.go
helpers/
└── api_helpers.go
resources/
└── labels.go
templates/
└── invoice-detail.html
```

**Structure Decision**: Keep the existing server-rendered frontend layout. The
handler remains responsible for mapping backend data; the template remains
responsible for localized conditional presentation. No new package or endpoint
is warranted.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| None | N/A | This is a narrow data-path correction within existing components. |
