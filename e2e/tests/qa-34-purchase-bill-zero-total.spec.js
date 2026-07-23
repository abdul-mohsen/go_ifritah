const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.describe('Purchase bill zero-total guard', () => {
  test('stops a zero-total purchase bill and shows the user error', async ({ page }) => {
    await login(page);

    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');
    await expect(page.locator('#purchase-form')).toBeVisible();

    await page.locator('[name="supplier_sequance_number"]').fill(`PB-ZERO-${Date.now()}`);
    await page.locator('[name="payment_date"]').fill('2026-05-18');

    const pdfInput = page.locator('[name="bill_pdf"]');
    if (await pdfInput.count()) {
      await pdfInput.setInputFiles({
        name: 'zero-total.pdf',
        mimeType: 'application/pdf',
        buffer: Buffer.from('%PDF-1.4\n%%EOF\n'),
      });
    }

    await page.locator('button[onclick="addItem()"]').click();
    const row = page.locator('#products-container .item-row').last();
    await row.locator('.store-product-search').fill('Zero total part');
    await row.locator('[name="products_quantity"]').fill('1');
    await row.locator('[name="products_cost_price"]').fill('0');

    await expect(page.locator('#total_amount')).toHaveValue('0.00');

    const submitResponse = page.waitForResponse((response) =>
      response.url().includes('/api/purchase-bills') &&
      response.request().method() === 'POST'
    );

    await page.locator('#purchase-form button[type="submit"]').click();
    expect((await submitResponse).status()).toBe(400);

    await expect(page).toHaveURL(/\/dashboard\/purchase-bills\/add$/);
    const errorToast = page.locator('.toast').filter({ hasText: "You can't submit an invoice with 0" });
    await expect(errorToast).toHaveCount(1);
    await expect(errorToast.first()).toBeVisible();
  });
});