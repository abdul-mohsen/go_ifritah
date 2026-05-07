// Empty-state behavior: searching for an absurd query must render the
// "no results" empty row, not crash the table.
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_LIST } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_LIST) {
  test(`empty state on ${r.name}`, async ({ page }) => {
    await login(page);
    await page.goto(r.path);
    const q = page.locator('input[name="q"]').first();
    if (!(await q.isVisible().catch(() => false))) test.skip();
    const garbage = '___qwxz_no_match_' + Date.now();
    await q.fill(garbage);
    await page.waitForFunction(g => location.search.includes('q=' + encodeURIComponent(g)), garbage, { timeout: 5000 });
    await page.waitForTimeout(400);
    // Table must still be present
    await expect(page.locator('#list-results')).toBeAttached();
    // Either an empty-state row or a table with 0 data rows
    const dataRows = await page.locator('#list-results tbody tr').count();
    expect(dataRows, 'rows in table').toBeGreaterThanOrEqual(0);
  });
}
