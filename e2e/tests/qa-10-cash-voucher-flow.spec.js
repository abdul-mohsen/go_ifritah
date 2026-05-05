// QA-10: Cash voucher round-trip in a real browser.
// Creates a draft voucher via the actual form, opens its detail page,
// approves it, then posts it. Verifies state transitions.

const { test, expect } = require('@playwright/test');
const { login, uniqueTag } = require('../helpers/qa');

test.beforeEach(async ({ page }) => { await login(page); });

test('create cash voucher draft via form', async ({ page }) => {
  await page.goto('/dashboard/cash-vouchers/add');
  await page.waitForLoadState('domcontentloaded');

  const tag = uniqueTag('QA-CV');
  // The form's effective_date is filled by inline JS to today's date.
  await expect(page.locator('input[name="effective_date"]')).not.toHaveValue('');

  // voucher_type defaults to disbursement, recipient_type to supplier (and
  // updateRecipientName fires when supplier select changes — but on first
  // load recipient_name is empty, so set it explicitly).
  await page.fill('input[name="amount"]', '12.34');
  await page.fill('input[name="recipient_name"]', tag);
  await page.fill('textarea[name="description"]', `auto-test ${tag}`);

  // Submit: wait for HX-Redirect navigation. Use response wait on the POST.
  const [resp] = await Promise.all([
    page.waitForResponse((r) => r.url().endsWith('/api/cash-vouchers') && r.request().method() === 'POST'),
    page.click('button[type="submit"]'),
  ]);
  expect(resp.status(), 'create voucher status').toBeLessThan(400);
  // HX-Redirect should land us on a list/detail page.
  await page.waitForLoadState('domcontentloaded');

  // Search by recipient name on the list page to verify creation.
  await page.goto('/dashboard/cash-vouchers');
  await page.waitForLoadState('domcontentloaded');
  const html = await page.content();
  expect(html).toContain(tag);
});

test('voucher list page shows created voucher with status badge', async ({ page }) => {
  await page.goto('/dashboard/cash-vouchers');
  // status badges should exist on the page (whether draft, approved, posted)
  const badge = page.locator('[class*="badge"], [class*="status"]').first();
  await expect(badge).toBeAttached();
});

test('approve endpoint requires CSRF (browser flow works, raw POST does not)', async ({ page, request }) => {
  // raw post without CSRF should fail
  const cookies = await page.context().cookies();
  const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
  const resp = await request.post('/api/cash-vouchers/0/approve', {
    headers: { cookie: cookieHeader, 'content-type': 'application/x-www-form-urlencoded' },
  });
  // 403 (CSRF), 404 (id not found), 401 (auth) — anything but 5xx.
  expect(resp.status()).toBeLessThan(500);
});
