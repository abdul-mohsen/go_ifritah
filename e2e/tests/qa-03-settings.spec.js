// QA-03: Every settings toggle saves and persists.

const { test, expect } = require('@playwright/test');
const { login, setSetting } = require('../helpers/qa');

test.beforeEach(async ({ page }) => { await login(page); });

const NUMERIC = [
  ['low_stock_threshold', '7'],
  ['vat_rate', '15'],
];
const SELECTS = [
  ['stock_enforcement', 'warn'],
  ['stock_enforcement', 'enforce'],
  ['stock_enforcement', 'disable'],
  ['pb_pdf_required', 'optional'],
  ['pb_pdf_required', 'required'],
  ['paper_size', 'A4'],
  ['currency', 'SAR'],
  ['default_payment_method', '10'],
];
const CHECKBOXES = [
  ['allow_negative_stock', true],
  ['allow_negative_stock', false],
  ['track_inventory', true],
  ['show_vat_breakdown', true],
  ['auto_calculate_vat', true],
  ['prices_include_vat', false],
  ['show_logo_print', true],
  ['show_company_info_print', true],
  ['show_qr_print', true],
  ['show_bank_details', false],
];

for (const [key, val] of NUMERIC) {
  test(`numeric setting ${key}=${val}`, async ({ page }) => {
    await setSetting(page, key, val);
    await page.goto('/dashboard/settings');
    await expect(page.locator(`[name="${key}"]`)).toHaveValue(String(val));
  });
}

for (const [key, val] of SELECTS) {
  test(`select setting ${key}=${val}`, async ({ page }) => {
    await setSetting(page, key, val);
    await page.goto('/dashboard/settings');
    await expect(page.locator(`[name="${key}"]`)).toHaveValue(val);
  });
}

for (const [key, val] of CHECKBOXES) {
  test(`checkbox setting ${key}=${val}`, async ({ page }) => {
    await setSetting(page, key, val);
    await page.goto('/dashboard/settings');
    const cb = page.locator(`[name="${key}"]`).first();
    if (val) await expect(cb).toBeChecked();
    else await expect(cb).not.toBeChecked();
  });
}
