// Purchase-bill store rows charge the supplier's purchase price and maintain
// the inventory cost price. They also surface the catalog's current selling
// price for review (editable for admin/manager, read-only otherwise).

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

async function assertStockRowPriceLabels(page) {
  await page.locator('button[onclick^="addItem"]').first().click();
  const row = page.locator('#products-container .item-row').last();
  await expect(row).toBeVisible();

  // Two price fields since the cost-price column was merged into purchase price:
  //   - Purchase Price (products_cost_price, aria-label = tpl.pb.purchase_price)
  //   - Selling Price  (products_selling_price)
  // The old separate Cost Price column was removed.
  await expect(row.getByLabel(/سعر الشراء|Purchase Price/)).toBeVisible();
  await expect(row.getByLabel(/سعر البيع|Selling Price/)).toBeVisible();
}

test.describe('Purchase-bill store item price columns', () => {
  test('add page labels store item prices as purchase price + selling price', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    await assertStockRowPriceLabels(page);
  });

  test('edit page labels store item prices as purchase price + selling price', async ({ page }) => {
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
