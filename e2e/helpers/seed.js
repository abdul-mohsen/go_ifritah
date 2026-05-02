// QA seeding helpers — find/use existing catalog products and create/delete bills.
// Creating products requires picking from the parts catalog, which makes
// scripted seeding brittle. We instead pick an EXISTING product and
// oversell it relative to its real stock.

const { expect } = require('@playwright/test');

// Find any existing product whose stock is a known number we can oversell.
// Returns { id, name, price, quantity } from the catalog search.
async function pickAnyProduct(page, queryHint = 'a') {
  if (!page.url().startsWith('http')) await page.goto('/dashboard');
  const product = await page.evaluate(async (q) => {
    const m = document.cookie.match(/csrf_token=([^;]+)/);
    const csrf = m ? m[1] : '';
    const fd = new FormData();
    fd.append('query', q);
    const r = await fetch('/api/products/search-json', {
      method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: fd, credentials: 'same-origin',
    });
    if (!r.ok) return null;
    const arr = await r.json().catch(() => []);
    if (!Array.isArray(arr) || arr.length === 0) return null;
    // Prefer one with a finite numeric quantity so oversell math is sound.
    return arr.find((p) => p && Number.isFinite(Number(p.quantity))) || arr[0];
  }, queryHint);
  if (!product) throw new Error(`pickAnyProduct: no product matched query "${queryHint}"`);
  return product;
}

// Build and submit a real bill via the /dashboard/invoices/add-invoice form.
// Returns diagnostics for the test to assert on.
async function attemptCreateBill(page, { productId, productName, productPrice, qty, userName, userPhone, expectDialog }) {
  const consoleMessages = [];
  const networkPosts = [];
  page.on('console', (msg) => consoleMessages.push(`[${msg.type()}] ${msg.text()}`));
  page.on('pageerror', (e) => consoleMessages.push(`[pageerror] ${e.message}`));
  page.on('request', (req) => {
    if (req.method() === 'POST' || req.method() === 'PUT' || req.method() === 'DELETE') {
      networkPosts.push(`${req.method()} ${req.url()}`);
    }
  });
  page.on('response', async (res) => {
    const u = res.url();
    if (u.includes('/api/')) {
      let body = '';
      try { body = (await res.text()).slice(0, 200); } catch (e) { }
      networkPosts.push(`<<< ${res.status()} ${u} :: ${body}`);
    }
  });

  await page.goto('/dashboard/invoices/add-invoice');
  await page.waitForLoadState('domcontentloaded');

  // Fill personal-mode customer fields.
  const nameField = page.locator('input[name="user_name"]').first();
  if (await nameField.count() > 0) await nameField.fill(userName);
  const phoneField = page.locator('input[name="user_phone_number"]').first();
  if (await phoneField.count() > 0) await phoneField.fill(userPhone);

  // Add a catalog item row — the page starts empty.
  await page.evaluate(() => {
    if (typeof addCatalogItem === 'function') addCatalogItem();
  });
  await page.waitForSelector('input[name="products_product_id"]', { state: 'attached', timeout: 5000 });

  // Fill the visible product-search box so the row doesn't look empty (UX).
  const searchBox = page.locator('input.product-search').first();
  if (await searchBox.count() > 0) await searchBox.fill(productName);

  // Populate hidden + visible row fields directly. This is what selectProduct()
  // does after picking from the dropdown.
  await page.evaluate(({ pid, name, price, q }) => {
    const idEl = document.querySelector('input[name="products_product_id"]');
    const nmEl = document.querySelector('input[name="products_name"]');
    const prEl = document.querySelector('input[name="products_price"]');
    const qtEl = document.querySelector('input[name="products_quantity"]');
    if (idEl) idEl.value = String(pid);
    if (nmEl) nmEl.value = name;
    if (prEl) prEl.value = String(price);
    if (qtEl) qtEl.value = String(q);
    if (typeof recalculateTotal === 'function') recalculateTotal();
  }, { pid: productId, name: productName, price: productPrice, q: qty });

  // Snapshot field values at submit time.
  const snapshot = await page.evaluate(() => ({
    productId: document.querySelector('input[name="products_product_id"]')?.value,
    productName: document.querySelector('input[name="products_name"]')?.value,
    productPrice: document.querySelector('input[name="products_price"]')?.value,
    productQty: document.querySelector('input[name="products_quantity"]')?.value,
    storeId: document.querySelector('select[name="store_id"]')?.value,
    branchId: document.querySelector('select[name="branch_id"]')?.value,
    userName: document.querySelector('input[name="user_name"]')?.value,
    userPhone: document.querySelector('input[name="user_phone_number"]')?.value,
  }));

  // Wire up dialog interception. Each dialog is captured into `dialogs`.
  const dialogs = [];
  page.on('dialog', async (d) => {
    dialogs.push({ type: d.type(), message: d.message() });
    if (expectDialog === 'accept') await d.accept();
    else if (expectDialog === 'dismiss') await d.dismiss();
    else await d.dismiss();
  });

  // Click the "Save & Issue" submit (state=1).
  await page.locator('button[name="state"][value="1"]').click();
  await page.waitForTimeout(3000);

  return { dialogs, finalUrl: page.url(), snapshot, consoleMessages, networkPosts };
}

// Find the most recent bill on /dashboard/invoices whose row text contains `needle`.
async function findBillIdByText(page, needle) {
  await page.goto('/dashboard/invoices?per_page=20');
  await page.waitForLoadState('domcontentloaded');
  const id = await page.evaluate((q) => {
    const rows = document.querySelectorAll('tr, .row, [class*="row"]');
    for (const r of rows) {
      if (r.innerText && r.innerText.includes(q)) {
        const a = r.querySelector('a[href*="/bill/"], a[href*="/dashboard/invoices/edit/"]');
        const href = a ? a.getAttribute('href') : '';
        const m = href ? href.match(/\/(?:bill|edit)\/(\d+)/) : null;
        if (m) return m[1];
      }
    }
    return null;
  }, needle);
  return id;
}

// Best-effort delete of a bill. Ignores failures so cleanup never poisons a run.
async function deleteBill(page, id) {
  if (!id) return;
  await page.evaluate(async (bid) => {
    const m = document.cookie.match(/csrf_token=([^;]+)/);
    const csrf = m ? m[1] : '';
    await fetch(`/api/invoices/${bid}`, {
      method: 'DELETE', credentials: 'same-origin',
      headers: { 'X-CSRF-Token': csrf },
    }).catch(() => { });
  }, id);
}

module.exports = {
  pickAnyProduct,
  attemptCreateBill, findBillIdByText, deleteBill,
};
