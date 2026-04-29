// QA-29: Supplier account report export must be usable in Excel.
//
// Regression target:
// - the current export is a ragged multi-section CSV (summary rows + ledger + bills),
//   which opens poorly in Excel
// - bill number must be an explicit exported column and match the visible report

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

function parseCsv(text) {
  const rows = [];
  let row = [];
  let cell = '';
  let inQuotes = false;

  const input = text.replace(/^\uFEFF/, '');
  for (let i = 0; i < input.length; i += 1) {
    const ch = input[i];
    const next = input[i + 1];
    if (ch === '"') {
      if (inQuotes && next === '"') {
        cell += '"';
        i += 1;
      } else {
        inQuotes = !inQuotes;
      }
      continue;
    }
    if (ch === ',' && !inQuotes) {
      row.push(cell);
      cell = '';
      continue;
    }
    if ((ch === '\n' || ch === '\r') && !inQuotes) {
      if (ch === '\r' && next === '\n') i += 1;
      row.push(cell);
      rows.push(row);
      row = [];
      cell = '';
      continue;
    }
    cell += ch;
  }
  if (cell.length > 0 || row.length > 0) {
    row.push(cell);
    rows.push(row);
  }
  return rows;
}

test.describe('Supplier account report export', () => {
  test('CSV export is a single rectangular table and includes visible bill numbers', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/suppliers');
    await page.waitForLoadState('domcontentloaded');

    const reportHref = await page.locator('a[href*="/report"]').first().getAttribute('href').catch(() => null);
    if (!reportHref) test.skip(true, 'no supplier report links on dev backend');

    const supplierId = (reportHref.match(/\/dashboard\/suppliers\/(\d+)\/report/) || [])[1];
    expect(supplierId, `could not parse supplier id from ${reportHref}`).toBeTruthy();

    const exportHref = `/dashboard/suppliers/${supplierId}/report/export-csv?from=2000-01-01&to=2099-12-31`;
    const response = await page.request.get(exportHref);
    expect(response.ok(), `supplier report export failed with ${response.status()}`).toBeTruthy();

    const rows = parseCsv(await response.text()).filter((r) => r.some((c) => c.trim() !== ''));
    expect(rows.length, 'export must include a header and at least one data row').toBeGreaterThan(1);

    const header = rows[0].map((c) => c.trim());
    const requiredHeaders = ['رقم الفاتورة', 'التاريخ', 'النوع', 'المرجع', 'الوصف', 'مدين', 'دائن', 'الرصيد'];
    for (const name of requiredHeaders) {
      expect(header, `missing export column: ${name}`).toContain(name);
    }

    const width = header.length;
    const raggedRow = rows.find((r) => r.length !== width);
    expect(raggedRow, `CSV must be rectangular for Excel; found row with ${raggedRow ? raggedRow.length : width} cells instead of ${width}`).toBeUndefined();

    const billNoIdx = header.indexOf('رقم الفاتورة');
    const exportedBillNumbers = rows.slice(1).map((r) => (r[billNoIdx] || '').trim()).filter(Boolean);
    expect(exportedBillNumbers.length, 'bill number column exists but has no bill values').toBeGreaterThan(0);
  });
});