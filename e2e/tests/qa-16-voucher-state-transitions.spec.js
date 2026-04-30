// QA-16: Cash voucher state transition round-trip.
//
// Creates a draft voucher → approves it → posts it, asserting each
// transition via the detail page (status badge + which action buttons
// are shown). Per the template, state codes are:
//   0 = draft  (Edit / Approve / Delete buttons)
//   1 = approved (Post button)
//   2 = posted (no action buttons; immutable)

const { test, expect } = require('@playwright/test');
const { login, uniqueTag } = require('../helpers/qa');

async function findVoucherIdByRecipient(page, needle) {
  await page.goto('/dashboard/cash-vouchers?q=' + encodeURIComponent(needle));
  await page.waitForLoadState('domcontentloaded');
  const id = await page.evaluate((n) => {
    const rows = Array.from(document.querySelectorAll('tr'));
    for (const r of rows) {
      if (r.textContent && r.textContent.includes(n)) {
        const a = r.querySelector('a[href*="/dashboard/cash-vouchers/"]');
        if (a) {
          const m = a.getAttribute('href').match(/\/cash-vouchers\/(\d+)(?:\/|$)/);
          if (m) return m[1];
        }
      }
    }
    return null;
  }, needle);
  return id;
}

async function createDraftVoucher(page, tag) {
  await page.goto('/dashboard/cash-vouchers/add');
  await page.waitForLoadState('domcontentloaded');
  await expect(page.locator('input[name="effective_date"]')).not.toHaveValue('');
  await page.fill('input[name="amount"]', '5.55');
  await page.fill('input[name="recipient_name"]', tag);
  await page.fill('textarea[name="description"]', `qa-16 ${tag}`);
  const [resp] = await Promise.all([
    page.waitForResponse((r) => r.url().endsWith('/api/cash-vouchers') && r.request().method() === 'POST'),
    page.click('button[type="submit"]'),
  ]);
  expect(resp.status(), 'create voucher status').toBeLessThan(400);
  await page.waitForLoadState('domcontentloaded');
  return await findVoucherIdByRecipient(page, tag);
}

test.describe('Cash voucher state transitions', () => {
  test('full round-trip: draft → approved → posted (or graceful permission error)', async ({ page }) => {
    test.setTimeout(60000);
    await login(page);

    const tag = uniqueTag('QA-CV16');
    const id = await createDraftVoucher(page, tag);
    expect(id, `voucher id for ${tag} must be findable on list page`).toBeTruthy();

    // --- DRAFT detail: must offer Approve + Edit + Delete buttons. ---
    await page.goto(`/dashboard/cash-vouchers/${id}`);
    await page.waitForLoadState('domcontentloaded');
    const draftHtml = await page.content();
    expect(draftHtml, 'draft must expose approve form').toContain(`/api/cash-vouchers/${id}/approve`);
    expect(draftHtml, 'draft must NOT yet expose post form').not.toContain(`/api/cash-vouchers/${id}/post`);

    // --- APPROVE: capture response status to handle either success or backend-403. ---
    page.once('dialog', (d) => d.accept());
    const [approveResp] = await Promise.all([
      page.waitForResponse((r) => /\/api\/cash-vouchers\/\d+\/approve$/.test(r.url())),
      page.locator(`form[action="/api/cash-vouchers/${id}/approve"] button[type="submit"]`).click(),
    ]);
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(300);

    expect(approveResp.status(), 'approve must not 5xx').toBeLessThan(500);

    if (approveResp.status() < 400) {
      // Success path → post action must now appear.
      const approvedHtml = await page.content();
      expect(approvedHtml, 'after approve, post action must be available').toContain(`/api/cash-vouchers/${id}/post`);
      expect(approvedHtml, 'after approve, approve action must be gone').not.toContain(`/api/cash-vouchers/${id}/approve`);

      // --- POST: accept dialog and submit. ---
      page.once('dialog', (d) => d.accept());
      const [postResp] = await Promise.all([
        page.waitForResponse((r) => /\/api\/cash-vouchers\/\d+\/post$/.test(r.url())),
        page.locator(`form[action="/api/cash-vouchers/${id}/post"] button[type="submit"]`).click(),
      ]);
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(300);
      expect(postResp.status(), 'post must not 5xx').toBeLessThan(500);

      if (postResp.status() < 400) {
        const postedHtml = await page.content();
        expect(postedHtml, 'after post, no approve action').not.toContain(`/api/cash-vouchers/${id}/approve`);
        expect(postedHtml, 'after post, no post action (immutable)').not.toContain(`/api/cash-vouchers/${id}/post`);
      }
    } else {
      // Permission/backend rejection path → frontend MUST surface a clean error
      // page (no 5xx, no blank screen). The error template renders a <pre> block.
      const errHtml = await page.content();
      expect(errHtml, 'failed approve must render an error message to the user').toMatch(/فشل|error|خطأ|اعتماد/);
      // The voucher must remain in draft state — approve action still present.
      await page.goto(`/dashboard/cash-vouchers/${id}`);
      await page.waitForLoadState('domcontentloaded');
      const stillDraft = await page.content();
      expect(stillDraft, 'failed approve must leave voucher in draft state').toContain(`/api/cash-vouchers/${id}/approve`);
    }
  });

  test('approving a non-existent voucher returns a clean error (no 5xx)', async ({ page, request }) => {
    await login(page);
    const cookies = await page.context().cookies();
    const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
    const csrf = (cookies.find((c) => c.name === 'csrf_token') || {}).value || '';
    const resp = await request.post('/api/cash-vouchers/999999999/approve', {
      headers: {
        cookie: cookieHeader,
        'content-type': 'application/x-www-form-urlencoded',
        'x-csrf-token': csrf,
      },
      data: 'csrf_token=' + encodeURIComponent(csrf),
    });
    expect(resp.status(), 'approve non-existent must not 5xx').toBeLessThan(500);
  });

  test('posting a draft (skipping approve) is rejected', async ({ page, request }) => {
    await login(page);
    const tag = uniqueTag('QA-CV16-NOAPP');
    const id = await createDraftVoucher(page, tag);
    expect(id).toBeTruthy();

    const cookies = await page.context().cookies();
    const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
    const csrf = (cookies.find((c) => c.name === 'csrf_token') || {}).value || '';
    const resp = await request.post(`/api/cash-vouchers/${id}/post`, {
      headers: {
        cookie: cookieHeader,
        'content-type': 'application/x-www-form-urlencoded',
        'x-csrf-token': csrf,
      },
      data: 'csrf_token=' + encodeURIComponent(csrf),
    });
    // Backend should refuse (typically 400) — must not 5xx and must not 2xx-redirect to success.
    expect(resp.status(), 'post-without-approve must fail').toBeGreaterThanOrEqual(400);
    expect(resp.status(), 'post-without-approve must not 5xx').toBeLessThan(500);
  });
});
