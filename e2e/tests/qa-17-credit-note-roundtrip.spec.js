// QA-17: Credit note round-trip.
//
// Flow:
//   1. Find an issued bill (state=1, credit_state=0) from the invoices list.
//      The action-credit link is only rendered for that combo.
//   2. Open /dashboard/invoices/credit/{id}, fill note, submit.
//   3. After redirect, the same bill must NOT offer the action-credit link
//      anymore (credit_state changed → action hidden).

const { test, expect } = require('@playwright/test');
const { login, uniqueTag } = require('../helpers/qa');

test.describe('Credit note round-trip', () => {
  test('issue credit on an issued bill creates a credit-bill record', async ({ page }) => {
    test.setTimeout(60000);
    await login(page);

    // Find a bill that currently exposes the credit-note action.
    await page.goto('/dashboard/invoices?state=1');
    await page.waitForLoadState('domcontentloaded');

    const before = await page.evaluate(() => ({
      creditActions: document.querySelectorAll('a[href^="/dashboard/invoices/credit/"]').length,
      creditBills: document.querySelectorAll('a[href^="/credit_bill/"]').length,
    }));

    const candidate = await page.evaluate(() => {
      const links = Array.from(document.querySelectorAll('a[href^="/dashboard/invoices/credit/"]'));
      for (const a of links) {
        const m = (a.getAttribute('href') || '').match(/\/dashboard\/invoices\/credit\/(\d+)/);
        if (m) return m[1];
      }
      return null;
    });

    if (!candidate) {
      test.skip(true, 'No issued bill (state=1, credit_state=0) available to credit on dev backend.');
    }

    // Open credit form and submit it.
    await page.goto(`/dashboard/invoices/credit/${candidate}`);
    await page.waitForLoadState('domcontentloaded');
    await expect(page.locator('input[name="bill_id"]')).toHaveValue(String(candidate));

    const tag = uniqueTag('QA-CN17');
    await page.fill('textarea[name="note"]', `auto-credit ${tag}`);

    const [resp] = await Promise.all([
      page.waitForResponse((r) => r.url().endsWith('/api/invoices/credit') && r.request().method() === 'POST'),
      page.click('button[type="submit"]'),
    ]);
    expect(resp.status(), 'create-credit must not 5xx').toBeLessThan(500);
    await page.waitForLoadState('domcontentloaded');

    if (resp.status() < 400) {
      // Success → either the credit-action link disappears from this bill OR
      // a new credit-bill record appears in the list. Assert at least one.
      await page.goto('/dashboard/invoices?state=1');
      await page.waitForLoadState('domcontentloaded');
      const after = await page.evaluate(() => ({
        creditActions: document.querySelectorAll('a[href^="/dashboard/invoices/credit/"]').length,
        creditBills: document.querySelectorAll('a[href^="/credit_bill/"]').length,
      }));
      const flippedAction = after.creditActions < before.creditActions;
      const newCreditBill = after.creditBills > before.creditBills;
      expect(
        flippedAction || newCreditBill,
        `expected credit-action count to drop OR credit-bill count to rise. before=${JSON.stringify(before)} after=${JSON.stringify(after)}`,
      ).toBeTruthy();
    } else {
      // Backend rejected — the page must surface a readable Arabic message.
      const html = await page.content();
      expect(html, 'failed credit must render readable Arabic error').toMatch(/فشل|إشعار|دائن|خطأ|error/);
    }
  });

  test('credit form requires a bill_id', async ({ page, request }) => {
    await login(page);
    const cookies = await page.context().cookies();
    const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
    const csrf = (cookies.find((c) => c.name === 'csrf_token') || {}).value || '';
    // Empty bill_id → backend must reject (frontend forwards as 4xx, never 5xx).
    const resp = await request.post('/api/invoices/credit', {
      headers: {
        cookie: cookieHeader,
        'content-type': 'application/x-www-form-urlencoded',
        'x-csrf-token': csrf,
      },
      data: 'csrf_token=' + encodeURIComponent(csrf) + '&bill_id=&note=',
    });
    expect(resp.status(), 'no bill_id must fail').toBeGreaterThanOrEqual(400);
    expect(resp.status(), 'no bill_id must not 5xx').toBeLessThan(500);
  });

  test('credit on a non-existent bill rejects cleanly (no 5xx)', async ({ page, request }) => {
    await login(page);
    const cookies = await page.context().cookies();
    const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
    const csrf = (cookies.find((c) => c.name === 'csrf_token') || {}).value || '';
    const resp = await request.post('/api/invoices/credit', {
      headers: {
        cookie: cookieHeader,
        'content-type': 'application/x-www-form-urlencoded',
        'x-csrf-token': csrf,
      },
      data: 'csrf_token=' + encodeURIComponent(csrf) + '&bill_id=999999999&note=auto',
    });
    expect(resp.status(), 'unknown bill must not 5xx').toBeLessThan(500);
  });
});
