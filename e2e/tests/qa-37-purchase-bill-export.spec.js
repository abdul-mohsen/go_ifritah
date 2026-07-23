const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.describe('Purchase-bill export', () => {
  test('purchase bills can be exported as a two-sheet Excel workbook', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills');
    await page.waitForLoadState('domcontentloaded');

    const exportLink = page.locator('[data-purchase-bill-export]');
    await expect(exportLink).toBeVisible();

    // Bulk export walks every matching purchase bill's detail individually
    // on the backend (no batch endpoint yet), so it can legitimately take
    // longer than Playwright's default assertion timeout as the shared
    // dev account's bill history grows.
    const response = await page.request.get('/dashboard/purchase-bills/export-xlsx', { timeout: 30000 });
    expect(response.ok(), `purchase-bill export failed with ${response.status()}`).toBeTruthy();
    expect(response.headers()['content-type']).toContain('application/vnd.openxmlformats-officedocument.spreadsheetml.sheet');
    expect(response.headers()['content-disposition']).toContain('purchase-bills.xlsx');
    expect((await response.body()).subarray(0, 2).toString()).toBe('PK');
  });

  test('purchase bills use Received Date rather than Deliver Date', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    const receivedDateLabel = page.locator('label[for="purchase_deliver_date"]');
    await expect(receivedDateLabel).toContainText(/تاريخ الاستلام|Received Date/);
    await expect(receivedDateLabel).not.toContainText(/تاريخ التسليم|Deliver Date/);
  });

  test('purchase-bill template uses the canonical import column names', async ({ page }) => {
    await login(page);

    const response = await page.request.get('/api/purchase-bills/import-template');
    expect(response.ok(), `purchase-bill template failed with ${response.status()}`).toBeTruthy();

    const csv = (await response.text()).replace(/^\uFEFF/, '');
    // Template gained a trailing optional Product ID column so imports can
    // link a row to an existing catalog product (see qa-39).
    expect(csv.split(/\r?\n/)[0]).toMatch(/^(Product Name,Quantity,Purchase Price,Shelf Number,Product ID \(optional\)|اسم القطعة,الكمية,سعر الشراء,رقم الرف,معرف المنتج \(اختياري\))$/);
  });
});
