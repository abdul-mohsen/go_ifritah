// QA-07: Authentication boundaries.
// Unauth requests should be redirected/rejected, no protected page leaks data.

const { test, expect } = require('@playwright/test');
const { USER, PASS } = require('../helpers/auth');

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
  const loginUrl = page.url();
  let navigations = 0;
  page.on('framenavigated', (frame) => {
    if (frame === page.mainFrame()) navigations++;
  });
  await page.fill('input[name="username"]', 'nobody');
  await page.fill('input[name="password"]', 'wrongpass');
  const loginPost = page.waitForResponse((resp) => resp.url().endsWith('/login') && resp.request().method() === 'POST');
  await page.click('button[type="submit"]');
  await expect(page.locator('#login-error')).toBeVisible();
  await expect(page.locator('#login-error')).not.toBeEmpty();
  expect((await loginPost).status()).toBe(200);
  await page.waitForTimeout(2200);
  expect(page.url()).toBe(loginUrl);
  expect(navigations).toBe(0);
});

test('login password can be revealed and hidden', async ({ page }) => {
  await page.goto('/login');
  const password = page.locator('input[name="password"]');
  const toggle = page.locator('#toggle-login-password');

  await password.fill('secret-value');
  await expect(password).toHaveAttribute('type', 'password');
  await toggle.click();
  await expect(password).toHaveAttribute('type', 'text');
  await expect(toggle).toHaveAttribute('aria-pressed', 'true');
  await toggle.click();
  await expect(password).toHaveAttribute('type', 'password');
  await expect(toggle).toHaveAttribute('aria-pressed', 'false');
});

test('logout clears session', async ({ page }) => {
  await page.goto('/login');
  await page.fill('input[name="username"]', USER);
  await page.fill('input[name="password"]', PASS);
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
