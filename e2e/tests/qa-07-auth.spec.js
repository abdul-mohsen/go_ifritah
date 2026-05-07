// QA-07: Authentication boundaries.
// Unauth requests should be redirected/rejected, no protected page leaks data.

const { test, expect } = require('@playwright/test');

// /dashboard/users and /dashboard/zatca-monitor were removed on dev (no
// real backend yet). Keep this list in sync with main.go.
const PROTECTED = [
  '/dashboard',
  '/dashboard/invoices',
  '/dashboard/purchase-bills',
  '/dashboard/cash-vouchers',
  '/dashboard/products',
  '/dashboard/clients',
  '/dashboard/suppliers',
  '/dashboard/settings',
  '/dashboard/notifications',
];

// All tests in this file must run anonymously: the `parallel` project
// loads an authenticated storageState by default, which would defeat the
// "unauth" assertions below.
test.use({ storageState: { cookies: [], origins: [] } });

for (const url of PROTECTED) {
  test(`unauth ${url} redirects to login`, async ({ page }) => {
    const resp = await page.goto(url);
    // Either status 401/403 or redirect to /login
    const finalURL = page.url();
    if (resp && resp.status() < 400) {
      expect(finalURL).toMatch(/login|^\/$|\/$/);
    }
  });
}

test('login form rejects bad credentials', async ({ page }) => {
  await page.goto('/login');
  await page.fill('input[name="username"]', 'nobody');
  await page.fill('input[name="password"]', 'wrongpass');
  await page.click('button[type="submit"]');
  // Should not redirect to dashboard
  await page.waitForTimeout(1500);
  expect(page.url()).not.toMatch(/\/dashboard($|\?)/);
});

test('logout clears session', async ({ page }) => {
  // Log in via UI
  await page.goto('/login');
  await page.fill('input[name="username"]', 'ssda');
  await page.fill('input[name="password"]', 'Qwerty123');
  await page.click('button[type="submit"]');
  await page.waitForURL('**/dashboard**');
  // Logout
  await page.goto('/logout');
  // Visiting dashboard again should redirect/deny
  const resp = await page.goto('/dashboard');
  if (resp && resp.status() < 400) {
    expect(page.url()).toMatch(/login|^\/$|\/$/);
  }
});
