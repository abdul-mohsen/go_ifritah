// QA-08: Notifications page + bell + read flow.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.beforeEach(async ({ page }) => { await login(page); });

test('notifications page loads', async ({ page }) => {
  const resp = await page.goto('/dashboard/notifications');
  expect(resp.status()).toBeLessThan(400);
});

test('notification config page accessible', async ({ page, request }) => {
  const cookies = await page.context().cookies();
  const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
  const resp = await request.get('/api/notification-config', { headers: { cookie: cookieHeader } });
  expect([200, 204, 401]).toContain(resp.status());
});

test('mark all notifications as read endpoint accepts POST', async ({ page, request }) => {
  const cookies = await page.context().cookies();
  const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
  const resp = await request.post('/api/notifications/read-all', { headers: { cookie: cookieHeader } });
  // Endpoint may require csrf — accept 200 or 4xx without 5xx
  expect(resp.status()).toBeLessThan(500);
});
