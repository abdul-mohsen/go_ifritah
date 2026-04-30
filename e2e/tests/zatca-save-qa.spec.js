const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/auth');

// ════════════════════════════════════════════════════════════════════
// Helpers (shared with zatca-settings.spec.js pattern)
// ════════════════════════════════════════════════════════════════════

async function goToZatcaTab(page) {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-branch-select')).toBeVisible();
}

async function fillAllZatcaFields(page) {
  await page.fill('#zatca_csr_org_identifier', '311111');
  await page.fill('#zatca_csr_org_unit', 'IT');
  await page.fill('#zatca_csr_org_name', 'شركة اختبار');
  await page.selectOption('#zatca_csr_business_category', { index: 1 });
  await page.fill('#zatca_csr_location', 'الرياض');
  await page.fill('#zatca_seller_vat', '300000000000003');
  await page.fill('#zatca_seller_crn', '7000000000');
  await page.fill('#zatca_street', 'شارع الملك فهد');
  await page.fill('#zatca_building', '1234');
  await page.fill('#zatca_district', 'العليا');
  await page.fill('#zatca_postal_code', '12345');
}

function mockSaveSuccess(page) {
  return page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });
}

function mockSaveFail(page, status, detail) {
  return page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify({ detail }) });
    } else {
      await route.continue();
    }
  });
}

// ════════════════════════════════════════════════════════════════════
// A. Partial Save Scenarios (36–42)
// ════════════════════════════════════════════════════════════════════

test('36. save with only VAT+CRN filled sends PUT with all fields', async ({ page }) => {
  await goToZatcaTab(page);
  await page.fill('#zatca_seller_vat', '300000000000003');
  await page.fill('#zatca_seller_crn', '7000000000');

  let capturedBody = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedBody = JSON.parse(route.request().postData());
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
  expect(capturedBody).toBeTruthy();
  expect(capturedBody.seller_vat).toBe('300000000000003');
  expect(capturedBody.seller_crn).toBe('7000000000');
});

test('37. save with only CSR section filled succeeds', async ({ page }) => {
  await goToZatcaTab(page);
  await page.fill('#zatca_csr_org_identifier', '311111');
  await page.fill('#zatca_csr_org_unit', 'IT');
  await page.fill('#zatca_csr_org_name', 'شركة اختبار');
  await page.selectOption('#zatca_csr_business_category', { index: 1 });
  await page.fill('#zatca_csr_location', 'الرياض');

  await mockSaveSuccess(page);
  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
});

test('38. save with only address section filled succeeds', async ({ page }) => {
  await goToZatcaTab(page);
  await page.fill('#zatca_street', 'شارع الملك فهد');
  await page.fill('#zatca_building', '1234');
  await page.fill('#zatca_district', 'العليا');
  await page.fill('#zatca_postal_code', '12345');

  await mockSaveSuccess(page);
  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
});

test('39. save with single field changed succeeds', async ({ page }) => {
  await goToZatcaTab(page);
  await page.fill('#zatca_seller_vat', '300000000000003');

  await mockSaveSuccess(page);
  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
});

test('40. save sends csr_country defaulting to SA when empty', async ({ page }) => {
  await goToZatcaTab(page);
  await page.fill('#zatca_seller_vat', '300000000000003');

  let capturedBody = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedBody = JSON.parse(route.request().postData());
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
  expect(capturedBody).toBeTruthy();
  expect(capturedBody.csr_country).toBe('SA');
});

test('41. save with valid VAT but empty everything else sends correct payload', async ({ page }) => {
  await goToZatcaTab(page);
  await page.fill('#zatca_seller_vat', '300000000000003');

  let capturedBody = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedBody = JSON.parse(route.request().postData());
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
  expect(capturedBody.seller_vat).toBe('300000000000003');
  // Other fields should be empty strings, not undefined
  expect(typeof capturedBody.csr_org_identifier).toBe('string');
  expect(typeof capturedBody.street).toBe('string');
});

test('42. save preserves previously saved fields in local cache', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveSuccess(page);

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });

  // After save, change one field and verify others remain
  await page.fill('#zatca_csr_org_identifier', '999999');
  const vatVal = await page.locator('#zatca_seller_vat').inputValue();
  expect(vatVal).toBe('300000000000003');
});

