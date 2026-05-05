// Animation flicker regression: with sidebar sections forced closed in
// localStorage, no transition should fire within the first 500ms after
// navigation. This catches the navbar-flicker bug we fixed via
// preload-no-anim.
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_DASHBOARD } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_DASHBOARD) {
  test(`no flicker on ${r.name}`, async ({ page, context }) => {
    await login(page);
    // Pre-close all sidebar sections so the body-end IIFE *would* trigger a
    // visible animation if the preload-no-anim fix were absent.
    await context.addInitScript(() => {
      try {
        localStorage.setItem('sb-sections', JSON.stringify({
          invoices: 1, products: 1, clients: 1, suppliers: 1,
          settings: 1, sales: 1, purchases: 1, inventory: 1,
        }));
      } catch (e) {}
    });
    await page.addInitScript(() => {
      window.__animEvents = [];
      const cap = (type) => (e) => {
        const cs = e.target && e.target.nodeType === 1 ? getComputedStyle(e.target) : null;
        const dur = cs ? (type.startsWith('transition') ? cs.transitionDuration : cs.animationDuration) : '';
        window.__animEvents.push({ type, t: performance.now(), dur });
      };
      document.addEventListener('transitionrun', cap('transitionrun'), true);
      document.addEventListener('animationstart', cap('animationstart'), true);
    });
    await page.goto(r.path, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(500);
    const offenders = await page.evaluate(() =>
      (window.__animEvents || []).filter(e => e.t < 500 && e.dur && !/^0s/.test(e.dur))
    );
    expect(offenders, `${r.path} flicker offenders`).toEqual([]);
  });
}
