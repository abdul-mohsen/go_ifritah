// Auth gate: hitting any /dashboard/* route while signed-out must redirect
// to /login (302 or HTML page that lands on /login).
const { test, expect } = require('@playwright/test');
const { ROUTES_DASHBOARD } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_DASHBOARD) {
  test(`auth gate on ${r.name}`, async ({ browser }) => {
    // Explicitly override storageState so this context is anonymous.
    const ctx = await browser.newContext({ ignoreHTTPSErrors: true, storageState: { cookies: [], origins: [] } });
    const page = await ctx.newPage();
    await page.goto(r.path);
    // Accept either /login or the public landing root — both are valid auth gates.
    const url = page.url();
    const onLoginOrRoot = /\/login/.test(url) || new URL(url).pathname === '/';
    expect(onLoginOrRoot, `${r.path} should bounce, got ${url}`).toBe(true);
    // And the response must NOT contain dashboard chrome
    const hasDashboardSidebar = await page.locator('nav .sidebar, [data-sidebar]').count();
    expect(hasDashboardSidebar, `${r.path} leaked dashboard chrome to anon`).toBe(0);
    await ctx.close();
  });
}