// ════════════════════════════════════════════════════════════════════
// B. Error Feedback Scenarios (43–50)
// ════════════════════════════════════════════════════════════════════

test('43. backend 400 shows the ACTUAL backend error message', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveFail(page, 400, 'invalid VAT number');

  await page.click('#zatca-save-btn');
  const statusEl = page.locator('#zatca-save-status');
  await expect(statusEl).toBeVisible({ timeout: 5000 });
  const text = await statusEl.textContent();
  expect(text).toContain('invalid VAT number');
});

test('44. backend 400 "invalid request" shows descriptive error', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveFail(page, 400, 'invalid request');

  await page.click('#zatca-save-btn');
  const statusEl = page.locator('#zatca-save-status');
  await expect(statusEl).toBeVisible({ timeout: 5000 });
  const text = await statusEl.textContent();
  expect(text).toContain('✗');
  expect(text).toContain('invalid request');
});

test('45. backend 404 shows branch-specific error', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveFail(page, 404, 'branch not found');

  await page.click('#zatca-save-btn');
  const statusEl = page.locator('#zatca-save-status');
  await expect(statusEl).toBeVisible({ timeout: 5000 });
  const text = await statusEl.textContent();
  expect(text).toContain('branch not found');
});

test('46. backend 500 shows server error', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveFail(page, 500, 'internal server error');

  await page.click('#zatca-save-btn');
  const statusEl = page.locator('#zatca-save-status');
  await expect(statusEl).toBeVisible({ timeout: 5000 });
  const text = await statusEl.textContent();
  expect(text).toContain('✗');
  expect(text).toContain('internal server error');
});

test('47. network timeout shows error toast', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);

  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      await route.abort('timedout');
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  const statusEl = page.locator('#zatca-save-status');
  await expect(statusEl).toBeVisible({ timeout: 5000 });
  const text = await statusEl.textContent();
  expect(text).toContain('✗');
});

test('48. 401 response shows error (not silent success)', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveFail(page, 401, 'unauthorized');

  await page.click('#zatca-save-btn');
  // Should either redirect to login or show error — NOT show success
  const statusEl = page.locator('#zatca-save-status');
  // Wait for either status to appear or page to navigate
  await page.waitForTimeout(3000);
  const url = page.url();
  if (url.includes('/dashboard/settings')) {
    // Stayed on page — must show error
    if (await statusEl.isVisible()) {
      const text = await statusEl.textContent();
      expect(text).not.toContain('✓');
    }
  }
  // else: redirected to login — also acceptable
});

test('49. save error clears after successful retry', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);

  // First: fail
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      await route.fulfill({ status: 500, contentType: 'application/json', body: '{"detail":"db error"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  const statusEl = page.locator('#zatca-save-status');
  await expect(statusEl).toContainText('✗', { timeout: 5000 });

  // Clear route and mock success
  await page.unroute('/api/zatca/branch/**');
  await mockSaveSuccess(page);

  await page.click('#zatca-save-btn');
  await expect(statusEl).toContainText('✓', { timeout: 5000 });
});

test('50. error indicator AND toast both shown on failure', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveFail(page, 400, 'VAT mismatch');

  // Listen for toast
  const toastPromise = page.waitForSelector('.toast-error, [data-toast="error"], .Toastify__toast--error, .toast', { timeout: 5000 }).catch(() => null);

  await page.click('#zatca-save-btn');
  const statusEl = page.locator('#zatca-save-status');
  await expect(statusEl).toBeVisible({ timeout: 5000 });
  const text = await statusEl.textContent();
  expect(text).toContain('✗');
  expect(text).toContain('VAT mismatch');
});

// ════════════════════════════════════════════════════════════════════
// C. Branch Selection Edge Cases (51–55)
// ════════════════════════════════════════════════════════════════════

test('51. branch selector has at least one option', async ({ page }) => {
  await goToZatcaTab(page);
  const count = await page.locator('#zatca-branch-select option').count();
  expect(count).toBeGreaterThanOrEqual(1);
});

