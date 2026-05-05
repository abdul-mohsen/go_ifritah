// Search-reset behavior: typing 'a' then 'b' must produce ?q=b (NOT q=a&q=b).
// This is the regression test for the URL-accumulation bug.
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_LIST } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_LIST) {
  test(`search reset on ${r.name}: q=a → q=b never accumulates`, async ({ page }) => {
    await login(page);
    await page.goto(r.path);
    const q = page.locator('input[name="q"]');
    if (!(await q.isVisible().catch(() => false))) test.skip();

    await q.fill('a');
    await page.waitForFunction(() => /[\?&]q=a(&|$)/.test(location.search), { timeout: 5000 });
    expect(page.url(), 'after typing a').toMatch(/[\?&]q=a(&|$)/);

    await q.fill('');
    await q.fill('b');
    await page.waitForFunction(() => /[\?&]q=b(&|$)/.test(location.search) && !/q=a(&|$)/.test(location.search), { timeout: 5000 });
    const url = page.url();
    expect(url, 'q=b only').toMatch(/[\?&]q=b(&|$)/);
    expect(url, 'no leftover q=a').not.toMatch(/q=a(&|$)/);

    // Empty params must not litter the URL
    expect(url, 'no empty sort=').not.toMatch(/[\?&]sort=(&|$)/);
    expect(url, 'no empty dir=').not.toMatch(/[\?&]dir=(&|$)/);
  });
}
