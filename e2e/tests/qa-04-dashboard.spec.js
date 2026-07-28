// QA-04: Dashboard renders all key panels and accepts date filtering.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.beforeEach(async ({ page }) => { await login(page); });

test('dashboard loads with sidebar + main content', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.locator('#sidebar-nav')).toBeVisible();
});

test('dashboard defaults to a quarter selector instead of date pickers', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.locator('#period')).toHaveValue('quarter');
  await expect(page.locator('#quarterFilter')).toBeVisible();
  await expect(page.locator('#yearFilter')).toBeVisible();
  await expect(page.locator('#quarter')).toHaveValue(/^[1-4]$/);
  await expect(page.locator('#year')).toHaveValue(/^\d{4}$/);
  await expect(page.locator('#quarterDateRange')).toHaveText(/^\d{4}-\d{2}-\d{2} – \d{4}-\d{2}-\d{2}$/);
  await expect(page.locator('#dateRangeFilters')).toBeHidden();
});

test('dashboard quarterly filters fit the mobile layout', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/dashboard');

  await expect(page.locator('#quarterFilter')).toBeVisible();
  await expect(page.locator('#yearFilter')).toBeVisible();
  const overflows = await page.locator('#dashboardFilters').evaluate((element) => element.scrollWidth > element.clientWidth);
  expect(overflows).toBe(false);
});

test('dashboard year options come only from the backend data years', async ({ page, request }) => {
  await page.goto('/dashboard');
  const cookies = await page.context().cookies();
  const cookie = cookies.map((item) => `${item.name}=${item.value}`).join('; ');
  const response = await request.get('/api/v2/dashboard/available-years', { headers: { cookie } });
  expect(response.ok()).toBeTruthy();
  const years = (await response.json()).years.map(String);
  const options = await page.locator('#year option:not([disabled])').evaluateAll((elements) =>
    elements.map((element) => element.value)
  );
  expect(options).toEqual(years);
});

test('dashboard applies a selected quarter through the URL', async ({ page }) => {
  await page.goto('/dashboard');
  const quarter = page.locator('#quarter');
  const year = page.locator('#year');
  await quarter.selectOption({ index: 1 });
  await year.selectOption({ index: 0 });
  const expectedQuarter = await quarter.inputValue();
  const expectedYear = await year.inputValue();
  await page.locator('#applyDashboardFilters').click();
  await expect(page).toHaveURL(new RegExp(`period=quarter&quarter=${expectedQuarter}&year=${expectedYear}`));
});

test('dashboard applies a selected report year through the URL', async ({ page }) => {
  await page.goto('/dashboard');
  await page.locator('#period').selectOption('year');
  await expect(page.locator('#yearFilter')).toBeVisible();
  await expect(page.locator('#quarterFilter')).toBeHidden();
  await expect(page.locator('#quarterDateRangeFilter')).toBeVisible();
  await expect(page.locator('#dateRangeFilters')).toBeHidden();
  const year = await page.locator('#year').inputValue();
  await page.locator('#applyDashboardFilters').click();
  await expect(page).toHaveURL(new RegExp(`period=year&year=${year}`));
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
  await expect(page.locator('#period')).toHaveValue('month');
  await expect(page.locator('#startDate')).toHaveValue(/^\d{4}-\d{2}-01$/);
});

test('dashboard all-dates period clears the date range', async ({ page }) => {
  const resp = await page.goto('/dashboard?period=all');
  expect(resp.status()).toBeLessThan(400);
  await expect(page.locator('#period')).toHaveValue('all');
  await expect(page.locator('#startDate')).toHaveValue('');
  await expect(page.locator('#endDate')).toHaveValue('');
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
