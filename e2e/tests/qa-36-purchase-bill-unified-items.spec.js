const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test('purchase bill items distinguish dropdown inventory from manual and CSV rows', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills/add');
  await page.waitForLoadState('domcontentloaded');

  const storeID = Number(await page.locator('[name="store_id"]').inputValue());
  if (!storeID) test.skip(true, 'no store is available for the purchase bill');

  await page.locator('button[onclick^="addItem"]').click();
  const typedRow = page.locator('#products-container .item-row').last();
  await expect(typedRow.locator('.item-state')).toContainText(/مضاف يدوياً|Manually Added/);
  await expect(typedRow.locator('[name="products_track_stock"]')).toHaveValue('false');
  await expect(typedRow.locator('[name="products_cost_price"]')).toBeVisible();
  await expect(typedRow.locator('[name="products_shelf_number"]')).toBeVisible();

  await page.route('**/api/products/search-json', async (route) => {
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify([{
        id: 456,
        name: 'QA Store Filter',
        store_id: storeID,
        cost_price: '20.00',
        shelf_number: 'A-1',
      }]),
    });
  });

  await typedRow.locator('.store-product-search').fill('QA Store Filter');
  await typedRow.getByText('QA Store Filter', { exact: true }).click();
  await expect(typedRow.locator('.item-state')).toContainText(/من المخزن|From Store/);
  await expect(typedRow.locator('[name="products_track_stock"]')).toHaveValue('true');
  await expect(typedRow.locator('[name="products_cost_price"]')).toHaveValue('20.00');
  await expect(typedRow.locator('[name="products_shelf_number"]')).toHaveValue('A-1');

  await page.route('**/api/purchase-bills/parse-csv', async (route) => {
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify([{
        partName: 'QA Imported Manual Item',
        quantity: 2,
        purchasePrice: 25,
        costPrice: 18,
        shelfNumber: 'B-2',
      }]),
    });
  });

  await page.locator('input[type="file"][accept=".csv,text/csv"]').setInputFiles({
    name: 'purchase-bill.csv',
    mimeType: 'text/csv',
    buffer: Buffer.from('name,quantity,purchase_price,cost_price,shelf_number\nQA Imported Manual Item,2,25,18,B-2\n'),
  });

  const importedRow = page.locator('#products-container .item-row').last();
  await expect(importedRow.locator('.item-state')).toContainText(/مضاف يدوياً|Manually Added/);
  await expect(importedRow.locator('[name="products_track_stock"]')).toHaveValue('false');
  await expect(importedRow.locator('.store-product-search')).toHaveValue('QA Imported Manual Item');
  await expect(importedRow.locator('[name="products_cost_price"]')).toHaveValue('18');
  await expect(importedRow.locator('[name="products_shelf_number"]')).toHaveValue('B-2');
});
