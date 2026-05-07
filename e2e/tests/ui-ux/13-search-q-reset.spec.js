// Regression: list-page search must always RESET URL query params before
// applying the next form-driven update.  Bugs we have repeatedly shipped
// and want to lock down forever:
//
//   1. Typing q=a then q=b yields ?q=b (NOT q=a&q=b).      ← old bug
//   2. Removing a typed-filter chip drops its param from the URL.
//      Before this test, the htmx:configRequest helper only stripped
//      keys still present in the form, so a removed `phone=…` chip kept
//      the param glued to every subsequent submit forever.
//   3. Removing a <select> filter chip drops its param from the URL.
//   4. Direct-loading a URL with duplicate `q` and submitting the form
//      collapses it to a single `q` value.
//   5. Adding a typed-filter while q is non-empty replaces q (chip
//      promotion empties the q input) instead of leaving stale q in URL.
//
// All assertions go through the production smart-search flow — no test
// hooks. If any of these fire, the URL-reset helper has regressed.

const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');

test.describe.configure({ mode: 'parallel' });

// Read the URL params as a plain {key: [values…]} multimap so duplicate
// keys are visible to assertions.
async function paramsMulti(page) {
  return page.evaluate(() => {
    const out = {};
    for (const [k, v] of new URL(location.href).searchParams) {
      (out[k] = out[k] || []).push(v);
    }
    return out;
  });
}

test('q never duplicates across successive typed searches (clients)', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/clients');
  const q = page.locator('input[name="q"]');
  await q.fill('alpha');
  await page.waitForFunction(() => /[?&]q=alpha(&|$)/.test(location.search), { timeout: 5000 });
  await q.fill('');
  await q.fill('beta');
  await page.waitForFunction(() => /[?&]q=beta(&|$)/.test(location.search), { timeout: 5000 });
  const m = await paramsMulti(page);
  expect(m.q, 'q must be single-valued').toEqual(['beta']);
});

test('removing a typed-filter chip strips its param from the URL (clients)', async ({ page }) => {
  await login(page);
  // Direct-load with a typed filter param so we don't depend on the popover UI
  await page.goto('/dashboard/clients?phone=0501234567');
  // Wait for the typed chip to be rehydrated by smart-search.js
  const chip = page.locator('.smart-chip-typed');
  await expect(chip.first()).toBeVisible({ timeout: 5000 });

  // Click the chip to remove it. The X span is inside the button; clicking
  // anywhere on the chip fires the same handler.
  await chip.first().click();

  // After the htmx submit settles, the URL must NOT contain `phone`
  await page.waitForFunction(
    () => !/[?&]phone=/.test(location.search),
    { timeout: 5000 }
  );
  const m = await paramsMulti(page);
  expect(m.phone, 'phone param must be gone').toBeUndefined();
});

test('removing a typed-filter chip strips its param (suppliers vat_number)', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/suppliers?vat_number=300000000000003');
  const chip = page.locator('.smart-chip-typed');
  await expect(chip.first()).toBeVisible({ timeout: 5000 });
  await chip.first().click();
  await page.waitForFunction(
    () => !/[?&]vat_number=/.test(location.search),
    { timeout: 5000 }
  );
  const m = await paramsMulti(page);
  expect(m.vat_number).toBeUndefined();
});

test('clearing the q input removes q= from the URL (invoices)', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/invoices?q=foo');
  const q = page.locator('input[name="q"]');
  // Press Escape — production UX clears + resubmits and is keyboard-accessible.
  await q.focus();
  await q.press('Escape');
  await page.waitForFunction(
    () => !/[?&]q=foo(&|$)/.test(location.search),
    { timeout: 5000 }
  );
  const m = await paramsMulti(page);
  // q must be either absent or empty-string — never the stale 'foo'
  const qVals = m.q || [];
  expect(qVals.includes('foo'), 'stale q=foo must not survive').toBe(false);
});

test('duplicate-q deep-link collapses to single q after first form submit', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/products?q=a&q=b&q=c');
  const q = page.locator('input[name="q"]');
  await q.fill('z');
  await page.waitForFunction(() => /[?&]q=z(&|$)/.test(location.search), { timeout: 5000 });
  const m = await paramsMulti(page);
  expect(m.q, 'q collapsed to single value').toEqual(['z']);
});

test('typed param + q reset together: change q does not duplicate phone', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard/clients?phone=0501234567');
  await expect(page.locator('.smart-chip-typed').first()).toBeVisible({ timeout: 5000 });

  const q = page.locator('input[name="q"]');
  await q.fill('one');
  await page.waitForFunction(() => /[?&]q=one(&|$)/.test(location.search), { timeout: 5000 });
  await q.fill('');
  await q.fill('two');
  await page.waitForFunction(() => /[?&]q=two(&|$)/.test(location.search), { timeout: 5000 });

  const m = await paramsMulti(page);
  expect(m.q, 'q single-valued').toEqual(['two']);
  expect(m.phone, 'phone preserved exactly once').toEqual(['0501234567']);
});
