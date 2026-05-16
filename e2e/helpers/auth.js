// Auth helper. With Playwright's storageState (configured in
// playwright.config.js), tests already start authenticated. login() is
// idempotent — if /dashboard is reachable it returns immediately,
// otherwise it fails fast instead of hammering /login.
const path = require('path');
const fs = require('fs');

const BASE = process.env.PW_BASE_URL || 'http://127.0.0.1:8000';
const USER = process.env.PW_USER || 'admin';
const PASS = process.env.PW_PASS || 'admin123';
const STORAGE = path.join(__dirname, '..', '.auth', 'storageState.json');

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

async function login(page) {
  try {
    await applyStoredAuth(page);
    await page.goto(`${BASE}/dashboard`, { waitUntil: 'domcontentloaded', timeout: 20000 });
    if (/\/dashboard/.test(page.url())) return;
  } catch (e) {
    throw new Error(`stored auth could not open dashboard at ${BASE}: ${e.message}`);
  }

  throw new Error(`stored auth was rejected at ${BASE}; regenerate .auth/storageState.json`);
}

module.exports = { login, BASE, USER, PASS };