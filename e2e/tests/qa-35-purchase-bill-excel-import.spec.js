const fs = require('fs');

const { test, expect } = require('@playwright/test');
const XLSX = require('../../static/js/xlsx.full.min.js');

const TEMPLATE_HEADERS = ['اسم القطعة', 'الكمية', 'سعر الشراء', 'سعر التكلفة', 'رقم الرف'];

async function downloadTemplate(page, testInfo) {
  const downloadPromise = page.waitForEvent('download');
  await page.locator('button[onclick="downloadExcelTemplate()"]').click();
  const download = await downloadPromise;
  const templatePath = testInfo.outputPath(download.suggestedFilename() || 'purchase-bill-template.xlsx');
  await download.saveAs(templatePath);
  return templatePath;
}

function writeFilledTemplate(templatePath, rows, testInfo) {
  const workbook = XLSX.read(fs.readFileSync(templatePath), { type: 'buffer' });
  const sheetName = workbook.SheetNames[0];
  workbook.Sheets[sheetName] = XLSX.utils.aoa_to_sheet([TEMPLATE_HEADERS, ...rows]);

  const uploadPath = testInfo.outputPath('purchase-bill-import.xlsx');
  fs.writeFileSync(uploadPath, XLSX.write(workbook, { type: 'buffer', bookType: 'xlsx' }));
  return uploadPath;
}

async function searchForSequence(page, sequence) {
  const searchInput = page.locator('input[name="q"]').first();
  await expect(searchInput).toBeVisible();
  await searchInput.fill(sequence);
  await searchInput.press('Enter');
  await expect(page.locator('#list-results tbody tr').filter({ hasText: sequence }).first()).toBeVisible();
}

test.describe('Purchase-bill Excel import flow', () => {
  test('downloads template, uploads filled workbook, and submits imported items', async ({ page }, testInfo) => {
    const importedRows = [
      ['فلتر زيت E2E', '2', '33.50', '28.25', 'A-11'],
      ['بواجي E2E', '1', '12.00', '10.00', 'B-22'],
    ];
    const sequence = String(Date.now()).slice(-10);

    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');
    await expect(page).toHaveURL(/\/dashboard\/purchase-bills\/add$/);
    await expect(page.locator('#purchase-form')).toBeVisible();

    const templatePath = await downloadTemplate(page, testInfo);
    const uploadPath = writeFilledTemplate(templatePath, importedRows, testInfo);

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

    const parseResponsePromise = page.waitForResponse((response) =>
      response.url().includes('/api/purchase-bills/parse-excel') &&
      response.request().method() === 'POST'
    );
    await page.locator('input[type="file"][accept=".xlsx"]').setInputFiles(uploadPath);
    expect((await parseResponsePromise).ok()).toBeTruthy();

    const productRows = page.locator('#products-container .item-row');
    await expect(productRows).toHaveCount(2);

    const firstRow = productRows.nth(0);
    await expect(firstRow.locator('.part-search')).toHaveValue('فلتر زيت E2E');
    await expect(firstRow.locator('[name="products_part_name"]')).toHaveValue('فلتر زيت E2E');
    await expect(firstRow.locator('[name="products_product_id"]')).toHaveValue('0');
    await expect(firstRow.locator('[name="products_quantity"]')).toHaveValue('2');
    await expect(firstRow.locator('[name="products_price"]')).toHaveValue('33.5');
    await expect(firstRow.locator('[name="products_cost_price"]')).toHaveValue('28.25');
    await expect(firstRow.locator('[name="products_shelf_number"]')).toHaveValue('A-11');

    const secondRow = productRows.nth(1);
    await expect(secondRow.locator('.part-search')).toHaveValue('بواجي E2E');
    await expect(secondRow.locator('[name="products_quantity"]')).toHaveValue('1');
    await expect(secondRow.locator('[name="products_price"]')).toHaveValue('12');
    await expect(secondRow.locator('[name="products_cost_price"]')).toHaveValue('10');
    await expect(secondRow.locator('[name="products_shelf_number"]')).toHaveValue('B-22');

    await expect(page.locator('#total_amount')).toHaveValue('79.00');

    const submitResponsePromise = page.waitForResponse((response) =>
      response.url().includes('/api/purchase-bills') &&
      response.request().method() === 'POST'
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

    const manualSection = page.locator('.section-card').filter({
      has: page.locator('h4.section-card-title').filter({ hasText: /القطع اليدوية|Manual Products/ }),
    }).first();
    await expect(manualSection).toBeVisible();
    await expect(manualSection.locator('tbody tr')).toHaveCount(2);
    await expect(manualSection).toContainText('فلتر زيت E2E');
    await expect(manualSection).toContainText('بواجي E2E');
    await expect(manualSection).toContainText('33.50');
    await expect(manualSection).toContainText('12.00');
  });
});
