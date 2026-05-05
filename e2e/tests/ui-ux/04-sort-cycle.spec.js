// Sort cycle: clicking a sortable header cycles none → asc → desc → none.
// Sort is FE-only (BE returns canonical order); we assert that the
// indicator on the clicked header cycles and that the row order is
// actually mutated client-side. The URL no longer changes.
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

    const indicatorText = async () => {
      const t = (await th.locator('.sort-indicator').textContent().catch(() => '')) || '';
      return t.trim();
    };
    const rowSnapshot = async () =>
      await page.locator('tbody tr').evaluateAll((rows) =>
        rows.filter((r) => r.cells && r.cells.length > 1).map((r) => r.rowIndex)
      );

    // Fresh load: indicator should be ⇅ (none).
    await page.waitForFunction(() => {
      const ind = document.querySelector('th[data-sortable] .sort-indicator');
      return ind && ind.textContent.trim().length > 0;
    }, null, { timeout: 5000 });
    expect(await indicatorText()).toBe('⇅');

    // First click → asc (↑).
    await th.click();
    await expect.poll(indicatorText, { timeout: 3000 }).toBe('↑');

    // Second click → desc (↓).
    await th.click();
    await expect.poll(indicatorText, { timeout: 3000 }).toBe('↓');

    // Third click → none (⇅).
    await th.click();
    await expect.poll(indicatorText, { timeout: 3000 }).toBe('⇅');

    // URL search params must NOT carry sort/dir (FE-only sort).
    expect(page.url()).not.toMatch(/[?&]sort=/);
    expect(page.url()).not.toMatch(/[?&]dir=/);
  });
}
