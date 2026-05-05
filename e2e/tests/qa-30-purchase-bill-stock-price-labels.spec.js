// QA-30: Purchase-bill stock rows should expose Cost Price + Selling Price.
//
// Regression target: catalog/stock product rows showed both "Purchase Price"
// and "Cost Price", which is ambiguous. For products pulled from stock, the
// two user-facing columns must be cost price and selling price.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

async function assertStockRowPriceLabels(page) {
  await page.locator('button[onclick^="addItem"]').first().click();
  const row = page.locator('#products-container .item-row').last();
  await expect(row).toBeVisible();

  const labels = (await row.locator('label').allTextContents()).map((t) => t.trim()).join(' | ');
  expect(labels, 'stock row must show cost price').toMatch(/سعر التكلفة|Cost Price/);
  expect(labels, 'stock row must show selling price').toMatch(/سعر البيع|Selling Price/);
  expect(labels, 'stock row must not show purchase price as a separate column').not.toMatch(/سعر الشراء|Purchase Price/);
}

test.describe('Purchase-bill stock product price columns', () => {
  test('add page labels stock product prices as cost price + selling price', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    await assertStockRowPriceLabels(page);
  });

  test('edit page labels stock product prices as cost price + selling price', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills');
    await page.waitForLoadState('domcontentloaded');

    const editLink = page.locator('a[href^="/dashboard/purchase-bills/edit/"]').first();
    if (await editLink.count() === 0) test.skip(true, 'no purchase bill edit page on dev backend');
    const editHref = await editLink.getAttribute('href');
    if (!editHref) test.skip(true, 'no purchase bill edit page on dev backend');

    await page.goto(editHref);
    await page.waitForLoadState('domcontentloaded');

    await assertStockRowPriceLabels(page);
  });
});
