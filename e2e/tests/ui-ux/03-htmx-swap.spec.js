// HTMX swap target test: server response should swap only #list-results,
// the page chrome (toolbar, sidebar, toast container) must remain attached
// to the SAME DOM node before and after the swap (no full reload).
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_LIST } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_LIST) {
  test(`htmx partial swap on ${r.name}`, async ({ page }) => {
    await login(page);
    await page.goto(r.path);
    const q = page.locator('input[name="q"]');
    if (!(await q.isVisible().catch(() => false))) test.skip();

    // Tag the toast container so we can detect a full page reload.
    await page.evaluate(() => { document.getElementById('toast-container').dataset.tag = 'pre'; });

    await q.fill('zzz');
    await page.waitForFunction(() => /[\?&]q=zzz(&|$)/.test(location.search), { timeout: 5000 });
    // Allow the swap to settle
    await page.waitForTimeout(400);

    const tag = await page.evaluate(() => document.getElementById('toast-container').dataset.tag);
    expect(tag, 'toast container preserved (no full reload)').toBe('pre');

    // #list-results should still exist after swap
    await expect(page.locator('#list-results')).toBeAttached();
  });
}
