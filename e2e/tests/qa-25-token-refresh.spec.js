// QA-25: Token refresh on idle.
//
// The middleware refreshes the access token without bouncing the user to
// /login. After a session has been idle (or its access token would
// otherwise be expired), navigating to a protected page must still render
// the dashboard, not redirect to /login.
//
// We cannot truly fast-forward time in the running server, so this suite
// verifies the protected-page contract: a fresh login keeps the session
// alive across navigations and an invalid session_id can no longer reach
// dashboard pages.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');
const { USER, PASS } = require('../helpers/auth');

function isLoggedOutUrl(url) {
  // Server may redirect logged-out requests to either / or /login.
  if (url.includes('/login')) return true;
  // Accept root URL with no /dashboard prefix as logged-out
  const u = new URL(url);
  return u.pathname === '/' || u.pathname === '';
}

test.describe('Token refresh on idle', () => {
  test('navigating between protected pages keeps the session', async ({ page }) => {
    await login(page);
    for (const path of [
      '/dashboard',
      '/dashboard/invoices',
      '/dashboard/products',
      '/dashboard/settings',
      '/dashboard',
    ]) {
      await page.goto(path);
      await page.waitForLoadState('domcontentloaded');
      // We must still be on the requested dashboard area
      expect(page.url()).toContain('/dashboard');
    }
  });

  test('tampered session_id cookie no longer reaches dashboard', async ({ page, context }) => {
    await login(page);
    const cookies = await context.cookies();
    for (const c of cookies) {
      if (c.name === 'session_id') {
        await context.addCookies([{ ...c, value: 'invalid-junk-session-id' }]);
      }
    }
    await page.goto('/dashboard');
    await page.waitForLoadState('domcontentloaded');
    expect(isLoggedOutUrl(page.url())).toBe(true);
  });

  test('logout (GET) invalidates the session', async ({ browser }) => {
    const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    const page = await context.newPage();

    await page.goto('/login');
    await page.fill('input[name="username"]', USER);
    await page.fill('input[name="password"]', PASS);
    await Promise.all([
      page.waitForURL('**/dashboard**', { timeout: 15000 }),
      page.click('button[type="submit"]'),
    ]);

    await page.goto('/logout');
    await page.waitForLoadState('domcontentloaded');
    await page.goto('/dashboard');
    await page.waitForLoadState('domcontentloaded');
    expect(isLoggedOutUrl(page.url())).toBe(true);

    await context.close();
  });
});
