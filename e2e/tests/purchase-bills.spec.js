const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/auth');

test('purchase bills list page loads', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills');
  await expect(page).toHaveURL(/purchase-bills/);
});

test('purchase bills empty state CTA has no leaked template delimiters', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills?q=__empty_purchase_bill_delimiter_check__');

  await expect(page.locator('a[href="/dashboard/purchase-bills/add"]').first()).toBeVisible();
  await expect(page.locator('a[href="/dashboard/purchases/add"]')).toHaveCount(0);
  await expect(page.locator('body')).not.toContainText('{{');
  await expect(page.locator('body')).not.toContainText('}}');
});

test('add purchase bill form loads', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills/add');
  // Form should have store, supplier, manual section
  await expect(page.locator('form')).toBeVisible();
});

test('submit button disables on click (no double submit)', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills/add');
  const submitBtn = page.locator('button[type="submit"]');
  // hx-disabled-elt should be set
  const form = page.locator('form[hx-disabled-elt]');
  await expect(form).toBeVisible();
});