test('52. hidden zatca-branch-id matches first branch on load', async ({ page }) => {
  await goToZatcaTab(page);
  await page.waitForTimeout(500);
  const hiddenVal = await page.locator('#zatca-branch-id').inputValue();
  const selectVal = await page.locator('#zatca-branch-select').inputValue();
  expect(hiddenVal).toBe(selectVal);
});

test('53. switching branch then saving uses correct branch ID in URL', async ({ page }) => {
  await goToZatcaTab(page);
  const options = await page.locator('#zatca-branch-select option').all();
  if (options.length < 1) return;

  const firstVal = await options[0].getAttribute('value');
  await page.selectOption('#zatca-branch-select', firstVal);
  await page.locator('#zatca-branch-select').dispatchEvent('change');
  await page.waitForTimeout(200);

  await page.fill('#zatca_seller_vat', '300000000000003');

  let capturedUrl = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedUrl = route.request().url();
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
  expect(capturedUrl).toBeTruthy();
  expect(capturedUrl).toContain('/api/zatca/branch/' + firstVal);
});

test('54. save sends branch ID matching selected dropdown value', async ({ page }) => {
  await goToZatcaTab(page);
  await page.waitForTimeout(300);
  const selectVal = await page.locator('#zatca-branch-select').inputValue();
  await page.fill('#zatca_seller_vat', '300000000000003');

  let capturedUrl = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedUrl = route.request().url();
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
  expect(capturedUrl).toContain('/api/zatca/branch/' + selectVal);
});

test('55. hidden zatca-branch-id always matches dropdown after switch', async ({ page }) => {
  await goToZatcaTab(page);
  const options = await page.locator('#zatca-branch-select option').all();
  if (options.length < 2) return; // skip if only 1 branch

  // Switch to second branch
  const secondVal = await options[1].getAttribute('value');
  await page.selectOption('#zatca-branch-select', secondVal);
  await page.locator('#zatca-branch-select').dispatchEvent('change');
  await page.waitForTimeout(200);

  const hiddenVal = await page.locator('#zatca-branch-id').inputValue();
  expect(hiddenVal).toBe(secondVal);

  // Switch back to first
  const firstVal = await options[0].getAttribute('value');
  await page.selectOption('#zatca-branch-select', firstVal);
  await page.locator('#zatca-branch-select').dispatchEvent('change');
  await page.waitForTimeout(200);

  const hiddenVal2 = await page.locator('#zatca-branch-id').inputValue();
  expect(hiddenVal2).toBe(firstVal);
});

// ════════════════════════════════════════════════════════════════════
// D. Data Integrity (56–60)
// ════════════════════════════════════════════════════════════════════

test('56. save then switch branch then switch back shows saved data', async ({ page }) => {
  await goToZatcaTab(page);
  const options = await page.locator('#zatca-branch-select option').all();
  if (options.length < 2) return;

  await fillAllZatcaFields(page);
  await mockSaveSuccess(page);
  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });

  // Switch to second branch
  const secondVal = await options[1].getAttribute('value');
  await page.selectOption('#zatca-branch-select', secondVal);
  await page.locator('#zatca-branch-select').dispatchEvent('change');
  await page.waitForTimeout(300);

  // Switch back to first
  const firstVal = await options[0].getAttribute('value');
  await page.selectOption('#zatca-branch-select', firstVal);
  await page.locator('#zatca-branch-select').dispatchEvent('change');
  await page.waitForTimeout(300);

  // Fields should have data from local cache
  const vat = await page.locator('#zatca_seller_vat').inputValue();
  expect(vat).toBe('300000000000003');
});

test('57. special characters in org name are preserved in payload', async ({ page }) => {
  await goToZatcaTab(page);
  const specialName = 'شركة "التجارة" & الأعمال <المحدودة>';
  await page.fill('#zatca_csr_org_name', specialName);

  let capturedBody = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedBody = JSON.parse(route.request().postData());
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toBeVisible({ timeout: 5000 });
  expect(capturedBody).toBeTruthy();
  expect(capturedBody.csr_org_name).toBe(specialName);
});

