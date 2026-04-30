const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/auth');

test('login page loads', async ({ page }) => {
  await page.goto('/login');
  await expect(page).toHaveURL(/login/);
});

test('login with valid credentials redirects to dashboard', async ({ page }) => {
  await login(page);
  await expect(page).toHaveURL(/dashboard/);
});

test('dashboard shows sidebar navigation', async ({ page }) => {
  await login(page);
  await expect(page.locator('#sidebar-nav')).toBeVisible();
});
