const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/auth');

test('ZATCA monitor page loads', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/zatca-monitor');
  await expect(page).toHaveURL(/zatca-monitor/);
});

test('ZATCA monitor shows summary cards', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/zatca-monitor');
  await expect(page.locator('#zm-total')).toBeVisible();
  await expect(page.locator('#zm-accepted')).toBeVisible();
  await expect(page.locator('#zm-warnings')).toBeVisible();
  await expect(page.locator('#zm-rejected')).toBeVisible();
  await expect(page.locator('#zm-pending')).toBeVisible();
});

test('ZATCA monitor shows branch status table', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/zatca-monitor');
  // Table should have rows
  const rows = page.locator('tbody tr');
  const count = await rows.count();
  expect(count).toBeGreaterThan(0);
});

test('ZATCA monitor has no retry button (auto-submission)', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/zatca-monitor');
  // Retry button should NOT exist
  const retryButtons = page.locator('button:has-text("إعادة إرسال"), button:has-text("Retry")');
  await expect(retryButtons).toHaveCount(0);
});

test('ZATCA monitor status filter works', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/zatca-monitor');
  await expect(page.locator('#zm-filter-status')).toBeVisible();
  // All rows should be visible initially
  const allRows = page.locator('.zm-row');
  const totalRows = await allRows.count();
  expect(totalRows).toBeGreaterThan(0);
});

test('ZATCA monitor submissions table shows invoice links', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/zatca-monitor');
  const invoiceLinks = page.locator('#zm-submissions-table a[href*="/dashboard/invoices/"]');
  const count = await invoiceLinks.count();
  expect(count).toBeGreaterThan(0);
});

test('ZATCA monitor refresh button exists', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/zatca-monitor');
  const refreshBtn = page.locator('button:has-text("تحديث"), button:has-text("Refresh")');
  await expect(refreshBtn).toBeVisible();
});
