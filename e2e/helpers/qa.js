// QA suite shared helpers — login, settings, seeding, dashboard reads.
const { expect } = require('@playwright/test');

const BASE = 'http://localhost:8001';

async function login(page, user = 'admin', pass = 'admin') {
  await page.goto('/login');
  await page.fill('input[name="username"]', user);
  await page.fill('input[name="password"]', pass);
  await page.click('button[type="submit"]');
  await page.waitForURL('**/dashboard**', { timeout: 15000 });
}

// Set a single settings field (checkbox/select/number) by submitting the
// settings form. Reads existing values first to preserve other fields.
async function setSetting(page, key, value) {
  await page.goto('/dashboard/settings');
  await page.waitForLoadState('domcontentloaded');

  // Find the field
  const field = page.locator(`[name="${key}"]`).first();
  await field.waitFor({ state: 'attached', timeout: 5000 });

  const tag = await field.evaluate((el) => el.tagName.toLowerCase());
  const type = await field.evaluate((el) => el.type || '');

  if (type === 'checkbox') {
    const checked = await field.isChecked();
    if (Boolean(value) && !checked) await field.check();
    if (!Boolean(value) && checked) await field.uncheck();
  } else if (tag === 'select') {
    await field.selectOption(String(value));
  } else {
    await field.fill(String(value));
  }

  // Submit the form containing this field
  const form = field.locator('xpath=ancestor::form[1]');
  await Promise.all([
    page.waitForURL('**/dashboard/settings**', { timeout: 10000 }),
    form.locator('button[type="submit"]').first().click(),
  ]);
  // Allow flash to render
  await page.waitForTimeout(300);
}

// Read a numeric KPI from the dashboard by visible label fragment.
async function readDashboardNumber(page, labelFragment) {
  await page.goto('/dashboard');
  await page.waitForLoadState('domcontentloaded');
  const card = page.locator(`xpath=//*[contains(normalize-space(.), "${labelFragment}")]/ancestor::*[self::div or self::a][1]`).first();
  const text = await card.innerText().catch(() => '');
  const m = text.match(/[-+]?[0-9][0-9,\.]*/);
  return m ? parseFloat(m[0].replace(/,/g, '')) : null;
}

function uniqueTag(prefix = 'QA') {
  const ts = new Date().toISOString().replace(/[^0-9]/g, '').slice(0, 14);
  return `${prefix}-${ts}-${Math.floor(Math.random() * 1000)}`;
}

module.exports = { BASE, login, setSetting, readDashboardNumber, uniqueTag, expect };
