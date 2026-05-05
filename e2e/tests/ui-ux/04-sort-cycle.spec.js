// Sort cycle: clicking a sortable header cycles none → asc → desc → none.
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_LIST } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_LIST) {
  test(`sort cycle on ${r.name}`, async ({ page }) => {
    await login(page);
    await page.goto(r.path);
    const th = page.locator('th[data-sortable]').first();
    if (!(await th.count())) test.skip();

    const key = await th.getAttribute('data-sort-key');
    expect(key, 'sort key').toBeTruthy();

    await th.click();
    await page.waitForFunction(k => new RegExp(`sort=${k}`).test(location.search) && /dir=asc/.test(location.search), key, { timeout: 5000 });
    expect(page.url()).toMatch(new RegExp(`sort=${key}`));
    expect(page.url()).toMatch(/dir=asc/);

    await th.click();
    await page.waitForFunction(() => /dir=desc/.test(location.search), { timeout: 5000 });
    expect(page.url()).toMatch(/dir=desc/);

    await th.click();
    await page.waitForFunction(() => !/sort=\w+/.test(location.search) || !/dir=(asc|desc)/.test(location.search), { timeout: 5000 });
    expect(page.url()).not.toMatch(/dir=asc|dir=desc/);
  });
}
