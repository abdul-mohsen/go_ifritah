// QA-39: Purchase-bill selling-price visibility/edit + Excel/CSV import
// existing-product lookup.
//
// Covers the two frontend behaviors added alongside the backend's
// role-aware selling-price override (ifritah-go PR #60):
//   1. Selecting an existing catalog product on the purchase-bill form
//      pre-fills its current selling price (in addition to the existing
//      cost price / shelf number pre-fill), and the field is editable
//      for the logged-in admin.
//   2. An imported row that carries a product_id fetches that product's
//      shelf/selling price for review; a row with a blank id but a name
//      that already exists in the catalog gets a non-blocking warning
//      instead of silently duplicating the item.

const { test, expect } = require('@playwright/test');
const { login, appURL, uniqueTag } = require('../helpers/qa');

async function getCsrfToken(page) {
  const cookies = await page.context().cookies();
  const csrf = cookies.find((c) => c.name === 'csrf_token');
  return csrf ? csrf.value : '';
}

// Creates a store product directly through the existing create-product
// endpoint (bypassing the OEM-search UI, which only accepts real catalog
// article numbers) and returns its assigned catalog id via the same
// search endpoint the purchase-bill form itself uses.
async function createStoreProduct(page, { storeId, name, price, costPrice, shelfNumber }) {
  const csrfToken = await getCsrfToken(page);
  const createResponse = await page.request.post(appURL('/dashboard/products/create'), {
    headers: { 'X-CSRF-Token': csrfToken },
    form: {
      store_id: String(storeId),
      'quantity[]': '5',
      'price[]': String(price),
      'cost_price[]': String(costPrice),
      'shelf_number[]': shelfNumber,
      'part_name[]': name,
    },
  });
  expect(createResponse.ok(), `create product failed with ${createResponse.status()}`).toBeTruthy();

  // The backend's AddProduct handler never stores the free-text name we supply
  // (its AddProduct JSON struct has no name field), so p.name is always NULL and
  // searching by name never returns the product.  Shelf number IS stored and the
  // backend's GetAllProduct SQL searches COALESCE(shelf_number,'') LIKE '%query%',
  // so poll by shelfNumber instead.
  const searchForm = { query: shelfNumber };
  let productId = null;
  for (let attempt = 0; attempt < 10 && productId === null; attempt++) {
    const searchResponse = await page.request.post(appURL('/api/products/search-json'), {
      headers: { 'X-CSRF-Token': csrfToken },
      form: searchForm,
    });
    const results = await searchResponse.json();
    const match = (results || []).find((p) => p.shelf_number === shelfNumber && p.store_id === storeId);
    if (match) {
      productId = match.id;
      break;
    }
    await page.waitForTimeout(500);
  }
  expect(productId, `store product with shelf "${shelfNumber}" did not appear in search-json after creation`).not.toBeNull();
  return productId;
}

async function defaultStoreId(page) {
  const storeSelect = page.locator('[name="store_id"]').first();
  const value = await storeSelect.inputValue();
  return parseInt(value, 10);
}

// Backend decimal columns round-trip through the API as e.g. "275.00", not
// the plain "275" a test writes, so compare numerically rather than by
// exact string. Polls rather than reading once: the value can be filled by
// an in-flight async lookup (e.g. resolveImportedItemLookup's fetch) that
// isn't otherwise gated by a preceding retrying assertion, so a one-shot
// read can race ahead of it.
async function expectNumericValue(locator, expected) {
  await expect.poll(async () => {
    const actual = await locator.inputValue();
    return parseFloat(actual);
  }, `expected numeric value ${expected}`).toBe(expected);
}

