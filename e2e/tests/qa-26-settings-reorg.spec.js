// QA-26: Settings tab reorganization — no duplicate inputs, fields on
// correct tabs.
//
// Regression guards from the recent UI cleanup:
//   - Exactly ONE element with id="company_name" in the DOM.
//   - Exactly ONE element with id="company_name_ar".
//   - The General tab's Company Info card has only company_email and
//     company_description (legacy KV settings) — NO duplicate name input.
//   - The ZATCA tab's Company Info card has the EN+AR name inputs that
//     write to /api/v2/company.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.describe('Settings tab reorganization', () => {
  test('no duplicate company_name / company_name_ar in DOM', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/settings');
    await page.waitForLoadState('domcontentloaded');

    await expect(page.locator('#company_name')).toHaveCount(1);
    await expect(page.locator('#company_name_ar')).toHaveCount(1);
  });

  test('General tab Company Info has email and description only — not name', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/settings');
    await page.waitForLoadState('domcontentloaded');
    // General is the default visible tab
    await expect(page.locator('input[name="company_email"]')).toBeVisible();
    await expect(page.locator('textarea[name="company_description"]')).toBeVisible();
    // company_name should NOT be in a panel-general form name attribute
    const hasNameInGeneralForm = await page.evaluate(() => {
      const general = document.getElementById('panel-general');
      if (!general) return false;
      return !!general.querySelector('input[name="company_name"]');
    });
    expect(hasNameInGeneralForm).toBe(false);
  });

  test('ZATCA tab Company Info card has EN and AR name inputs', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/settings');
    await page.click('#tab-zatca');
    await page.waitForTimeout(200);
    await expect(page.locator('#company_name')).toBeVisible();
    await expect(page.locator('#company_name_ar')).toBeVisible();
    // direction must be RTL on AR input
    const dir = await page.locator('#company_name_ar').getAttribute('dir');
    expect(dir).toBe('rtl');
  });

  test('VAT/CRN are NOT on General tab (moved to per-branch ZATCA)', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/settings');
    await page.waitForLoadState('domcontentloaded');
    const generalHasVat = await page.evaluate(() => {
      const g = document.getElementById('panel-general');
      if (!g) return false;
      return !!g.querySelector('input[name="company_vat"], input[name="company_cr"]');
    });
    expect(generalHasVat).toBe(false);
  });
});
