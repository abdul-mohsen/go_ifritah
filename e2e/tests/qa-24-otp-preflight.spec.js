// QA-24: OTP modal pre-flight validation.
//
// Before opening the OTP modal, the UI must block on:
//   1. company.name_ar empty                  → daemon Taxpayer.CompanyNameAR
//   2. zatca_city empty                       → daemon Taxpayer.City
//   3. zatca_csr_org_unit / org_name contain  → CSR rejects
//      forbidden chars: ! @ # $ % & * _ <
// The Connect button is gated by zatcaConfigSaved + filled fields, so
// we mock the save flow to enable it before invoking the OTP modal.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

const REQUIRED_FIELDS = [
  'zatca_csr_org_identifier', 'zatca_csr_org_unit', 'zatca_csr_org_name',
  'zatca_csr_country', 'zatca_csr_location', 'zatca_csr_business_category',
  'zatca_seller_vat', 'zatca_seller_crn',
  'zatca_street', 'zatca_building', 'zatca_district', 'zatca_postal_code',
];

async function goToZatcaTab(page) {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-branch-select')).toBeVisible();
  await page.waitForTimeout(400);
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
  await page.fill('#zatca_street', 'شارع الملك فهد');
  await page.fill('#zatca_building', '1234');
  await page.fill('#zatca_district', 'العليا');
  await page.fill('#zatca_postal_code', '12345');
  await page.evaluate((ids) => {
    for (const id of ids) {
      const el = document.getElementById(id);
      if (el) el.dispatchEvent(new Event('blur', { bubbles: true }));
    }
  }, REQUIRED_FIELDS);
}

async function mockSavePath(page, companyNameAr) {
  await page.route('**/api/company', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200, contentType: 'application/json',
        body: JSON.stringify({ detail: { name: 'ACME', name_ar: companyNameAr } }),
      });
      return;
    }
    await route.continue();
  });
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
}

async function modalVisible(page) {
  return await page.locator('#zatca-otp-modal').isVisible();
}

test.describe('OTP modal pre-flight validation', () => {
  test('opens normally when all fields are valid', async ({ page }) => {
    await mockSavePath(page, 'أكمي');
    await goToZatcaTab(page);
    await fillAllFields(page);
    await page.fill('#zatca_city', 'الرياض');

    // Save first to enable the connect button
    await page.click('#zatca-save-btn');
    await page.waitForTimeout(600);

    await page.click('#zatca-connect-btn');
    await page.waitForTimeout(600);
    expect(await modalVisible(page)).toBe(true);
  });

  test('blocks when name_ar is empty', async ({ page }) => {
    await mockSavePath(page, ''); // empty name_ar
    await goToZatcaTab(page);
    await fillAllFields(page);
    await page.fill('#zatca_city', 'الرياض');
    await page.click('#zatca-save-btn');
    await page.waitForTimeout(600);

    await page.click('#zatca-connect-btn');
    await page.waitForTimeout(600);
    expect(await modalVisible(page)).toBe(false);
  });

  test('blocks when city is empty', async ({ page }) => {
    await mockSavePath(page, 'أكمي');
    await goToZatcaTab(page);
    await fillAllFields(page);
    // Save with city, then clear city before clicking connect
    await page.fill('#zatca_city', 'الرياض');
    await page.click('#zatca-save-btn');
    await page.waitForTimeout(600);

    await page.fill('#zatca_city', '');
    await page.click('#zatca-connect-btn');
    await page.waitForTimeout(600);
    expect(await modalVisible(page)).toBe(false);
  });
});
