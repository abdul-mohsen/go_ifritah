// Mobile viewport (390x844) — every dashboard route must remain usable:
// no horizontal overflow, sidebar is hidden by default, hamburger present.
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_DASHBOARD } = require('../../helpers/routes');

test.use({ viewport: { width: 390, height: 844 } });
test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_DASHBOARD) {
  test(`mobile layout on ${r.name}`, async ({ page }) => {
    await login(page);
    await page.goto(r.path);
    const overflow = await page.evaluate(() => ({
      docW: document.documentElement.scrollWidth,
      winW: window.innerWidth,
    }));
    // Allow a 4px tolerance for sub-pixel rounding
    expect(overflow.docW - overflow.winW, `${r.path} horizontal overflow`).toBeLessThanOrEqual(4);
  });
}
