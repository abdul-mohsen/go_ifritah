const { test, expect } = require('@playwright/test');
const { login, appURL } = require('../helpers/qa');

// The generated template's example Store ID/Supplier ID/Branch ID are just
// illustrative placeholders - re-uploading the raw template unedited only
// succeeds if those placeholders happen to be real ids for the account
// that downloaded it. Fetch real ones from the account's own add-bill
// forms and ask the template endpoint to bake them in instead (see
// HandleDownloadBillImportTemplate's store_id/supplier_id/branch_id
// overrides), so this test exercises the actual import logic rather than
// depending on lucky placeholder values.
async function realAccountIds(page, kind) {
  if (kind === 'sales') {
    await page.goto('/dashboard/invoices/add');
    await page.waitForLoadState('domcontentloaded');
    const storeId = await page.locator('[name="store_id"]').first().inputValue();
    const branchId = await page.locator('[name="branch_id"]').first().inputValue();
    return { store_id: storeId, branch_id: branchId };
  }
  await page.goto('/dashboard/purchase-bills/add');
  await page.waitForLoadState('domcontentloaded');
  const storeId = await page.locator('[name="store_id"]').first().inputValue();
  const supplierId = await page.locator('[name="supplier_id"]').first().inputValue();
  return { store_id: storeId, supplier_id: supplierId };
}

async function assertToolbarAndImport(page, kind) {
  const listURL = kind === 'sales' ? '/dashboard/invoices' : '/dashboard/purchase-bills';
  const expectedType = kind === 'sales' ? 'sales' : 'purchase';
  const accountIds = await realAccountIds(page, kind);
  await page.goto(listURL);

  const actions = page.locator('.toolbar-actions [data-bill-import], .toolbar-actions [data-bill-export]');
  await expect(actions).toHaveCount(2);
  await expect(actions.nth(0)).toHaveAttribute('data-bill-import', '');
  await expect(actions.nth(1)).toHaveAttribute('data-bill-export', '');
  for (const action of [actions.nth(0), actions.nth(1)]) {
    await expect(action.locator('svg')).toBeVisible();
  }
  await expect(actions.nth(0)).toHaveText(/استيراد|Import/);
  await expect(actions.nth(1)).toHaveText(/تصدير|Export/);
  await expect(actions.nth(0)).toHaveAttribute('href', new RegExp(`bill-import\\?type=${expectedType}`));
  const exportURL = await actions.nth(1).getAttribute('href');

  await actions.nth(0).click();
  await expect(page).toHaveURL(new RegExp(`/dashboard/bill-import\\?type=${expectedType}`));
  await expect(page.locator('#bill-import-type')).toHaveValue(expectedType);
  const templateURL = await page.locator('#bill-import-template').getAttribute('href');
  expect(templateURL).toContain(`type=${expectedType}`);

  const templateWithRealIds = appURL(templateURL) + '&' + new URLSearchParams(accountIds).toString();
  const templateResponse = await page.request.get(templateWithRealIds);
  expect(templateResponse.ok()).toBeTruthy();
  expect(templateResponse.headers()['content-type']).toContain(
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
  );
  const template = await templateResponse.body();
  expect(template.subarray(0, 2).toString()).toBe('PK');

  await page.locator('#bill-import-file').setInputFiles({
    name: `${kind}-multiple-bills.xlsx`,
    mimeType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    buffer: template,
  });
  await page.locator('#bill-import-submit').click();
  await expect(page.locator('#bill-import-result')).toContainText(/2.*2/);
  await expect(page.locator('#bill-import-result')).toHaveAttribute('data-success', '2');

  const exportResponse = await page.request.get(exportURL);
  expect(exportResponse.ok(), `${kind} export failed with ${exportResponse.status()}`).toBeTruthy();
  expect(exportResponse.headers()['content-type']).toContain(
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
  );
}

test.describe('Shared Excel bill import and export', () => {
  test('sales flow imports a multiple-bill workbook through the shared page', async ({ page }) => {
    await login(page);
    await assertToolbarAndImport(page, 'sales');
  });

  test('purchase flow imports a multiple-bill workbook through the shared page', async ({ page }) => {
    await login(page);
    await assertToolbarAndImport(page, 'purchase');
  });
});
