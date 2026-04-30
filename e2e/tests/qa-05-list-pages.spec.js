// QA-05: List pages — pagination, search, status filter for each entity list.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

test.beforeEach(async ({ page }) => { await login(page); });

const LISTS = [
  { url: '/dashboard/invoices', search: 'q', state: 'state' },
  { url: '/dashboard/purchase-bills', search: 'q' },
  { url: '/dashboard/cash-vouchers' },
  { url: '/dashboard/products', search: 'q' },
  { url: '/dashboard/clients', search: 'q' },
  { url: '/dashboard/suppliers', search: 'q' },
  { url: '/dashboard/orders' },
  { url: '/dashboard/users' },
];

for (const cfg of LISTS) {
  test(`${cfg.url} renders`, async ({ page }) => {
    const resp = await page.goto(cfg.url);
    expect(resp.status()).toBeLessThan(400);
    // Page should contain some kind of list/table region.
    const body = await page.content();
    expect(body.length).toBeGreaterThan(2000);
  });

  if (cfg.search) {
    test(`${cfg.url} accepts search query`, async ({ page }) => {
      const resp = await page.goto(`${cfg.url}?${cfg.search}=zzznotfound999`);
      expect(resp.status()).toBeLessThan(400);
    });
  }

  if (cfg.state) {
    test(`${cfg.url} accepts state filter`, async ({ page }) => {
      const resp = await page.goto(`${cfg.url}?${cfg.state}=0`);
      expect(resp.status()).toBeLessThan(400);
    });
  }

  test(`${cfg.url} accepts pagination`, async ({ page }) => {
    const resp = await page.goto(`${cfg.url}?page=1&per_page=5`);
    expect(resp.status()).toBeLessThan(400);
  });
}
