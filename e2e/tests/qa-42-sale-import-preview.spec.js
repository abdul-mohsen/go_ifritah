const { test, expect } = require('@playwright/test');
const { login, appURL } = require('../helpers/qa');

// The sales preview is checked against stubbed answers, so these cases do not depend on the backend's data. The real check is
// covered in qa-38. Each case says what the stub answers.
const PREVIEW_ROWS = [
  { row: 1, customer: 'Ahmed', date: '2026-01-15', total: '150.00', valid: true, errors: [], warnings: [{ code: 'duplicate_sale', message: 'this sale matches bill 42 on this branch' }], duplicate_of: 42 },
  { row: 2, customer: 'Khaled', date: '2026-01-16', total: '40.00', valid: true, errors: [], warnings: [], duplicate_of: null },
  { row: 3, customer: 'Sara', date: '2026-01-17', total: '20.00', valid: false, errors: [{ code: 'branch_not_found', message: 'branch 9 does not exist' }], warnings: [], duplicate_of: null },
  { row: 4, customer: 'Omar', date: '2026-01-18', total: '75.00', valid: true, errors: [], warnings: [{ code: 'vin_format', message: 'the VIN has an unknown format' }], duplicate_of: null },
  { row: 5, customer: 'Noor', date: '2026-01-19', total: '10.00', valid: false, errors: [{ code: 'zatca_unregistered', message: 'the branch is not registered with ZATCA' }], warnings: [], duplicate_of: null },
];

async function openSalesPage(page) {
  await login(page);
  await page.goto(appURL('/dashboard/bill-import?type=sales'));
  await page.locator('#bill-import-file').setInputFiles({
    name: 'sales.xlsx',
    mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    buffer: Buffer.from('stub workbook'),
  });
}

// stubRoutes answers the check and the upload with the given answers, and records what the page sent to the upload.
async function stubRoutes(page, { check, upload }) {
  const uploads = [];
  await page.route('**/api/bill-import/check**', (route) => route.fulfill(check));
  await page.route('**/api/bill-import/upload**', async (route) => {
    const body = route.request().postData() || '';
    uploads.push(body);
    await route.fulfill(upload);
  });
  return uploads;
}

const CHECK_OK = { status: 200, contentType: 'application/json', body: JSON.stringify({ total: PREVIEW_ROWS.length, rows: PREVIEW_ROWS }) };

function selectedRowsOf(body) {
  return [...body.matchAll(/name="selected_rows"\r\n\r\n(\d+)/g)].map((match) => match[1]);
}

