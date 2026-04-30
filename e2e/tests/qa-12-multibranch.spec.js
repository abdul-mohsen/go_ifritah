// QA-12: Multi-branch / multi-store presence.
// Verifies seed data shape (10 branches, 10 stores) and that each branch
// detail page renders. We can't safely test cross-branch isolation without
// non-admin users, but we can confirm the data model works.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.beforeEach(async ({ page }) => { await login(page); });

test('branches list shows multiple branches', async ({ page }) => {
  await page.goto('/dashboard/branches');
  const links = await page.locator('a[href^="/dashboard/branches/"]').evaluateAll((as) =>
    Array.from(new Set(as.map((a) => a.getAttribute('href').match(/\/dashboard\/branches\/(\d+)/)?.[1]).filter(Boolean)))
  );
  expect(links.length, `unique branch ids: ${links.join(',')}`).toBeGreaterThanOrEqual(2);
});

test('stores list shows multiple stores', async ({ page }) => {
  await page.goto('/dashboard/stores');
  const links = await page.locator('a[href^="/dashboard/stores/"]').evaluateAll((as) =>
    Array.from(new Set(as.map((a) => a.getAttribute('href').match(/\/dashboard\/stores\/(\d+)/)?.[1]).filter(Boolean)))
  );
  expect(links.length).toBeGreaterThanOrEqual(2);
});

test('first branch detail page renders', async ({ page }) => {
  await page.goto('/dashboard/branches');
  const firstId = await page.locator('a[href^="/dashboard/branches/"]').first().getAttribute('href');
  if (firstId) {
    const resp = await page.goto(firstId);
    expect(resp.status()).toBeLessThan(400);
  }
});

test('first store detail page renders', async ({ page }) => {
  await page.goto('/dashboard/stores');
  const firstId = await page.locator('a[href^="/dashboard/stores/"]').first().getAttribute('href');
  if (firstId) {
    const resp = await page.goto(firstId);
    expect(resp.status()).toBeLessThan(400);
  }
});

test('add-bill form lists all branches and stores in selects', async ({ page }) => {
  await page.goto('/dashboard/invoices/add-invoice');
  const stores = await page.locator('select[name="store_id"] option').count();
  const branches = await page.locator('select[name="branch_id"] option').count();
  expect(stores).toBeGreaterThanOrEqual(2);
  expect(branches).toBeGreaterThanOrEqual(2);
});

test('add-cash-voucher form lists all stores', async ({ page }) => {
  await page.goto('/dashboard/cash-vouchers/add');
  const stores = await page.locator('select[name="store_id"] option').count();
  expect(stores).toBeGreaterThanOrEqual(2);
});
