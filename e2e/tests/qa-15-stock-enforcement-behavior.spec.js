// QA-15: Stock enforcement BEHAVIOR (frontend only).
//
// Validates the three modes (`disable` / `warn` / `enforce`) actually
// behave differently when the user attempts to oversell a real product.
// Per the codebase, the enforcement is implemented in client-side JS in
// templates/add-invoice.html that:
//   - mode=disable: no dialog, form submits (backend may still reject).
//   - mode=warn:    confirm() dialog with "تحذير" text. Dismiss → blocked.
//   - mode=enforce: alert() dialog with "لا يمكن" text. Submission blocked.
//
// We assert ONLY the frontend behavior we control here. Whether the
// downstream backend then accepts or rejects an oversell is a separate
// concern.

const { test, expect } = require('@playwright/test');
const { login, setSetting, uniqueTag } = require('../helpers/qa');
const { pickAnyProduct, attemptCreateBill, findBillIdByText, deleteBill } = require('../helpers/seed');

// This suite has a known intermittent-flake source: dev.ifritah.com's
// settings read-after-write can occasionally lag by a beat even after
// setSetting()'s own persistence-verification retries succeed (a separate
// caching layer between the confirm-mode-write and the very next
// add-invoice page render), which surfaces here as the enforcement dialog
// (confirm/alert) not firing on the first attempt. Retry the
// set-mode-then-attempt-oversell sequence a few times before failing, same
// spirit as setSetting()'s own built-in retry loop - this does not weaken
// any assertion, it just tolerates the same documented lag setSetting
// already retries around.
async function attemptOversellExpectingDialog(page, { mode, product, oversellQty, userTag, expectDialog }) {
  let result;
  for (let attempt = 1; attempt <= 3; attempt++) {
    await setSetting(page, 'stock_enforcement', mode);
    result = await attemptCreateBill(page, {
      productId: product.id, productName: product.name, productPrice: product.price,
      qty: oversellQty, userName: userTag, userPhone: '0500000000',
      expectDialog,
    });
    if (result.dialogs.length > 0) return result;
  }
  return result;
}

test.describe('Stock enforcement behavior (frontend)', () => {
  let product; // { id, name, price, quantity }
  let oversellQty;
  const billsToCleanup = [];

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await login(page);
    try {
      product = await pickAnyProduct(page, 'a');
      oversellQty = (Number(product.quantity) || 0) + 9999;
    } catch (e) {
      product = null;
    }
    await ctx.close();
  });

  test.afterAll(async ({ browser }) => {
    if (billsToCleanup.length === 0) return;
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await login(page);
    for (const id of billsToCleanup) await deleteBill(page, id);
    await ctx.close();
  });

  test('mode=disable: oversell shows NO dialog', async ({ page }) => {
    test.skip(!product, 'no products available on dev backend');
    await login(page);
    await setSetting(page, 'stock_enforcement', 'disable');

    const userTag = uniqueTag('QA-CUST');
    const result = await attemptCreateBill(page, {
      productId: product.id, productName: product.name, productPrice: product.price,
      qty: oversellQty, userName: userTag, userPhone: '0500000000',
      expectDialog: 'accept',
    });

    expect(result.dialogs, `disable mode must NOT pop any dialog. Got: ${JSON.stringify(result.dialogs)}`).toEqual([]);

    const billId = await findBillIdByText(page, userTag);
    if (billId) billsToCleanup.push(billId);
  });

  test('mode=warn + dismiss: oversell pops confirm and is BLOCKED', async ({ page }) => {
    test.skip(!product, 'no products available on dev backend');
    await login(page);

    const userTag = uniqueTag('QA-CUST');
    const result = await attemptOversellExpectingDialog(page, {
      mode: 'warn', product, oversellQty, userTag, expectDialog: 'dismiss',
    });

    expect(result.dialogs.length, `warn mode must show a confirm dialog. Got: ${JSON.stringify(result.dialogs)}`).toBeGreaterThan(0);
    expect(result.dialogs[0].type, 'first dialog must be confirm()').toBe('confirm');
    const messages = result.dialogs.map((d) => d.message).join('|');
    expect(messages, 'warn dialog must mention warning/stock').toMatch(/تحذير|warn|⚠|stock|مخزون/i);

    expect(result.finalUrl, 'warn+dismiss: must stay on add page').toMatch(/add-invoice/);
    const billId = await findBillIdByText(page, userTag);
    expect(billId, `warn+dismiss must NOT create a bill, found id=${billId}`).toBeFalsy();
  });

  test('mode=warn + accept: oversell pops confirm (frontend lets through)', async ({ page }) => {
    test.skip(!product, 'no products available on dev backend');
    await login(page);

    const userTag = uniqueTag('QA-CUST');
    const result = await attemptOversellExpectingDialog(page, {
      mode: 'warn', product, oversellQty, userTag, expectDialog: 'accept',
    });

    expect(result.dialogs.length, 'warn mode must show a confirm dialog').toBeGreaterThan(0);
    expect(result.dialogs[0].type, 'first dialog must be confirm()').toBe('confirm');

    const billId = await findBillIdByText(page, userTag);
    if (billId) billsToCleanup.push(billId);
  });

  test('mode=enforce: oversell pops alert and BLOCKS submission', async ({ page }) => {
    test.skip(!product, 'no products available on dev backend');
    await login(page);

    const userTag = uniqueTag('QA-CUST');
    const result = await attemptOversellExpectingDialog(page, {
      mode: 'enforce', product, oversellQty, userTag, expectDialog: 'accept',
    });

    expect(result.dialogs.length, `enforce mode must show an alert. Got: ${JSON.stringify(result.dialogs)}`).toBeGreaterThan(0);
    expect(result.dialogs[0].type, 'first dialog must be alert()').toBe('alert');
    const messages = result.dialogs.map((d) => d.message).join('|');
    expect(messages, 'enforce alert must mention block/insufficient').toMatch(/لا يمكن|إتمام|⛔|stock|مخزون/i);

    expect(result.finalUrl, 'enforce: must NOT redirect (submission blocked)').toMatch(/add-invoice/);
    const billId = await findBillIdByText(page, userTag);
    expect(billId, `enforce must NOT create a bill, found id=${billId}`).toBeFalsy();
  });
});