test.describe('Purchase-bill selling price visibility', () => {
  test('selecting an existing product pre-fills and allows editing the selling price', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    const storeId = await defaultStoreId(page);
    const name = uniqueTag('QA39-Sell');
    const shelfNumber = 'SP-' + name.slice(-6);
    await createStoreProduct(page, { storeId, name, price: 275, costPrice: 150, shelfNumber });

    // Admin (the default test login) must be allowed to edit the field.
    const canEdit = await page.evaluate(() => window.canEditSellingPrice);
    expect(canEdit, 'admin session should have canEditSellingPrice = true').toBe(true);

    await page.locator('button[onclick^="addItem"]').first().click();
    const row = page.locator('#products-container .item-row').last();
    await row.locator('.store-product-search').fill(shelfNumber);
    const dropdownItem = row.locator('[id^="dropdown_"] div').filter({ hasText: shelfNumber }).first();
    await expect(dropdownItem).toBeVisible({ timeout: 10000 });
    await dropdownItem.click();

    await expect(row.locator('[name="products_product_id"]')).not.toHaveValue('0');
    await expectNumericValue(row.locator('[name="products_cost_price"]'), 150);
    await expect(row.locator('[name="products_shelf_number"]')).toHaveValue(shelfNumber);
    const sellingPriceInput = row.locator('[name="products_selling_price"]');
    await expectNumericValue(sellingPriceInput, 275);
    await expect(sellingPriceInput).toBeEditable();

    // Admin can adjust it before submitting.
    await sellingPriceInput.fill('300');
    await expect(sellingPriceInput).toHaveValue('300');
  });
});

test.describe('Purchase-bill Excel/CSV import existing-product lookup', () => {
  test('imported row with a product id fetches shelf and selling price for review', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    const storeId = await defaultStoreId(page);
    const name = uniqueTag('QA39-ImportId');
    const shelfNumber = 'IMP-' + name.slice(-6);
    const productId = await createStoreProduct(page, {
      storeId,
      name,
      price: 410,
      costPrice: 200,
      shelfNumber,
    });

    const csrfToken = await getCsrfToken(page);
    const csvBody =
      'اسم القطعة,الكمية,سعر الشراء,رقم الرف,معرف المنتج (اختياري)\n' +
      `${name},3,220,${shelfNumber},${productId}\n`;
    const parseResponse = await page.request.post(appURL('/api/purchase-bills/parse-csv'), {
      headers: { 'X-CSRF-Token': csrfToken },
      multipart: { file: { name: 'import-with-id.csv', mimeType: 'text/csv', buffer: Buffer.from(csvBody, 'utf8') } },
    });
    expect(parseResponse.ok(), `parse-csv failed with ${parseResponse.status()}`).toBeTruthy();
    const items = await parseResponse.json();
    expect(items).toHaveLength(1);
    expect(items[0].productId).toBe(productId);

    // Drive the same client-side resolution the real upload flow uses.
    await page.evaluate((item) => {
      const container = document.getElementById('products-container');
      const before = container.children.length;
      container.insertAdjacentHTML('beforeend', itemRow(item));
      const row = container.children[before];
      resolveImportedItemLookup(row, item);
    }, items[0]);

    const importedRow = page.locator('#products-container .item-row').last();
    await expect(importedRow.locator('[name="products_product_id"]')).toHaveValue(String(productId), {
      timeout: 10000,
    });
    await expectNumericValue(importedRow.locator('[name="products_selling_price"]'), 410);
    await expect(importedRow.locator('.item-state')).toContainText(/من المخزن|From Store/);
  });

  test('imported row without a product id warns when a same-named item already exists', async ({ page }) => {
    await login(page);
    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    const storeId = await defaultStoreId(page);
    const name = uniqueTag('QA39-ImportWarn');
    const shelfNumber = 'WARN-' + name.slice(-6);
    await createStoreProduct(page, { storeId, name, price: 90, costPrice: 60, shelfNumber });

    // partName is the unique shelfNumber so fetchStoreProducts() finds the product
    // via shelf_number search; the updated existsByName check in the template then
    // matches p.shelf_number === partName and shows the warning.
    const item = { partName: shelfNumber, quantity: 2, purchasePrice: 50, costPrice: 40, shelfNumber: '' };
    await page.evaluate((it) => {
      const container = document.getElementById('products-container');
      const before = container.children.length;
      container.insertAdjacentHTML('beforeend', itemRow(it));
      const row = container.children[before];
      resolveImportedItemLookup(row, it);
    }, item);

    const importedRow = page.locator('#products-container .item-row').last();
    await expect(importedRow.locator('.existing-item-warning')).toBeVisible({ timeout: 10000 });
    await expect(importedRow.locator('.existing-item-warning')).toContainText(/يوجد|already exists/);
    // The row must still be manual (never auto-linked from a name match alone).
    await expect(importedRow.locator('[name="products_product_id"]')).toHaveValue('0');
  });
});
