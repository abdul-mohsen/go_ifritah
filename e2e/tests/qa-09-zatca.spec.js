// QA-09: ZATCA monitor + branch config endpoints.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.beforeEach(async ({ page }) => { await login(page); });

test('zatca monitor page loads', async ({ page }) => {
  const resp = await page.goto('/dashboard/zatca-monitor');
  expect(resp.status()).toBeLessThan(400);
});

test('zatca branch config GET responds for branch id 1', async ({ page, request }) => {
  const cookies = await page.context().cookies();
  const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
  const resp = await request.get('/api/zatca/branch/1', { headers: { cookie: cookieHeader } });
  expect(resp.status()).toBeLessThan(500);
});
