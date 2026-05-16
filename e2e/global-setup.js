// Global setup: log in once via the API and persist storage state so
// workers and separate project invocations can reuse the same session.
const { request } = require('@playwright/test');
const path = require('path');
const fs = require('fs');

const BASE = process.env.PW_BASE_URL || 'http://127.0.0.1:8000';
const USER = process.env.PW_USER || 'admin';
const PASS = process.env.PW_PASS || 'admin123';
const STORAGE = path.join(__dirname, '.auth', 'storageState.json');

async function hasUsableStoredLogin() {
  if (!fs.existsSync(STORAGE)) return false;

  const ctx = await request.newContext({
    baseURL: BASE,
    ignoreHTTPSErrors: true,
    storageState: STORAGE,
  });
  try {
    const r = await ctx.get('/dashboard', { maxRedirects: 0 });
    if (r.status() >= 200 && r.status() < 300) return true;
    console.warn(`[global-setup] stored auth rejected: status ${r.status()}`);
    return false;
  } catch (e) {
    console.warn(`[global-setup] stored auth check failed: ${e.message}`);
    return false;
  } finally {
    await ctx.dispose();
  }
}

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
  if (await hasUsableStoredLogin()) {
    console.log(`[global-setup] reusing storage state -> ${STORAGE}`);
    return;
  }

  const ok = await tryLogin();
  if (ok) {
    console.log(`[global-setup] storage state saved -> ${STORAGE}`);
    return;
  }
  // Hard-fail when global login can't succeed: silently writing an empty
  // storageState lets a misconfigured PW_USER/PW_PASS produce a green run
  // where every test falls through to the form-login fallback (which
  // swallows errors). The form-login fallback is for individual flaky
  // tests, not for "my creds are wrong" — fail fast instead.
  fs.writeFileSync(STORAGE, JSON.stringify({ cookies: [], origins: [] }));
  console.error(`[global-setup] FATAL: login failed against ${BASE} as ${USER}. Aborting run.`);
  process.exit(1);
};
