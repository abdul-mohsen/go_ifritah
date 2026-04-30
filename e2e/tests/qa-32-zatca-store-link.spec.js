// QA-32: Adding a store from the ZATCA "no store linked" panel must link
// it to the selected branch.
//
// Bug it covers: the add-store form did not include a `branch_id` field,
// so a store created via "/dashboard/stores/add" had no branch link. The
// ZATCA settings panel reads `/api/branch/{id}/store-address` which checks
// `branch.stores[0]` — with no link, it kept saying "no store" forever
// even after the user added one.
//
// What this test exercises:
//   1. Login as admin.
//   2. Open settings → ZATCA tab, pick the first branch.
//   3. Click the "create store" deep link from the no-store panel.
//   4. The add-store form must include a Branch <select> with the
//      branch pre-selected (via ?branch_id=N query param).
//   5. Submit the form with a unique name + city.
//   6. Go back to settings → ZATCA → same branch.
//   7. The "no store" warning must be gone and the address-source panel
//      must be visible (i.e. linked successfully).

const { test, expect } = require('@playwright/test');

async function loginAsAdmin(page) {
  await page.goto('/login');
  await page.fill('input[name="username"]', 'admin');
  await page.fill('input[name="password"]', 'admin');
  await Promise.all([
    page.waitForURL('**/dashboard**', { timeout: 15000 }),
    page.click('button[type="submit"]'),
  ]);
}

async function pickFirstBranchId(page) {
  await page.goto('/dashboard/settings');
  await page.waitForLoadState('domcontentloaded');
  // Force the ZATCA tab so the dropdown is rendered.
  await page.evaluate(() => window.switchSettingsTab && window.switchSettingsTab('zatca'));
  const sel = page.locator('#zatca-branch-select');
  await sel.waitFor({ state: 'visible', timeout: 10000 });
  // Pick the first non-empty option.
  const id = await sel.evaluate((el) => {
    for (const o of el.options) if (o.value) return o.value;
    return '';
  });
  return id || null;
}

test.describe('QA-32 store ↔ branch link from ZATCA panel', () => {
  test('add-store form pre-selects branch_id from query string', async ({ page }) => {
    await loginAsAdmin(page);
    const branchId = await pickFirstBranchId(page);
    test.skip(!branchId, 'No branch on dev backend.');

    await page.goto(`/dashboard/stores/add?branch_id=${branchId}`);
    await page.waitForLoadState('domcontentloaded');

    const select = page.locator('select[name="branch_id"]');
    await expect(select, 'add-store form must expose branch_id select').toHaveCount(1);
    const selectedValue = await select.inputValue();
    expect(selectedValue, 'branch_id query param must preselect the branch').toBe(String(branchId));
  });

  test('creating a store from the ZATCA deep link links it to the branch', async ({ page }) => {
    await loginAsAdmin(page);
    const branchId = await pickFirstBranchId(page);
    test.skip(!branchId, 'No branch on dev backend.');

    // Verify the deep link the UI exposes points at this branch.
    await page.evaluate((id) => window.switchZatcaBranch && window.switchZatcaBranch(id), branchId);
    const href = await page.locator('#zatca-create-store-link').getAttribute('href');
    expect(href, 'deep link must include branch_id').toContain(`branch_id=${branchId}`);

    // Drive the form.
    const storeName = `QA32-${Date.now()}`;
    await page.goto(href);
    await page.waitForLoadState('domcontentloaded');
    await page.fill('input[name="name"]', storeName);
    await page.fill('input[name="city"]', 'Riyadh');
    await page.fill('input[name="building_number"]', '1234');
    await page.fill('input[name="postal_code"]', '12345');
    await Promise.all([
      page.waitForURL('**/dashboard/stores**', { timeout: 15000 }),
      page.locator('form button[type="submit"]').first().click(),
    ]);

    // Re-query the bridge: the branch must now report a linked store.
    const r = await page.request.get(`/api/branch/${branchId}/store-address`);
    expect(r.ok()).toBeTruthy();
    const body = await r.json();
    expect(body.linked, 'branch must report linked=true after store creation').toBe(true);
    expect(body.store_id, 'store_id must be populated').toBeTruthy();
  });
});
