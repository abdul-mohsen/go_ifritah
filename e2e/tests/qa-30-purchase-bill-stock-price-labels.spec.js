// Purchase-bill store rows charge the supplier's purchase price and maintain
// the inventory cost price. A selling price is not part of a purchase bill.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

async function assertStockRowPriceLabels(page) {
  await page.locator('button[onclick^="addItem"]').first().click();
  const row = page.locator('#products-container .item-row').last();
  await expect(row).toBeVisible();

  const labels = (await row.locator('label').allTextContents()).map((t) => t.trim()).join(' | ');
  expect(labels, 'stock row must show cost price').toMatch(/سعر التكلفة|Cost Price/);
  expect(labels, 'stock row must show purchase price').toMatch(/سعر الشراء|Purchase Price/);
  expect(labels, 'stock row must not show selling price').not.toMatch(/سعر البيع|Selling Price/);
}

test.describe('Purchase-bill store item price columns', () => {
  test('add page labels store item prices as purchase price + cost price', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    await assertStockRowPriceLabels(page);
  });

  test('edit page labels store item prices as purchase price + cost price', async ({ page }) => {
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
