// QA suite shared helpers — login, settings, seeding, dashboard reads.
const { expect } = require('@playwright/test');
const path = require('path');
const fs = require('fs');

const BASE = process.env.PW_BASE_URL || 'http://127.0.0.1:8000';
const USER = process.env.PW_USER || 'admin';
const PASS = process.env.PW_PASS || 'admin';
const STORAGE = path.join(__dirname, '..', '.auth', 'storageState.json');

function appURL(route) {
  return new URL(route, BASE).toString();
}

async function applyStoredAuth(page) {
  if (!fs.existsSync(STORAGE)) return;

  let state;
  try {
    state = JSON.parse(fs.readFileSync(STORAGE, 'utf8'));
  } catch (e) {
    return;
  }

  if (Array.isArray(state.cookies) && state.cookies.length) {
    await page.context().addCookies(state.cookies);
  }
}

async function login(page, user = USER, pass = PASS) {
  try {
    await applyStoredAuth(page);
    await page.goto(appURL('/dashboard'), { waitUntil: 'domcontentloaded', timeout: 20000 });
    if (/\/dashboard/.test(new URL(page.url()).pathname)) return;
  } catch (e) {
    throw new Error(`stored auth could not open dashboard at ${BASE}: ${e.message}`);
  }

  throw new Error(`stored auth was rejected at ${BASE}; regenerate .auth/storageState.json for ${user}`);
}

// Set a single settings field (checkbox/select/number) by submitting the
// settings form. Reads existing values first to preserve other fields.
async function setSetting(page, key, value, options = {}) {
  const { verifyPersisted = true } = options;
  const settingsUrl = '/dashboard/settings';
  await page.goto(appURL(settingsUrl), { waitUntil: 'domcontentloaded', timeout: 30000 });

  // Find the field
  const field = page.locator(`[name="${key}"]`).first();
  await field.waitFor({ state: 'attached', timeout: 10000 });

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

  // Submit the form containing this field. Use programmatic .submit() to
  // bypass any overlay/disabled-button quirks (the settings page has a
  // single huge form with the submit button far below the viewport, and
  // some toggles attach click-time JS that can intercept .click()).
  const form = field.locator('xpath=ancestor::form[1]');
  const saveResponsePromise = page.waitForResponse((response) => {
    return response.request().method() === 'POST' && response.url().includes(settingsUrl);
  }, { timeout: 15000 }).catch(() => null);
  const navigationPromise = page.waitForURL((url) => url.pathname === settingsUrl, {
    waitUntil: 'domcontentloaded',
    timeout: 15000,
  }).catch(() => null);
  await form.evaluate((f) => {
    if (typeof f.requestSubmit === 'function') {
      f.requestSubmit();
      return;
    }
    f.submit();
  });
  await Promise.all([saveResponsePromise, navigationPromise]);
  // Allow flash to render
  await page.waitForTimeout(300);

  if (!verifyPersisted) return;

  // The dev backend's settings GET is occasionally read-after-write stale
  // (caching layer between Go FE and the DB). Re-fetch with a cache-bust
  // and retry until the new value is visible (or we exhaust retries).
  for (let i = 0; i < 16; i++) {
    await page.goto(appURL(`${settingsUrl}?_=${Date.now()}`), { waitUntil: 'domcontentloaded', timeout: 30000 });
    const fresh = page.locator(`[name="${key}"]`).first();
    let actual;
    if (type === 'checkbox') {
      actual = (await fresh.isChecked()) ? 'true' : 'false';
    } else {
      actual = await fresh.inputValue();
    }
    const want = type === 'checkbox' ? (Boolean(value) ? 'true' : 'false') : String(value);
    if (actual === want) return;
    await page.waitForTimeout(1000);
  }

  throw new Error(`setting ${key} did not persist as ${value}`);
}

// Read a numeric KPI from the dashboard by visible label fragment.
async function readDashboardNumber(page, labelFragment) {
  await page.goto(appURL('/dashboard'));
  await page.waitForLoadState('domcontentloaded');
  const card = page.locator(`xpath=//*[contains(normalize-space(.), "${labelFragment}")]/ancestor::*[self::div or self::a][1]`).first();
  const text = await card.innerText().catch(() => '');
  const m = text.match(/[-+]?[0-9][0-9,\.]*/);
  return m ? parseFloat(m[0].replace(/,/g, '')) : null;
}

function uniqueTag(prefix = 'QA') {
  const ts = new Date().toISOString().replaceAll(/\D/g, '').slice(0, 14);
  // NOSONAR S2245: non-cryptographic suffix used only to disambiguate test fixture names.
  const suffix = Math.floor(Math.random() * 1000); // NOSONAR
  return `${prefix}-${ts}-${suffix}`;
}

module.exports = { BASE, appURL, login, setSetting, readDashboardNumber, uniqueTag, expect };