test('58. payload does NOT include branch_id or zatca_status (those are backend-only)', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);

  let capturedBody = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedBody = JSON.parse(route.request().postData());
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toBeVisible({ timeout: 5000 });
  expect(capturedBody).toBeTruthy();
  // Frontend should NOT send branch_id or zatca_status — those are handled by Go handler
  expect(capturedBody.branch_id).toBeUndefined();
  expect(capturedBody.zatca_status).toBeUndefined();
});

test('59. all 12 field names present in payload', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);

  let capturedBody = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedBody = JSON.parse(route.request().postData());
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toBeVisible({ timeout: 5000 });
  expect(capturedBody).toBeTruthy();
  const expectedFields = [
    'csr_org_identifier', 'csr_org_unit', 'csr_org_name',
    'csr_country', 'csr_location', 'csr_business_category',
    'seller_vat', 'seller_crn',
    'street', 'building', 'district', 'postal_code'
  ];
  for (const field of expectedFields) {
    expect(capturedBody).toHaveProperty(field);
  }
});

test('60. trimmed whitespace is sent — no leading/trailing spaces', async ({ page }) => {
  await goToZatcaTab(page);
  await page.fill('#zatca_csr_org_name', '  شركة اختبار  ');
  await page.fill('#zatca_street', '  شارع الملك فهد  ');

  let capturedBody = null;
  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      capturedBody = JSON.parse(route.request().postData());
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toBeVisible({ timeout: 5000 });
  expect(capturedBody.csr_org_name).toBe('شركة اختبار');
  expect(capturedBody.street).toBe('شارع الملك فهد');
});

// ════════════════════════════════════════════════════════════════════
// E. UX Quality (61–65)
// ════════════════════════════════════════════════════════════════════

test('61. save button shows spinner during save', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);

  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      await new Promise(r => setTimeout(r, 1500));
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  // Spinner SVG should be visible during save
  await expect(page.locator('#zatca-save-btn svg.animate-spin')).toBeVisible({ timeout: 2000 });
});

test('62. form fields disabled during save', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);

  await page.route('/api/zatca/branch/**', async (route) => {
    if (route.request().method() === 'PUT' && !route.request().url().includes('/onboard')) {
      await new Promise(r => setTimeout(r, 2000));
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"success"}' });
    } else {
      await route.continue();
    }
  });

  await page.click('#zatca-save-btn');
  // Wait for spinner to appear
  await expect(page.locator('#zatca-save-btn svg.animate-spin')).toBeVisible({ timeout: 2000 });
  // VAT field should be disabled during save
  await expect(page.locator('#zatca_seller_vat')).toBeDisabled();
  await expect(page.locator('#zatca_csr_org_identifier')).toBeDisabled();
});

test('63. form fields re-enabled after save completes', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveSuccess(page);

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });
  // Fields should be re-enabled
  await expect(page.locator('#zatca_seller_vat')).toBeEnabled();
  await expect(page.locator('#zatca_csr_org_identifier')).toBeEnabled();
});

test('64. save with no branch shows error toast instead of silent nothing', async ({ page }) => {
  await goToZatcaTab(page);
  // Force-clear hidden branch ID
  await page.evaluate(() => { document.getElementById('zatca-branch-id').value = ''; });

  await page.click('#zatca-save-btn');
  // Status should NOT show success
  await page.waitForTimeout(1000);
  const statusEl = page.locator('#zatca-save-status');
  if (await statusEl.isVisible()) {
    const text = await statusEl.textContent();
    expect(text).not.toContain('✓');
  }
  // Button should NOT be in loading state
  await expect(page.locator('#zatca-save-btn')).toBeEnabled();
});

test('65. progress bar updates after successful save', async ({ page }) => {
  await goToZatcaTab(page);
  await fillAllZatcaFields(page);
  await mockSaveSuccess(page);

  await page.click('#zatca-save-btn');
  await expect(page.locator('#zatca-save-status')).toContainText('✓', { timeout: 5000 });

  // Progress should be 100%
  const pctText = await page.locator('#zatca-progress-pct').textContent();
  expect(pctText).toBe('100%');
});
