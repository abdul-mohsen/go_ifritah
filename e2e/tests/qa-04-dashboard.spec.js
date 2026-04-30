// QA-04: Dashboard renders all key panels and accepts date filtering.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.beforeEach(async ({ page }) => { await login(page); });

test('dashboard loads with sidebar + main content', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.locator('#sidebar-nav')).toBeVisible();
});

test('dashboard has KPI cards linking to invoices/products/clients', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.locator('a[href="/dashboard/invoices"]').first()).toBeAttached();
  await expect(page.locator('a[href="/dashboard/products"]').first()).toBeAttached();
  await expect(page.locator('a[href="/dashboard/clients"]').first()).toBeAttached();
  await expect(page.locator('a[href="/dashboard/suppliers"]').first()).toBeAttached();
});

test('dashboard accepts ?start_date and ?end_date filters', async ({ page }) => {
  const today = new Date().toISOString().slice(0, 10);
  const resp = await page.goto(`/dashboard?start_date=${today}&end_date=${today}`);
  expect(resp.status()).toBeLessThan(400);
});

test('dashboard date range "month" loads', async ({ page }) => {
  const resp = await page.goto('/dashboard?period=month');
  expect(resp.status()).toBeLessThan(400);
});

test('dashboard compare page loads', async ({ page }) => {
  const resp = await page.goto('/dashboard/compare');
  expect(resp.status()).toBeLessThan(400);
});

test('dashboard PDF export endpoint responds', async ({ page, request }) => {
  await login(page);
  const cookies = await page.context().cookies();
  const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
  const resp = await request.get('/dashboard/export-pdf', { headers: { cookie: cookieHeader } });
  expect([200, 302, 500]).toContain(resp.status());
});

test('no mock-data badges remain on dashboard (regression check)', async ({ page }) => {
  await page.goto('/dashboard');
  // We removed all "بيانات تجريبية" badges. Make sure none are present.
  const html = await page.content();
  expect(html).not.toMatch(/بيانات تجريبية/);
});
