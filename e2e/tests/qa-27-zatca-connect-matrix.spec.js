// QA-27: ZATCA connect-button precondition matrix.
//
// The Connect button must be:
//   - DISABLED on a branch with empty required fields
//   - DISABLED on a branch with required fields filled but config NOT saved
//   - ENABLED  on a branch with all required fields filled AND config saved

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

const REQUIRED_FIELDS = [
  'zatca_csr_org_identifier', 'zatca_csr_org_unit', 'zatca_csr_org_name',
  'zatca_csr_country', 'zatca_csr_location', 'zatca_csr_business_category',
  'zatca_seller_vat', 'zatca_seller_crn',
  'zatca_street', 'zatca_building', 'zatca_district', 'zatca_postal_code',
];

async function setReadonlyValue(page, id, value) {
  await page.evaluate(({ id, value }) => {
    const el = document.getElementById(id);
    if (!el) return;
    el.value = value;
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
    el.dispatchEvent(new Event('blur', { bubbles: true }));
  }, { id, value });
}

async function ensureStoreLinked(page) {
  await page.evaluate(() => {
    const noStore = document.getElementById('zatca-address-no-store');
    if (noStore) noStore.classList.add('hidden');
    const src = document.getElementById('zatca-address-source');
    if (src) src.classList.remove('hidden');
  });
}

async function goToZatcaTab(page) {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-branch-select')).toBeVisible();
  await page.waitForTimeout(400);
  await ensureStoreLinked(page);
}

// Clear values AND dispatch input events so the in-IIFE listener marks
// zatcaConfigSaved=false and re-runs checkZatcaConnectReady.
async function clearAndFireInput(page) {
  await page.evaluate((ids) => {
    for (const id of ids) {
      const el = document.getElementById(id);
      if (!el) continue;
      el.value = '';
      el.dispatchEvent(new Event('input', { bubbles: true }));
      el.dispatchEvent(new Event('blur', { bubbles: true }));
    }
  }, REQUIRED_FIELDS);
}

async function fillAllFields(page) {
  await page.fill('#zatca_csr_org_identifier', '311111');
  await page.fill('#zatca_csr_org_unit', 'IT');
  await page.fill('#zatca_csr_org_name', 'شركة اختبار');
  await page.selectOption('#zatca_csr_country', 'SA');
  await page.fill('#zatca_csr_location', 'الرياض');
  await page.selectOption('#zatca_csr_business_category', { index: 1 });
  await page.fill('#zatca_seller_vat', '300000000000003');
  await page.fill('#zatca_seller_crn', '7000000000');
  await setReadonlyValue(page, 'zatca_street', 'شارع الملك فهد');
  await setReadonlyValue(page, 'zatca_building', '1234');
  await setReadonlyValue(page, 'zatca_district', 'العليا');
  await setReadonlyValue(page, 'zatca_postal_code', '12345');
  await page.evaluate((ids) => {
    for (const id of ids) {
      const el = document.getElementById(id);
      if (el) el.dispatchEvent(new Event('blur', { bubbles: true }));
    }
  }, REQUIRED_FIELDS);
}

test.describe('ZATCA connect-button precondition matrix', () => {
  test('disabled when required fields are empty', async ({ page }) => {
    await goToZatcaTab(page);
    await clearAndFireInput(page);
    await expect(page.locator('#zatca-connect-btn')).toBeDisabled();
  });

  test('disabled when fields are filled but config not yet saved', async ({ page }) => {
    await goToZatcaTab(page);
    await clearAndFireInput(page);
    await fillAllFields(page);
    await expect(page.locator('#zatca-connect-btn')).toBeDisabled();
  });

  test('enabled after fields are filled AND config saved (mocked)', async ({ page }) => {
    await goToZatcaTab(page);
    await clearAndFireInput(page);
    await fillAllFields(page);

    await page.route('**/api/zatca/branch/**', async (route) => {
      const url = route.request().url();
      if (route.request().method() === 'PUT' && !url.includes('/onboard')) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"ok"}' });
        return;
      }
      await route.continue();
    });
    await page.route('**/api/branch/*/store-address', async (route) => {
      if (route.request().method() === 'PUT') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"ok"}' });
        return;
      }
      await route.continue();
    });

    const cityEl = page.locator('#zatca_city');
    if (await cityEl.count()) {
      await setReadonlyValue(page, 'zatca_city', 'الرياض');
    }

    await page.click('#zatca-save-btn');
    await page.waitForTimeout(800);

    await expect(page.locator('#zatca-connect-btn')).toBeEnabled();
  });
});