test.describe('Sales preview states and posting', () => {
  test('each row shows its status, and only the OK rows are ticked by default', async ({ page }) => {
    await stubRoutes(page, { check: CHECK_OK, upload: { status: 200, contentType: 'application/json', body: '{}' } });
    await openSalesPage(page);
    await page.locator('#bill-import-submit').click();

    await expect(page.locator('#bill-import-preview')).toBeVisible();
    await expect(page.locator('#bill-import-table tbody tr')).toHaveCount(PREVIEW_ROWS.length);
    const status = async (row) => page.locator(`#bill-import-table tr[data-row="${row}"]`).getAttribute('data-status');
    expect(await status(1)).toBe('duplicate');
    expect(await status(2)).toBe('ok');
    expect(await status(3)).toBe('error');
    expect(await status(4)).toBe('warning');
    expect(await status(5)).toBe('error');

    const ticked = page.locator('#bill-import-table input[data-row-check]:checked');
    await expect(ticked).toHaveCount(1);
    await expect(page.locator('#bill-import-table tr[data-row="2"] input[data-row-check]')).toBeChecked();
  });

  test('an error row cannot be ticked, and its messages are shown, including a code the page does not know', async ({ page }) => {
    await stubRoutes(page, { check: CHECK_OK, upload: { status: 200, contentType: 'application/json', body: '{}' } });
    await openSalesPage(page);
    await page.locator('#bill-import-submit').click();

    const unknown = page.locator('#bill-import-table tr[data-row="5"]');
    await expect(unknown.locator('input[data-row-check]')).toBeDisabled();
    await expect(unknown).toContainText('the branch is not registered with ZATCA');
    await expect(unknown).toContainText('zatca_unregistered');
    await expect(page.locator('#bill-import-table tr[data-row="3"] input[data-row-check]')).toBeDisabled();
  });

  test('only the ticked rows are posted, with their row numbers, and a duplicate is not posted unless it is ticked', async ({ page }) => {
    const uploads = await stubRoutes(page, {
      check: CHECK_OK,
      upload: { status: 200, contentType: 'application/json', body: JSON.stringify({ total: 1, success: 1, failed: 0, results: [{ row: 2, ok: true, bill_id: 77 }] }) },
    });
    await openSalesPage(page);
    await page.locator('#bill-import-submit').click();
    await page.locator('#bill-import-commit').click();

    await expect.poll(() => uploads.length).toBe(1);
    expect(selectedRowsOf(uploads[0])).toEqual(['2']);
    expect(selectedRowsOf(uploads[0])).not.toContain('1');
    expect(selectedRowsOf(uploads[0])).not.toContain('4');
  });

  test('ticking a duplicate posts it with its row number', async ({ page }) => {
    const uploads = await stubRoutes(page, {
      check: CHECK_OK,
      upload: { status: 200, contentType: 'application/json', body: JSON.stringify({ total: 2, success: 2, failed: 0, results: [{ row: 1, ok: true, bill_id: 42 }, { row: 2, ok: true, bill_id: 77 }] }) },
    });
    await openSalesPage(page);
    await page.locator('#bill-import-submit').click();
    await page.locator('#bill-import-table tr[data-row="1"] input[data-row-check]').check();
    await page.locator('#bill-import-commit').click();

    await expect.poll(() => uploads.length).toBe(1);
    expect(selectedRowsOf(uploads[0])).toEqual(['1', '2']);
  });

  test('the result counts the rows as created, replayed and failed', async ({ page }) => {
    await stubRoutes(page, {
      check: CHECK_OK,
      upload: { status: 200, contentType: 'application/json', body: JSON.stringify({ total: 3, success: 2, failed: 1, results: [{ row: 1, ok: true, bill_id: 42 }, { row: 2, ok: true, bill_id: 77 }, { row: 4, ok: false, error: 'backend refused' }] }) },
    });
    await openSalesPage(page);
    await page.locator('#bill-import-submit').click();
    await page.locator('#bill-import-table tr[data-row="1"] input[data-row-check]').check();
    await page.locator('#bill-import-table tr[data-row="4"] input[data-row-check]').check();
    await page.locator('#bill-import-commit').click();

    await expect(page.locator('#bill-import-result')).toHaveAttribute('data-created', '1');
    await expect(page.locator('#bill-import-result')).toHaveAttribute('data-replayed', '1');
    await expect(page.locator('#bill-import-result')).toHaveAttribute('data-failed', '1');
    await expect(page.locator('#bill-import-result')).toHaveAttribute('data-success', '2');
  });

  test('a failed check posts nothing and shows its error', async ({ page }) => {
    const uploads = await stubRoutes(page, {
      check: { status: 502, contentType: 'application/json', body: JSON.stringify({ total: 0, rows: [], error: 'backend check failed (status 500)' }) },
      upload: { status: 200, contentType: 'application/json', body: '{}' },
    });
    await openSalesPage(page);
    await page.locator('#bill-import-submit').click();

    await expect(page.locator('#bill-import-result')).toContainText('backend check failed');
    await expect(page.locator('#bill-import-preview')).toBeHidden();
    expect(uploads).toHaveLength(0);
  });

  test('cancel posts nothing', async ({ page }) => {
    const uploads = await stubRoutes(page, { check: CHECK_OK, upload: { status: 200, contentType: 'application/json', body: '{}' } });
    await openSalesPage(page);
    await page.locator('#bill-import-submit').click();
    await page.locator('#bill-import-cancel').click();

    await expect(page.locator('#bill-import-preview')).toBeHidden();
    await expect(page.locator('#bill-import-result')).toHaveText(/Nothing was imported|لم يُستورد شيء/);
    expect(uploads).toHaveLength(0);
  });
});
