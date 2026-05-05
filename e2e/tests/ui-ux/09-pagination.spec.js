// Pagination: per-page selector triggers an HTMX request and the URL reflects
// the new per value. List pages must not full-reload.
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_LIST } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_LIST) {
  test(`per-page change updates URL on ${r.name}`, async ({ page }) => {
    await login(page);
    await page.goto(r.path);
    const sel = page.locator('select[name="per"]').first();
    if (!(await sel.count())) test.skip();
    await page.evaluate(() => { document.getElementById('toast-container').dataset.tag = 'pre'; });
    await sel.selectOption({ index: Math.min(1, await sel.locator('option').count() - 1) });
    await page.waitForFunction(() => /[\?&]per=\d+/.test(location.search), { timeout: 5000 });
    expect(page.url()).toMatch(/[\?&]per=\d+/);
    const tag = await page.evaluate(() => document.getElementById('toast-container').dataset.tag);
    expect(tag, 'no full reload').toBe('pre');
  });
}
