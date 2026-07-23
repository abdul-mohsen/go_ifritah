const fs = require('fs');

const { test, expect } = require('@playwright/test');

async function downloadTemplate(page, testInfo) {
  await expect(page.locator('button[onclick="downloadImportTemplate()"]')).toBeVisible();
  const response = await page.evaluate(async () => {
    const result = await fetch('/api/purchase-bills/import-template', {
      credentials: 'include',
    });
    return {
      ok: result.ok,
      contentDisposition: result.headers.get('content-disposition') || '',
      contentType: result.headers.get('content-type') || '',
      body: await result.text(),
    };
  });
  const templatePath = testInfo.outputPath('purchase-bill-template.csv');
  fs.writeFileSync(templatePath, response.body, 'utf8');
  return { response, templatePath };
}

function writeFilledTemplate(templatePath, rows, outputName, testInfo) {
  const templateBody = fs.readFileSync(templatePath, 'utf8');
  const header = templateBody
    .split(/\r?\n/)
    .find(Boolean)
    .replace(/^\uFEFF/, '');
  const csvBody = [header, ...rows.map((row) => row.join(','))].join('\n') + '\n';
  const uploadPath = testInfo.outputPath(outputName);
  fs.writeFileSync(uploadPath, csvBody, 'utf8');
  return uploadPath;
}

async function searchForSequence(page, sequence) {
  const searchInput = page.locator('input[name="q"]').first();
  await expect(searchInput).toBeVisible();
  await searchInput.fill(sequence);
  await searchInput.press('Enter');
  await expect(page.locator('#list-results tbody tr').filter({ hasText: sequence }).first()).toBeVisible();
}

