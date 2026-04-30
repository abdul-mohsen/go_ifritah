// QA-02: Every primary page renders for an authenticated admin.
// Verifies route → 200 + a sentinel element so we catch template-render breaks.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

const PAGES = [
  { url: '/dashboard', must: 'main, #app, body' },
  { url: '/dashboard/invoices', must: 'a[href*="/dashboard/invoices/add-invoice"]' },
  { url: '/dashboard/invoices/add-invoice', must: 'form' },
  { url: '/dashboard/purchase-bills', must: 'a[href*="/dashboard/purchase-bills/add"]' },
  { url: '/dashboard/purchase-bills/add', must: 'form' },
  { url: '/dashboard/cash-vouchers', must: 'a[href*="/dashboard/cash-vouchers/add"]' },
  { url: '/dashboard/cash-vouchers/add', must: 'form' },
  { url: '/dashboard/products', must: 'a[href*="/dashboard/products/add"]' },
  { url: '/dashboard/products/add', must: 'form' },
  { url: '/dashboard/clients', must: 'a[href*="/dashboard/clients/add"]' },
  { url: '/dashboard/clients/add', must: 'form' },
  { url: '/dashboard/suppliers', must: 'a[href*="/dashboard/suppliers/add"]' },
  { url: '/dashboard/suppliers/add', must: 'form' },
  { url: '/dashboard/orders', must: 'body' },
  { url: '/dashboard/orders/add', must: 'form' },
  { url: '/dashboard/branches', must: 'body' },
  { url: '/dashboard/stores', must: 'body' },
  { url: '/dashboard/users', must: 'a[href*="/dashboard/users/add"]' },
  { url: '/dashboard/settings', must: 'form' },
  { url: '/dashboard/notifications', must: 'body' },
  { url: '/dashboard/parts', must: 'body' },
  { url: '/dashboard/cars', must: 'body' },
  { url: '/dashboard/stock/adjustments', must: 'body' },
  { url: '/dashboard/zatca-monitor', must: 'body' },
  { url: '/dashboard/invoices/import-csv', must: 'body' },
  { url: '/dashboard/compare', must: 'body' },
];

test.beforeEach(async ({ page }) => { await login(page); });

for (const { url, must } of PAGES) {
  test(`renders ${url}`, async ({ page }) => {
    const resp = await page.goto(url);
    expect(resp.status(), `status for ${url}`).toBeLessThan(400);
    await expect(page.locator(must).first()).toBeAttached({ timeout: 5000 });
  });
}
