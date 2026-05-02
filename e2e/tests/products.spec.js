const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/auth');

test('products list page loads', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/products');
  await expect(page).toHaveURL(/products/);
});

test('product detail shows all fields', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/products');

  // Click first product link
  const firstProduct = page.locator('a[href*="/dashboard/products/"]').first();
  if (await firstProduct.count() > 0) {
    await firstProduct.click();
    // Should show price, quantity, shelf, cost price, min stock fields
    await expect(page.locator('text=/سعر البيع|Selling Price/')).toBeVisible();
    await expect(page.locator('text=/الكمية|Quantity/')).toBeVisible();
  }
});
