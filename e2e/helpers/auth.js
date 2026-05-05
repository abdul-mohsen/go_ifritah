// Auth helper. With Playwright's storageState (configured in
// playwright.config.js), tests already start authenticated. login() is
// idempotent — if /dashboard is reachable it returns immediately,
// otherwise it falls back to a form-based login.
const BASE = process.env.PW_BASE_URL || 'http://127.0.0.1:8000';
const USER = process.env.PW_USER || 'admin';
const PASS = process.env.PW_PASS || 'admin123';

async function login(page) {
  // Fast path: storageState already authenticated us.
  try {
    await page.goto(`${BASE}/dashboard`, { waitUntil: 'domcontentloaded', timeout: 8000 });
    if (/\/dashboard/.test(page.url())) return;
  } catch (e) { /* fall through */ }

  await page.goto(`${BASE}/login`);
  await page.fill('input[name="username"]', USER);
  await page.fill('input[name="password"]', PASS);
  await Promise.all([
    page.waitForURL(/\/dashboard/, { timeout: 15000 }).catch(() => {}),
    page.click('button[type="submit"]'),
  ]);
}

module.exports = { login, BASE, USER, PASS };