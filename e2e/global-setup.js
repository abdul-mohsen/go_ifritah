// Global setup: log in once via the API and persist storage state so
// the parallel workers don't all hammer /login simultaneously. If login
// fails (backend unreachable, credentials don't match this environment,
// etc.) we still write a *valid empty* storageState file so Playwright's
// `use.storageState` doesn't blow up — tests that need auth then fall
// back to the per-test `login()` helper.
const { request } = require('@playwright/test');
const path = require('path');
const fs = require('fs');

const BASE = process.env.PW_BASE_URL || 'http://127.0.0.1:8000';
const USER = process.env.PW_USER || 'admin';
const PASS = process.env.PW_PASS || 'admin123';
const STORAGE = path.join(__dirname, '.auth', 'storageState.json');

async function tryLogin() {
  const ctx = await request.newContext({ baseURL: BASE, ignoreHTTPSErrors: true });
  try {
    for (let i = 0; i < 5; i++) {
      try {
        const r = await ctx.post('/login', {
          form: { username: USER, password: PASS },
          maxRedirects: 0,
        });
        if (r.status() < 400) {
          await ctx.storageState({ path: STORAGE });
          return true;
        }
        console.warn(`[global-setup] login attempt ${i + 1}: status ${r.status()}`);
      } catch (e) {
        console.warn(`[global-setup] login attempt ${i + 1}: ${e.message}`);
      }
      await new Promise(r => setTimeout(r, 800));
    }
    return false;
  } finally {
    await ctx.dispose();
  }
}

module.exports = async () => {
  fs.mkdirSync(path.dirname(STORAGE), { recursive: true });
  const ok = await tryLogin();
  if (ok) {
    console.log(`[global-setup] storage state saved -> ${STORAGE}`);
    return;
  }
  // Write a valid empty storageState so Playwright's use.storageState
  // can still load. Tests will fall back to the form-based login helper.
  fs.writeFileSync(STORAGE, JSON.stringify({ cookies: [], origins: [] }));
  console.warn(`[global-setup] WARN: login failed, wrote empty storageState (tests will self-login)`);
};
