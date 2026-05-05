// Global setup: log in once via the API and persist storage state so
// the parallel workers don't all hammer /login simultaneously.
const { request, chromium } = require('@playwright/test');
const path = require('path');
const fs = require('fs');

const BASE = process.env.PW_BASE_URL || 'http://127.0.0.1:8000';
const USER = process.env.PW_USER || 'admin';
const PASS = process.env.PW_PASS || 'admin123';
const STORAGE = path.join(__dirname, '.auth', 'storageState.json');

module.exports = async () => {
  fs.mkdirSync(path.dirname(STORAGE), { recursive: true });
  // Use a request context so the login flow doesn't depend on form
  // submit + JS redirect timing in a headless browser.
  const ctx = await request.newContext({ baseURL: BASE, ignoreHTTPSErrors: true });
  let ok = false;
  for (let i = 0; i < 5 && !ok; i++) {
    try {
      const r = await ctx.post('/login', {
        form: { username: USER, password: PASS },
        maxRedirects: 0,
      });
      // Either 200 (HTMX response with HX-Redirect) or 302/303
      if (r.status() < 400 || (r.status() >= 300 && r.status() < 400)) {
        ok = true;
        break;
      }
      console.warn(`[global-setup] login attempt ${i + 1}: status ${r.status()}`);
    } catch (e) {
      console.warn(`[global-setup] login attempt ${i + 1}: ${e.message}`);
    }
    await new Promise(r => setTimeout(r, 800));
  }
  if (!ok) throw new Error('global-setup: failed to log in after 5 attempts');
  await ctx.storageState({ path: STORAGE });
  await ctx.dispose();
  console.log(`[global-setup] storage state saved -> ${STORAGE}`);
};
