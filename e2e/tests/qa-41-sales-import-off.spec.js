const { test, expect } = require('@playwright/test');
const { login, appURL } = require('../helpers/qa');

// Sales import is off until the duplicate check is live. Its buttons show as disabled, with the reason. Part B removes
// this file with the disabled buttons.
const OFF_MESSAGE = 'Sales import is off until the duplicate check is live.';

test.describe('Sales import is off until the duplicate check is live', () => {
  test('the XLSX sales page disables its import button and says why', async ({ page }) => {
    await login(page);
    await page.goto(appURL('/dashboard/bill-import?type=sales'));
    await expect(page.locator('#bill-import-submit')).toBeDisabled();
    await expect(page.locator('#bill-import-off')).toBeVisible();
    await expect(page.locator('#bill-import-off')).toHaveText(OFF_MESSAGE);
  });

  test('the XLSX purchase page keeps its import button enabled', async ({ page }) => {
    await login(page);
    await page.goto(appURL('/dashboard/bill-import?type=purchase'));
    await expect(page.locator('#bill-import-submit')).toBeEnabled();
    await expect(page.locator('#bill-import-off')).toHaveCount(0);
  });

  test('the CSV import disables its import button and says why', async ({ page }) => {
    await login(page);
    await page.goto(appURL('/dashboard/invoices/import-csv'));
    await page.locator('#tab-csv').click();
    await expect(page.locator('#csv-import-btn')).toBeDisabled();
    await expect(page.locator('#csv-import-off')).toBeVisible();
    await expect(page.locator('#csv-import-off')).toHaveText(OFF_MESSAGE);
  });

  test('the data import disables the sales import on its mapping step', async ({ page }) => {
    await login(page);
    await page.goto(appURL('/dashboard/settings'));
    await page.locator('#tab-import').click();
    await page.locator('#import-file').setInputFiles({
      name: 'bills.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from('part_name,price,quantity\nBrake Pad,75,2\n'),
    });
    await page.locator('#import-preview-btn').click();
    await expect(page.locator('#import-step2')).toBeVisible();
    await expect(page.locator('#import-execute-btn')).toBeDisabled();
    await expect(page.locator('#import-sales-off')).toBeVisible();
    await expect(page.locator('#import-sales-off')).toHaveText(OFF_MESSAGE);
  });
});
