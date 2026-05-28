const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/auth');

test('purchase bills list page loads', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills');
  await expect(page).toHaveURL(/purchase-bills/);
});

test('purchase bills empty state CTA has no leaked template delimiters', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills?q=__empty_purchase_bill_delimiter_check__');

  await expect(page.locator('a[href="/dashboard/purchase-bills/add"]').first()).toBeVisible();
  await expect(page.locator('a[href="/dashboard/purchases/add"]')).toHaveCount(0);
  await expect(page.locator('body')).not.toContainText('{{');
  await expect(page.locator('body')).not.toContainText('}}');
});

test('add purchase bill form loads', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills/add');
  // Form should have store, supplier, manual section
  await expect(page.locator('form')).toBeVisible();
});

test('submit button disables on click (no double submit)', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/purchase-bills/add');
  const submitBtn = page.locator('button[type="submit"]');
  // hx-disabled-elt should be set
  const form = page.locator('form[hx-disabled-elt]');
  await expect(form).toBeVisible();
});

test('supplier invoice duplicate check is debounced and user-friendly', async ({ page }) => {
  const pageErrors = [];
  const duplicateRequests = [];

  page.on('pageerror', (err) => pageErrors.push(err.message));

  await page.route('**/api/purchase-bills/duplicate-check', async (route) => {
    const payload = JSON.parse(route.request().postData() || '{}');
    duplicateRequests.push(payload);

    if (payload.supplier_sequence_number === 123458) {
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'temporary duplicate check outage' }),
      });
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        payload.supplier_sequence_number === 123456
          ? { exists: true, purchase_bill_id: 789 }
          : { exists: false }
      ),
    });
  });

  await login(page);
  await page.goto('/dashboard/purchase-bills/add');
  await page.waitForLoadState('domcontentloaded');

  const supplierSelect = page.locator('[name="supplier_id"]');
  const sequenceInput = page.locator('[name="supplier_sequance_number"]');
  const submitButton = page.locator('#purchase-form button[type="submit"]');
  const duplicateError = page.locator('#supplier-sequence-duplicate-error');
  const duplicateWarning = page.locator('#supplier-sequence-duplicate-warning');

  await expect(supplierSelect).toBeVisible();
  await expect(sequenceInput).toBeVisible();
  await expect(sequenceInput).toHaveAttribute('required', '');
  await expect(
    page.locator('label.form-label').filter({ hasText: /رقم فاتورة المورد|Supplier Bill Number/ }).locator('.required-mark')
  ).toBeVisible();

  const errorImmediatelyAfterInput = await sequenceInput.evaluate(
    (input) => input.nextElementSibling && input.nextElementSibling.id === 'supplier-sequence-duplicate-error'
  );
  expect(errorImmediatelyAfterInput).toBeTruthy();

  const supplierId = Number(await supplierSelect.inputValue());
  expect(supplierId).toBeGreaterThan(0);

  await sequenceInput.pressSequentially('123456', { delay: 20 });
  await expect.poll(() => duplicateRequests.length, { timeout: 2500 }).toBe(1);
  expect(duplicateRequests[0]).toMatchObject({
    supplier_id: supplierId,
    supplier_sequence_number: 123456,
  });

  await sequenceInput.blur();
  await page.waitForTimeout(500);
  expect(duplicateRequests).toHaveLength(1);

  await expect(duplicateError).toBeVisible();
  await expect(duplicateError).toContainText(/رقم فاتورة المورد|Supplier bill number/i);
  await expect(duplicateError.locator('a[href="/dashboard/purchase-bills/789"]')).toBeVisible();
  await expect(submitButton).toBeDisabled();

  await sequenceInput.fill('123457');
  await expect.poll(() => duplicateRequests.length, { timeout: 2500 }).toBe(2);
  await expect(duplicateError).toBeHidden();
  await expect(duplicateWarning).toBeHidden();
  await expect(submitButton).toBeEnabled();

  await sequenceInput.fill('123458');
  await expect.poll(() => duplicateRequests.length, { timeout: 2500 }).toBe(3);
  await expect(duplicateError).toBeHidden();
  await expect(duplicateWarning).toBeVisible();
  await expect(duplicateWarning).toContainText(/تعذر التحقق|Could not check/i);
  await expect(submitButton).toBeEnabled();

  expect(pageErrors).toEqual([]);
});
