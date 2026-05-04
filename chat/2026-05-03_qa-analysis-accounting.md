# Senior-QA accounting analysis — Afrita-Go

**Author:** QA pass synthesizing accounting-domain experience, ZATCA Phase 2
e-invoicing rules (https://zatca.gov.sa/en/E-Invoicing/), ISA 315 control
testing patterns, and a manual scan of the existing `e2e/tests/*.spec.js`
coverage.

**Audience:** team merging the cursor-pagination + BE-driven-search PRs.

**Goal:** identify the use cases that *actually matter* for an accounting
SaaS and that are currently uncovered or under-covered, and ship them as
runnable Playwright tests against the local FE+BE.

---

## 1. Why a fresh accounting-lens pass is needed

The existing 38-spec, ~260-test suite is broad on **integration plumbing**
(login, page renders, RBAC, ZATCA settings, edit-form round-trip) but
narrow on **accounting truth**. Three concrete examples:

1. `qa-11-bill-form` creates a bill but never asserts that
   `total = subtotal + VAT − discount`. A bill that displays `total =
   NaN SAR` would still pass.
2. `qa-05-list-pages` confirms list URLs return < 400 status but never
   inspects the rendered cells. A row showing
   `<no value>` for an unset Go template field, or `[object Object]`
   from a typo'd JS render, would slip through.
3. `qa-20-list-filters` tests state-based filtering correctness but
   doesn't probe the actual BE-driven search contract we just landed
   (Arabic queries, special chars, cursor preservation).

The user-facing impact of each gap is non-trivial. An accountant
reviewing a 30-line bill in `8.0% trans rounding mode` who sees
`-0.00 SAR` in the discount column has every reason to mistrust the
software for the rest of their year. So this pass is targeted at the
shapes of bug that *erode operator confidence* — math, formatting,
and the leak of internal sentinel values into the human view.

## 2. Domain reference

Saudi Arabia VAT rate has been 15 % since 1 July 2020 (ZATCA
[VAT page](https://zatca.gov.sa/en/RulesRegulations/Taxes/Pages/Vat.aspx)).
ZATCA Phase 2 e-invoicing (effective 1 Jan 2023, rolled out in waves)
mandates that **every** issued tax invoice + credit note + debit note
carries:

- seller VAT registration number (15-digit, starts and ends with `3`),
- buyer VAT (mandatory for B2B over SAR 1,000),
- a structured XML body with line items,
- a cryptographic stamp + QR code on the rendered PDF,
- a credit/debit note linked back to the parent invoice via UUID.

Practical implications for our test suite:

| Concern | What QA must prove |
|---|---|
| Invoice math | `sum(line.qty × line.unit_price) = subtotal`, `subtotal × vat_rate = vat_amount` (rounding to 2 dp), `total = subtotal + vat − discount` |
| Credit notes | Always linked to a parent invoice id; the credit must be a *separate* row (BE just confirmed this in `chat/2026-05-03_backend-to-frontend_bill-credit-note-correction.md`); displayed as a negative-signed entry in supplier/client statements |
| State machine | bill: draft (0) → issued (1) → paid (2) → ZATCA-issued (3); ZATCA submission is a *one-way door* — once state=3 you cannot go back to draft |
| Cash voucher | debit/credit balance must match the bill it pays; a voucher in `posted` cannot be edited |
| Stock | After a sale-bill posts, on-hand `qty` decreases by line qty; after a purchase-bill posts, it increases. Negative stock is gated by the `stock_enforcement` setting |
| Numeric formatting | Saudi convention uses Western-Arabic numerals (`1,234.56`) with comma thousands separator, `.` decimal, two decimal places for currency. Negative amounts wrap in parentheses on statements |
| Date | All accounting periods are anchored to Asia/Riyadh (UTC+3, no DST). A bill dated 23:59 Riyadh time on 30 Jun must not slip into the July VAT return |
| Internationalization | UI is RTL Arabic primary, Latin secondary. Search must accept Arabic without normalisation that would break the BE's UTF-8 LIKE clause |

## 3. Use-case catalogue

Each use case is written in the "user story → acceptance criteria"
format common in accounting test plans (cf. ISA 315 §A126, "evaluating
whether the financial reporting framework is properly applied"). Each
has a stable id (`UC-NN`) referenced in the test files.

### UC-01 — Search: BE is the only filter

> **As an** accountant searching a 50 000-row supplier ledger
> **I want** the search to be processed by the database
> **So that** typing 4 characters does not pin my browser tab while the
> FE filters in memory.

Acceptance:
- FE forwards `?q=…` to BE in the request body as `query`.
- FE renders whatever rows the BE returned, no post-filter.
- Empty `?q=` does NOT send a `query` field (different from `query=""`).
- A query with Arabic characters round-trips byte-for-byte.
- A query with `% & " '` survives JSON encoding and URL encoding.

Existing coverage: handler-level unit tests (added in this PR,
`handlers/search_feature_test.go`). Missing at e2e level — added in
`qa-34-search-deep.spec.js`.

### UC-02 — Search: pagination preserves the query

> **As an** accountant who searched "Al-Faisal" and is now on page 3
> **I want** the next/prev links to keep the query
> **So that** I'm not silently kicked back to an unfiltered list when I
> click "next".

Acceptance:
- After `?q=foo`, the rendered "next" link contains both `q=foo` and
  the cursor.
- Clicking next yields a page where the search input still shows "foo".
- Cursor is opaque base64-url; the FE never decodes it.

Existing coverage: none. Added.

### UC-03 — Search: empty-state UX

> **As an** accountant who mistyped a search
> **I want** to see "no results" rather than a blank page
> **So that** I know the page rendered correctly.

Acceptance:
- `?q=ZZZ-NEVER-MATCHES-…` returns a 200 page that contains a
  recognisable empty-state element (placeholder row, message, or empty
  table body) — not a JavaScript exception, not a Go template error.
- The search input still echoes the query so the user can edit it.

### UC-04 — HTML hygiene: no internal sentinels reach the view

> **As any** end user
> **I want** to never see `NaN`, `undefined`, `null`, `[object Object]`,
> `<no value>`, or template tokens like `{{` in a rendered page
> **So that** I trust the system.

Acceptance:
- Every list page (invoices, purchase-bills, cash-vouchers, products,
  clients, suppliers, orders, branches, stores) renders without any of
  the forbidden tokens.
- The dashboard renders without any of the forbidden tokens.
- Settings, ZATCA monitor and the credit-note form follow the same rule.

This is a high-leverage test: a single regression check guards against
the most embarrassing kind of UI bug (the kind your customer
screenshots and posts on Twitter). Existing coverage: none. Added.

### UC-05 — Numeric column safety

> **As an** accountant scanning a bill list
> **I want** every cell that *should* be a number to either be a number
> or be empty
> **So that** I never see "NaN SAR" or "undefined".

Acceptance: for every list with a known numeric column (invoices total,
purchase-bills total, cash-vouchers amount, products quantity, products
price), every non-empty cell parses as a finite number after stripping
the currency suffix and thousands separators.

Existing coverage: none. Added.

### UC-06 — Currency formatting on the visible columns

> **As an** accountant
> **I want** monetary values to render as `1,234.56` (or `0.00`) with
> exactly 2 decimals
> **So that** my eyes can scan a column.

Acceptance: visible monetary cells match the regex
`^\d{1,3}(,\d{3})*\.\d{2}\s*(SAR|ر\.س|ريال)?$` (Western digits, comma
grouping, dot decimal, two-decimal places, optional Saudi currency
symbol). Negative values use a leading minus, not parentheses
(matches the existing FE convention).

This is the "soft" formatting check; UC-07 below adds the hard math.

### UC-07 — Bill math: total = subtotal + VAT − discount

> **As an** auditor reading a printed invoice
> **I want** the total to equal subtotal + VAT − discount within 1 halala
> (0.01 SAR)
> **So that** the invoice balances.

Acceptance: For each issued invoice (`state ≥ 1`) shown on the
detail page, parse `subtotal`, `vat_amount`, `discount`, `total`,
verify `|total − (subtotal + vat − discount)| ≤ 0.01`.

Edge cases worth listing in the doc even when they're not testable
without seeded data (low-volume dev DB):

- discount > subtotal must be rejected at form submit (cannot create
  a negative-total bill outside an explicit credit note),
- VAT rate 0 % (zero-rated supply, e.g. exports) → `vat_amount = 0`,
- VAT rate 15 % → `vat_amount = round(subtotal × 0.15, 2)`,
- mixed-VAT bill (some lines exempt, some 15 %) — out of scope for v1.

### UC-08 — VAT precision (15 %, banker's rounding caveat)

> **As an** accountant
> **I want** VAT to be computed with 2-decimal precision
> **So that** the e-invoice XML matches the displayed PDF.

Acceptance: For a bill with subtotal `S`, expected VAT = `Math.round(S
* 0.15 * 100) / 100`. If displayed VAT differs by > 0.01 SAR, the bill
fails the test.

Per ZATCA Phase 2 XML implementation standard, the rule is
half-away-from-zero (commercial rounding), not banker's — but the FE
just renders what the BE computed, so we only check absolute
difference, not the rounding mode itself.

### UC-09 — Credit-note linkage and sign

> **As an** accountant viewing a client statement
> **I want** a credit note to appear as a separate, signed entry
> linked to its parent bill
> **So that** the running balance is correct.

Acceptance: On the invoices list, credit-note rows are visible *and*
distinct from their parent bills (BE just clarified in the
3-element-bill-cursor handoff that they are separate rows on
purpose). The credit row's bill number references the parent
sequence number, and the displayed total on the credit row is
non-positive (zero or negative).

### UC-10 — Cash-voucher state machine

> **As a** treasurer
> **I want** voucher transitions to be enforced server-side
> **So that** a posted voucher cannot be silently edited.

Acceptance: from `qa-16` we already test the happy path. This pass
adds:

- a posted voucher's row in the list shows the "posted" badge with the
  expected Arabic label, and the row's edit link is absent.
- attempting to navigate directly to `/dashboard/cash-vouchers/edit/{id}`
  for a posted voucher returns 200 with a read-only form (or 403 with
  a recognisable error page) — *never* a 500.

### UC-11 — Stock movement after bill posting

> **As a** warehouse manager
> **I want** the on-hand quantity to decrease by exactly the sale qty
> **So that** my pickers don't oversell.

Acceptance: snapshot product qty, post a sale-bill of qty `n`,
re-fetch — qty must drop by exactly `n`. Out of scope for the local
dev DB (no seeded sale path that doesn't depend on ZATCA), so we
list it here and test it manually for now.

### UC-12 — RBAC: search input is visible to every role

> **As any** authenticated user
> **I want** the search input on every list page I can reach
> **So that** the affordance is consistent.

Acceptance: as `admin`, `manager`, `employee`: every list page that
renders ≥ 200 also renders an `input[name="q"]`. (Pages a role can't
reach are excluded.)

Existing coverage: `qa-18-rbac` tests page reachability. We extend
that with the search-input affordance check.

### UC-13 — Unicode and bidirectional safety

> **As an** Arabic-speaking user
> **I want** Arabic search and Arabic display to "just work"
> **So that** the app respects my language.

Acceptance: the search box can submit `جدة` and `عميل تجريبي` and the
echoed value in the input is the same Arabic. The list page does not
render any LTR isolate or "REPLACEMENT CHARACTER" (`U+FFFD`) bytes in
visible text.

### UC-14 — Error containment: a single bad row doesn't 500 the page

> **As an** accountant whose seed data has one row with a corrupt
> `effective_date`
> **I want** the rest of the list to render
> **So that** I can fix the bad row.

Acceptance: even on the demo DB which has known
ZATCA-not-configured rows, `/dashboard/invoices` returns 200 and at
least the page chrome (header, sidebar, search input) renders.

This already passes; we keep it as a regression guard so the next
time someone adds a panic-prone template helper, the spec catches it.

### UC-15 — Pagination boundary cases

> **As an** accountant who clicked "next" and then refreshed
> **I want** the URL to still resolve
> **So that** I don't get a confusing error.

Acceptance:
- `cursor=` (empty, explicit) renders page 1.
- `cursor=garbage` is treated like an invalid cursor: returns 200 with
  page 1 (or an empty page), never 500.
- `per=0` and `per=99999` are clamped server-side; the body sent to BE
  has `limit ≤ 50` (FE MaxLimit) regardless.

The handler-level test `TestSearchClampsPerAtMaxLimit` already proves
the body shape; this e2e test proves the page does the right visible
thing.

## 4. Comparison vs existing coverage

| UC | Existing | Now |
|---|---|---|
| UC-01 BE-only filter | partial (`qa-20`) — handler tests fully | + e2e: `qa-34` confirms FE forwards & doesn't double-filter |
| UC-02 search + pagination preserved | none | + `qa-34` |
| UC-03 empty-state UX | partial (`qa-20`) | + `qa-34` (extends to all 9 lists, asserts on visible empty-state element) |
| UC-04 HTML hygiene tokens | none | + `qa-33` — every list + dashboard + key forms |
| UC-05 numeric column safety | none | + `qa-33` — numeric cells parse-or-empty |
| UC-06 currency formatting | none | + `qa-33` — regex for visible monetary cells |
| UC-07 bill math identity | none | + `qa-35` — invoice detail subtotal + vat − discount = total |
| UC-08 VAT 15 % precision | none | + `qa-35` |
| UC-09 credit-note sign | partial (`qa-17`) | + `qa-35` confirms list shape |
| UC-10 voucher state machine | strong (`qa-16`) | unchanged |
| UC-11 stock after bill | none | unchanged (out of scope without seeded data) |
| UC-12 search input on every list / role | none | + `qa-33` covers admin; role pass extends `qa-18` |
| UC-13 Arabic & bidi safety | partial (`qa-22`) | + `qa-34` |
| UC-14 error containment | strong (`qa-02`) | unchanged |
| UC-15 pagination boundary | none | + `qa-34` |

## 5. Test layout shipped in this PR

- `e2e/helpers/html-hygiene.js` — pure functions:
  `forbiddenTokens()`, `assertNoForbiddenTokens(text, where)`,
  `assertNumericCells(page, selector)`, `formatRegex()`.
- `e2e/tests/qa-33-html-hygiene.spec.js` — UC-04, UC-05, UC-06, UC-12.
- `e2e/tests/qa-34-search-deep.spec.js` — UC-01, UC-02, UC-03, UC-13,
  UC-15.
- `e2e/tests/qa-35-bill-math.spec.js` — UC-07, UC-08, UC-09.

All three new spec files are marked `parallel` (no shared mutable
state).

## 6. Out-of-scope follow-ups

These are real concerns I'd push to a follow-up PR rather than try to
shoehorn into the current scope:

- **Stock movement** (UC-11) needs seeded products with known qty;
  bring up a dedicated test fixture.
- **PDF QR code content** verification — Phase 2 ZATCA mandates the
  QR encodes a base64 of seller name + VAT + timestamp + total + VAT
  amount. Need a PDF parser + QR decoder. Worth doing once we have a
  paying customer.
- **CSV import** with malformed rows: a row with a quoted comma, a
  mixed-encoding line, a too-long supplier name. Existing import code
  is forgiving in some ways, strict in others; would benefit from its
  own QA pass.
- **Concurrent voucher posting** — race-condition test where two users
  hit "post" on the same draft voucher simultaneously. The BE should
  reject the second; FE should show the error gracefully.
- **Period-cutoff** test — bill dated 30 Jun 23:59 Asia/Riyadh stays in
  June reporting; bill dated 1 Jul 00:00 lands in July. Needs a way to
  inject server time, which we don't have today.

Cheers — opening the e2e spec files now.
