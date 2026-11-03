const { test, expect } = require('@playwright/test');
const { login, appURL } = require('../helpers/qa');

const UUID_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

// A sales import keeps one session per input in sessionStorage. Sales imports are off until the duplicate check is live,
// so these cases call the helper that the import pages use.
test.describe('Sales import session', () => {
  test('keeps the session for the same input, across a reload, and starts a new one for another input or after the import ends', async ({ page }) => {
    await login(page);
    await page.goto(appURL('/dashboard'));
    await page.waitForFunction(() => Boolean(window.saleImportSession));
    const begin = (fingerprint) => page.evaluate((value) => window.saleImportSession.begin(value), fingerprint);

    const first = await begin('sales.xlsx|1024|1700000000000');
    expect(first).toMatch(UUID_V4);

    await page.reload();
    await page.waitForFunction(() => Boolean(window.saleImportSession));
    expect(await begin('sales.xlsx|1024|1700000000000')).toBe(first);

    const other = await begin('other.xlsx|2048|1700000000001');
    expect(other).not.toBe(first);

    await page.evaluate(() => window.saleImportSession.end());
    expect(await page.evaluate(() => window.sessionStorage.getItem('afrita_sale_import'))).toBeNull();

    const next = await begin('other.xlsx|2048|1700000000001');
    expect(next).toMatch(UUID_V4);
    expect(next).not.toBe(other);
  });
});
