const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/auth');

test('settings page loads with ZATCA tab', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await expect(page.locator('#tab-zatca')).toBeVisible();
});

test('ZATCA tab shows branch selector and accordion sections', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-branch-select')).toBeVisible();
  // Accordion sections should be visible
  await expect(page.locator('#zatca-acc-csr')).toBeVisible();
  await expect(page.locator('#zatca-acc-seller')).toBeVisible();
  await expect(page.locator('#zatca-acc-address')).toBeVisible();
});

test('ZATCA stepper progress bar shows 0% initially', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-progress-pct')).toContainText('0%');
});

test('ZATCA connect button is disabled when fields are empty', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-connect-btn')).toBeDisabled();
});

test('ZATCA accordion sections collapse and expand', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  // Collapse CSR section
  await page.click('#zatca-acc-csr-btn');
  await expect(page.locator('#zatca-acc-csr')).toBeHidden();
  // Expand CSR section
  await page.click('#zatca-acc-csr-btn');
  await expect(page.locator('#zatca-acc-csr')).toBeVisible();
});

test('ZATCA VAT validation shows error for invalid format', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await page.fill('#zatca_seller_vat', '123');
  await page.locator('#zatca_seller_vat').blur();
  await expect(page.locator('#zatca_seller_vat_error')).toBeVisible();
});

test('ZATCA country field defaults to SA', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  const countryVal = await page.locator('#zatca_csr_country').inputValue();
  expect(countryVal).toBe('SA');
});

test('ZATCA business category is a dropdown with options', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  const options = page.locator('#zatca_csr_business_category option');
  const count = await options.count();
  expect(count).toBeGreaterThan(5);
});

test('ZATCA status badge is visible', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-status-badge')).toBeVisible();
});

test('ZATCA save button exists', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/settings');
  await page.click('#tab-zatca');
  await expect(page.locator('#zatca-save-btn')).toBeVisible();
});
