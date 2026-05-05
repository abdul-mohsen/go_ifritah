// animation-flash probe: detects elements that fire transitions/animations during
// the first 500ms after navigation. This catches the "flicker on reload" class of
// bug we recently fixed (collapsing nav sections animating from open->closed).
//
// Strategy: open Playwright chromium, set localStorage so collapsible sidebar
// sections start collapsed, then navigate and listen for transitionrun /
// animationstart events on document during the first 500ms.
const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const { base, routes } = require('./routes');

const cookie = fs.readFileSync(path.join(__dirname, 'cookie-header.txt'), 'utf8').trim();
const outDir = path.join(__dirname, 'reports', 'animation-flash');
fs.mkdirSync(outDir, { recursive: true });

function parseCookies(header, domain) {
  return header.split(';').map(s => s.trim()).filter(Boolean).map(kv => {
    const i = kv.indexOf('=');
    return { name: kv.slice(0, i), value: kv.slice(i + 1), domain, path: '/' };
  });
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1280, height: 800 } });
  const u = new URL(base);
  await ctx.addCookies(parseCookies(cookie, u.hostname));
  // Pre-seed localStorage so collapsible sections are CLOSED. This is exactly
  // the state that triggered the visible flicker before the preload-no-anim fix.
  await ctx.addInitScript(() => {
    try {
      const sections = { invoices: 1, products: 1, clients: 1, suppliers: 1, settings: 1, sales: 1, purchases: 1, inventory: 1 };
      localStorage.setItem('sb-sections', JSON.stringify(sections));
    } catch (e) { }
  });

  const page = await ctx.newPage();
  // Inject probe before any document scripts so it can capture the very first events.
  await page.addInitScript(() => {
    window.__animEvents = [];
    const cap = (type) => (e) => {
      try {
        const t = e.target;
        const sel = t && t.tagName ? (t.tagName.toLowerCase() + (t.id ? '#' + t.id : '') + (t.className && typeof t.className === 'string' ? '.' + t.className.trim().split(/\s+/).slice(0, 2).join('.') : '')) : '?';
        const cs = t && t.nodeType === 1 ? getComputedStyle(t) : null;
        const dur = cs ? (type.startsWith('transition') ? cs.transitionDuration : cs.animationDuration) : '';
        window.__animEvents.push({ type, sel, t: performance.now(), prop: e.propertyName || e.animationName || '', dur });
      } catch (err) { }
    };
    document.addEventListener('transitionrun', cap('transitionrun'), true);
    document.addEventListener('transitionstart', cap('transitionstart'), true);
    document.addEventListener('animationstart', cap('animationstart'), true);
  });

  const summary = [];
  for (const r of routes) {
    const url = base + r.path;
    process.stdout.write(`[anim] ${r.name.padEnd(14)} ${url} ... `);
    try {
      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 15000 });
      // Wait the same 500ms window the fix targets.
      await page.waitForTimeout(500);
      const events = await page.evaluate(() => window.__animEvents || []);
      // Reset for next route
      await page.evaluate(() => { window.__animEvents = []; });
      // An "offender" is any event firing within first 500ms with non-zero duration.
      const offenders = events.filter(e => e.t < 500 && e.dur && !/^0s/.test(e.dur));
      fs.writeFileSync(path.join(outDir, r.name + '.json'),
        JSON.stringify({ route: r.path, total: events.length, offenders: offenders.length, raw: offenders.slice(0, 50) }, null, 2));
      summary.push({ route: r.path, total: events.length, offenders: offenders.length });
      console.log(`events=${events.length} offenders=${offenders.length}`);
    } catch (e) {
      console.log('ERR ' + e.message);
      summary.push({ route: r.path, error: e.message });
    }
  }
  await browser.close();
  fs.writeFileSync(path.join(outDir, '_summary.json'), JSON.stringify(summary, null, 2));
  const total = summary.reduce((a, b) => a + (b.offenders || 0), 0);
  const errored = summary.filter(s => s.error).length;
  console.log(`\n[anim] total offenders=${total} routes=${summary.length} errors=${errored}`);
  process.exit((total > 0 || errored > 0) ? 1 : 0);
})();
