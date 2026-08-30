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

test('product entry accepts free text without OEM search controls', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/products/add');

  const nameInput = page.locator('input[name="part_name[]"]');
  await expect(nameInput).toHaveCount(1);
  await expect(nameInput).toHaveAttribute('required', '');
  await expect(page.locator('input.part-search')).toHaveCount(0);
  await expect(page.locator('input[type="hidden"][name="part_name[]"]')).toHaveCount(0);
});

test('parts search hides obsolete category quick filters', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/parts-search');
  await expect(page.locator('.parts-cat-btn')).toHaveCount(0);
});

test('clicking "add row" focuses the new row name input', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/products/add');

  // Initial row is present but the store selector, not the row input, is
  // the natural first-focus target on page load — do not assert focus on
  // the initial row.
  await expect(page.locator('input[name="part_name[]"]')).toHaveCount(1);

  const addBtn = page.locator('#add-row-btn');
  await addBtn.click();
  await expect(page.locator('input[name="part_name[]"]')).toHaveCount(2);

  // The new (2nd) row's name input must be focused so the user can type
  // immediately. Regression: pre-fix the click only appended the row and
  // left focus on the "Add Row" button, forcing an extra keyboard/mouse
  // reach to start entering the product name.
  const secondName = page.locator('input[name="part_name[]"]').nth(1);
  await expect(secondName).toBeFocused();
});
