// QA-31: Draft bill submission through the ZATCA-backed invoice path.
//
// This covers the user-facing flow QA uses: create a branch-backed draft bill,
// open its detail page, click Convert to Invoice, and verify the bill leaves
// draft state. It also verifies that a second submit is rejected cleanly.

const { test, expect } = require('@playwright/test');
const { login, uniqueTag } = require('../helpers/qa');

async function collectDraftIds(page) {
  await page.goto('/dashboard/invoices?state=0&per=100');
  await page.waitForLoadState('domcontentloaded');
  return page.evaluate(() => Array.from(document.querySelectorAll('a[href^="/bill/"]'))
    .map((link) => {
      const match = (link.getAttribute('href') || '').match(/^\/bill\/(\d+)$/);
      return match ? match[1] : null;
    })
    .filter(Boolean));
}

async function findDraftByTag(page, tag, preferredIds = []) {
  const ids = preferredIds.length > 0 ? preferredIds : await collectDraftIds(page);
  for (const id of ids.slice(0, 30)) {
    await page.goto(`/bill/${id}`);
    await page.waitForLoadState('domcontentloaded');
    const body = await page.locator('body').innerText();
    if (body.includes(tag)) return id;
  }
  return null;
}

async function createBranchBackedDraft(page, tag) {
  const before = new Set(await collectDraftIds(page));

  await page.goto('/dashboard/invoices/add-invoice');
  await page.waitForLoadState('domcontentloaded');

  const storeOptions = await page.locator('select[name="store_id"] option').count();
  const branchOptions = await page.locator('select[name="branch_id"] option').count();
  test.skip(storeOptions === 0 || branchOptions === 0, 'No store/branch options available to create a ZATCA-backed draft bill.');

  await page.selectOption('select[name="payment_method"]', '10');
  await page.fill('input[name="deliver_date"]', '2026-05-01');
  await page.fill('input[name="user_name"]', `QA ZATCA ${tag}`);
  await page.fill('input[name="user_phone_number"]', '0500000000');
  await page.fill('input[name="note"]', `QA-31 ZATCA submit ${tag}`);

  await page.fill('input[name="manual_part_name"]', `ZATCA manual ${tag}`);
  await page.fill('input[name="manual_part_number"]', `QA31-${tag.slice(-6)}`);
  await page.fill('input[name="manual_quantity"]', '1');
  await page.fill('input[name="manual_price"]', '100');
  await page.evaluate(() => recalculateTotal());

  const [createResp] = await Promise.all([
    page.waitForResponse((resp) => resp.url().endsWith('/api/invoices') && resp.request().method() === 'POST'),
    page.locator('button[name="state"][value="0"]').click(),
  ]);
  const createStatus = createResp.status();
  const createText = createStatus >= 400 ? await createResp.text().catch(() => '') : '';
  expect(createStatus, `draft create must succeed: ${createText}`).toBeLessThan(400);
  await page.waitForURL('**/dashboard/invoices**', { timeout: 15000 }).catch(() => {});

  const after = await collectDraftIds(page);
  const newIds = after.filter((id) => !before.has(id));
  const created = await findDraftByTag(page, tag, newIds);
  expect(created, `created draft should be discoverable by note tag ${tag}`).toBeTruthy();
  return created;
}

test.describe('ZATCA-backed draft bill submit', () => {
  test('branch-backed draft bill submits from detail and leaves draft state', async ({ page, request }) => {
    // FIXME(ci): requires ZATCA-onboarded branch + valid CSR/cert chain on the
    // shared dev backend, which isn't reliably present. Re-enable once the
    // CI seed migration provisions a fully-onboarded branch.
    test.fixme(true, 'dev backend lacks reliable ZATCA-onboarded branch for submit');
    test.setTimeout(90000);
    await login(page);

    const tag = uniqueTag('QA31-ZATCA');
    const billId = await createBranchBackedDraft(page, tag);

    await page.goto(`/bill/${billId}`);
    await page.waitForLoadState('domcontentloaded');
    await expect(page.locator(`button[hx-post="/api/invoices/${billId}/submit"]`)).toBeVisible();

    page.once('dialog', (dialog) => dialog.accept());
    const [submitResp] = await Promise.all([
      page.waitForResponse((resp) => resp.url().endsWith(`/api/invoices/${billId}/submit`) && resp.request().method() === 'POST', { timeout: 60000 }),
      page.locator(`button[hx-post="/api/invoices/${billId}/submit"]`).click(),
    ]);

    expect(submitResp.status(), 'draft submit should reach backend successfully').toBeLessThan(400);
    await page.waitForLoadState('domcontentloaded');
    await page.goto(`/bill/${billId}`);
    await page.waitForLoadState('domcontentloaded');
    await expect(page.locator(`button[hx-post="/api/invoices/${billId}/submit"]`)).toHaveCount(0);

    const draftIds = await collectDraftIds(page);
    expect(draftIds, 'submitted bill must no longer be listed as draft').not.toContain(String(billId));

    const cookies = await page.context().cookies();
    const cookie = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
    const csrf = (cookies.find((c) => c.name === 'csrf_token') || {}).value || '';
    const secondSubmit = await request.post(`/api/invoices/${billId}/submit`, {
      headers: { cookie, 'x-csrf-token': csrf },
    });
    expect(secondSubmit.status(), 'second submit must be rejected cleanly').toBeGreaterThanOrEqual(400);
    expect(secondSubmit.status(), 'second submit must not 5xx').toBeLessThan(500);
    await expect(await secondSubmit.text()).toMatch(/ليست مسودة|draft|مسودة|فشل|error/i);
  });
});