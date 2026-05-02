// QA-14: Concurrency — two browser contexts hit the settings form
// simultaneously to ensure no 5xx and last-write-wins.

const { test, expect } = require('@playwright/test');
const { login, setSetting } = require('../helpers/qa');

test('two simultaneous setting saves both succeed', async ({ browser }) => {
  const ctx1 = await browser.newContext();
  const ctx2 = await browser.newContext();
  const p1 = await ctx1.newPage();
  const p2 = await ctx2.newPage();

  await login(p1);
  await login(p2);

  await Promise.all([
    setSetting(p1, 'low_stock_threshold', '11'),
    setSetting(p2, 'low_stock_threshold', '12'),
  ]);

  // Final value is one of the two — the system must not 500.
  await p1.goto('/dashboard/settings');
  const final = await p1.locator('[name="low_stock_threshold"]').inputValue();
  expect(['11', '12']).toContain(final);

  await ctx1.close();
  await ctx2.close();
});
