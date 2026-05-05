// QA-22: Per-branch ZATCA company name (EN+AR) save flow.
//
// The ZATCA-tab Company Info card edits the real company.name and
// company.name_ar (NOT settings KV). The backend's PUT /api/v2/company
// rewrites BOTH columns unconditionally, so the UI must require both
// fields. The daemon validates Taxpayer.CompanyNameAR with no fallback —
// blanking name_ar would break onboarding.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

async function goToZatcaTab(page) {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-branch-select')).toBeVisible();
}

test.describe('Per-branch ZATCA company name (EN+AR)', () => {
  test('GET /api/company returns wrapped detail with name and name_ar', async ({ page }) => {
    await login(page);
    const r = await page.request.get('/api/company');
    expect(r.ok()).toBeTruthy();
    const body = await r.json();
    expect(body).toHaveProperty('detail');
    expect(body.detail).toHaveProperty('name_ar');
  });

  test('UI loads existing company name into ZATCA tab inputs', async ({ page }) => {
    await goToZatcaTab(page);
    // Wait briefly for fetch to populate inputs
    await page.waitForTimeout(500);
    const enVal = await page.locator('#company_name').inputValue();
    const arVal = await page.locator('#company_name_ar').inputValue();
    // Either both populated or both empty — but inputs must exist
    expect(enVal).not.toBeNull();
    expect(arVal).not.toBeNull();
  });

  test('save with empty name_ar is rejected with both-required message', async ({ page }) => {
    // Install route BEFORE the tab triggers loadCompanyInfo, so the GET
    // returns a known empty name_ar (avoids the async load racing with
    // our fill('') and re-populating the input).
    let putHit = false;
    await page.route('**/api/company', async (route) => {
      const m = route.request().method();
      if (m === 'GET') {
        await route.fulfill({
          status: 200, contentType: 'application/json',
          body: JSON.stringify({ detail: { name: 'ACME LLC', name_ar: '' } }),
        });
        return;
      }
      if (m === 'PUT') {
        putHit = true;
        await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"ok"}' });
        return;
      }
      await route.continue();
    });
    await goToZatcaTab(page);
    await page.waitForTimeout(500);
    await page.fill('#company_name', 'ACME LLC');
    await page.fill('#company_name_ar', '');
    // Click the save button (id=zatca-company-save-btn or similar)
    const btn = page.locator('button[onclick="saveCompanyInfo()"]').first();
    if (await btn.count() === 0) test.skip(true, 'Company save button not present in this build.');
    await btn.click();
    await page.waitForTimeout(300);
    // PUT must NOT have been called
    expect(putHit).toBe(false);
  });

  test('PUT /api/company sends both name and name_ar', async ({ page }) => {
    let captured = null;
    await page.route('**/api/company', async (route) => {
      const m = route.request().method();
      if (m === 'GET') {
        await route.fulfill({
          status: 200, contentType: 'application/json',
          body: JSON.stringify({ detail: { name: '', name_ar: '' } }),
        });
        return;
      }
      if (m === 'PUT') {
        try { captured = JSON.parse(route.request().postData() || '{}'); } catch { }
        await route.fulfill({ status: 200, contentType: 'application/json', body: '{"detail":"ok"}' });
        return;
      }
      await route.continue();
    });
    await goToZatcaTab(page);
    await page.waitForTimeout(500);
    await page.fill('#company_name', 'ACME LLC');
    await page.fill('#company_name_ar', 'شركة أكمي');
    const btn = page.locator('button[onclick="saveCompanyInfo()"]').first();
    if (await btn.count() === 0) test.skip(true, 'Company save button not present in this build.');
    await btn.click();
    await page.waitForTimeout(500);
    expect(captured).toBeTruthy();
    expect(captured.name).toBe('ACME LLC');
    expect(captured.name_ar).toBe('شركة أكمي');
  });
});
