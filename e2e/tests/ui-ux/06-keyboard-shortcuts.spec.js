// Keyboard a11y: Ctrl+K focuses the search input (declared in script.js shortcut block).
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_LIST } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_LIST) {
  test(`Ctrl+K focuses search on ${r.name}`, async ({ page }) => {
    await login(page);
    await page.goto(r.path);
    const q = page.locator('input[name="q"]').first();
    if (!(await q.isVisible().catch(() => false))) test.skip();
    // Synthesize the Ctrl+K keydown via DOM dispatch so we test the
    // application's listener rather than the browser-level shortcut
    // (which Playwright/Chromium can intercept).
    await page.evaluate(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, bubbles: true }));
    });
    const focused = await page.evaluate(() => document.activeElement && document.activeElement.getAttribute('name'));
    expect(focused).toBe('q');
  });
}
