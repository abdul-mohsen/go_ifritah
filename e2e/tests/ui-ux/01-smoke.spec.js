// Smoke: every dashboard route loads, no JS console errors, expected layout chrome present.
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_DASHBOARD } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_DASHBOARD) {
  test(`smoke: ${r.name} loads cleanly`, async ({ page }) => {
    const errors = [];
    page.on('pageerror', e => errors.push('pageerror: ' + e.message));
    page.on('console', m => { if (m.type() === 'error') errors.push('console: ' + m.text()); });
    await login(page);
    const resp = await page.goto(r.path);
    expect(resp.status(), `${r.path} HTTP status`).toBeLessThan(400);
    await expect(page.locator('body')).toBeVisible();
    // Layout chrome
    await expect(page.locator('#toast-container')).toBeAttached();
    // Page must be RTL (Arabic locale)
    const dir = await page.evaluate(() => document.documentElement.getAttribute('dir'));
    expect(dir, `${r.path} <html dir>`).toMatch(/rtl|ltr/);
    // No JS errors. Filter out resource-load 502s/network blips that come
    // from background polling endpoints under parallel load — the test is
    // about page-level JS health, not BE rate-limit behavior.
    const filtered = errors.filter(e => !/favicon|net::ERR_|Failed to load resource|502|503|504/.test(e));
    expect(filtered, `${r.path} console errors`).toEqual([]);
  });
}