test.describe('Purchase-bill CSV import flow', () => {
  test('downloads CSV sample, appends repeated uploads, and submits imported items', async ({ page }, testInfo) => {
    const firstUploadRows = [
      ['فلتر زيت E2E', '2', '33.50', '28.25', 'A-11'],
      ['بواجي E2E', '1', '12.00', '10.00', 'B-22'],
    ];
    const secondUploadRows = [['سير مكينة E2E', '4', '5.00', '4.00', 'C-33']];
    const sequence = String(Date.now()).slice(-10);

    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');
    await expect(page).toHaveURL(/\/dashboard\/purchase-bills\/add$/);
    await expect(page.locator('#purchase-form')).toBeVisible();

    const { response, templatePath } = await downloadTemplate(page, testInfo);
    expect(response.ok).toBeTruthy();
    expect(response.contentDisposition).toContain('purchase-bill-template.csv');
    expect(response.contentType).toContain('text/csv');

    const templateBody = fs.readFileSync(templatePath, 'utf8');
    expect(templateBody).toContain('اسم القطعة,الكمية,سعر الشراء,سعر التكلفة,رقم الرف');
    expect(templateBody).toContain('مثال,10,100.00,90.00,A1');

    const firstUploadPath = writeFilledTemplate(templatePath, firstUploadRows, 'purchase-bill-import-1.csv', testInfo);
    const secondUploadPath = writeFilledTemplate(
      templatePath,
      secondUploadRows,
      'purchase-bill-import-2.csv',
      testInfo
    );

    await page.locator('[name="supplier_sequance_number"]').fill(sequence);
    await page.locator('[name="payment_date"]').fill('2026-07-08');

    const pdfInput = page.locator('[name="bill_pdf"]');
    if (await pdfInput.count()) {
      await pdfInput.setInputFiles({
        name: 'purchase-bill-import.pdf',
        mimeType: 'application/pdf',
        buffer: Buffer.from('%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n'),
      });
    }

    const importInput = page.locator('input[type="file"][accept=".csv,text/csv"]');

    const firstParseResponsePromise = page.waitForResponse(
      (response) => response.url().includes('/api/purchase-bills/parse-csv') && response.request().method() === 'POST'
    );
    await importInput.setInputFiles(firstUploadPath);
    expect((await firstParseResponsePromise).ok()).toBeTruthy();

    const productRows = page.locator('#products-container .item-row');
    await expect(productRows).toHaveCount(2);

    const firstRow = productRows.nth(0);
    await expect(firstRow.locator('.store-product-search')).toHaveValue('فلتر زيت E2E');
    await expect(firstRow.locator('[name="products_part_name"]')).toHaveValue('فلتر زيت E2E');
    await expect(firstRow.locator('[name="products_product_id"]')).toHaveValue('0');
    await expect(firstRow.locator('[name="products_quantity"]')).toHaveValue('2');
    await expect(firstRow.locator('[name="products_price"]')).toHaveValue('33.5');
    await expect(firstRow.locator('[name="products_cost_price"]')).toHaveValue('28.25');
    await expect(firstRow.locator('[name="products_shelf_number"]')).toHaveValue('A-11');

    const secondRow = productRows.nth(1);
    await expect(secondRow.locator('.store-product-search')).toHaveValue('بواجي E2E');
    await expect(secondRow.locator('[name="products_quantity"]')).toHaveValue('1');
    await expect(secondRow.locator('[name="products_price"]')).toHaveValue('12');
    await expect(secondRow.locator('[name="products_cost_price"]')).toHaveValue('10');
    await expect(secondRow.locator('[name="products_shelf_number"]')).toHaveValue('B-22');
    await expect(page.locator('#total_amount')).toHaveValue('90.85');

    const secondParseResponsePromise = page.waitForResponse(
      (response) => response.url().includes('/api/purchase-bills/parse-csv') && response.request().method() === 'POST'
    );
    await importInput.setInputFiles(secondUploadPath);
    expect((await secondParseResponsePromise).ok()).toBeTruthy();

    await expect(productRows).toHaveCount(3);
    await expect(firstRow.locator('.store-product-search')).toHaveValue('فلتر زيت E2E');
    await expect(secondRow.locator('.store-product-search')).toHaveValue('بواجي E2E');

    const thirdRow = productRows.nth(2);
    await expect(thirdRow.locator('.store-product-search')).toHaveValue('سير مكينة E2E');
    await expect(thirdRow.locator('[name="products_part_name"]')).toHaveValue('سير مكينة E2E');
    await expect(thirdRow.locator('[name="products_product_id"]')).toHaveValue('0');
    await expect(thirdRow.locator('[name="products_quantity"]')).toHaveValue('4');
    await expect(thirdRow.locator('[name="products_price"]')).toHaveValue('5');
    await expect(thirdRow.locator('[name="products_cost_price"]')).toHaveValue('4');
    await expect(thirdRow.locator('[name="products_shelf_number"]')).toHaveValue('C-33');
    await expect(page.locator('#total_amount')).toHaveValue('113.85');

    const submitResponsePromise = page.waitForResponse(
      (response) => response.url().includes('/api/purchase-bills') && response.request().method() === 'POST'
    );
    await page.locator('#purchase-form button[type="submit"]').click();
    expect((await submitResponsePromise).ok()).toBeTruthy();

    await page.waitForURL(/\/dashboard\/purchase-bills(?:\?.*)?$/);
    await searchForSequence(page, sequence);

    const billRow = page.locator('#list-results tbody tr').filter({ hasText: sequence }).first();
    const viewLink = billRow.locator('a.action-view').first();
    await expect(viewLink).toBeVisible();
    await viewLink.click();

    await page.waitForURL(/\/dashboard\/purchase-bills\/\d+$/);

    // Imported rows never resolve to a real catalog product_id (the CSV
    // import only types free-text names - it doesn't match against
    // inventory), so the unified item form classifies all three as manual
    // products, not catalog products. This mirrors the same
    // product_id-driven catalog/manual split already covered for manual
    // form entry by TestCreatePBManualItemsOnlyInManualProducts.
    // The detail page renders one merged items table (catalog + manual)
    // with a per-row type badge instead of two separately-grouped tables -
    // all 3 imported rows should show the "manually added" badge.
    const itemsSection = page
      .locator('.section-card')
      .filter({
        has: page.locator('h4.section-card-title').filter({ hasText: /الأصناف|Items/ }),
      })
      .first();
    await expect(itemsSection).toBeVisible();
    await expect(itemsSection.locator('tbody tr')).toHaveCount(3);
    await expect(itemsSection.locator('.pbi-state-badge.is-manual')).toHaveCount(3);
    await expect(itemsSection.locator('.pbi-state-badge.is-stock')).toHaveCount(0);
    await expect(itemsSection).toContainText('فلتر زيت E2E');
    await expect(itemsSection).toContainText('بواجي E2E');
    await expect(itemsSection).toContainText('سير مكينة E2E');
    await expect(itemsSection).toContainText('33.50');
    await expect(itemsSection).toContainText('12.00');
    await expect(itemsSection).toContainText('5.00');
  });
});
