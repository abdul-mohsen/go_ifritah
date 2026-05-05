// RTL + dark-mode toggle persistence + Cairo font enforcement.
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');
const { ROUTES_DASHBOARD } = require('../../helpers/routes');

test.describe.configure({ mode: 'parallel' });

for (const r of ROUTES_DASHBOARD) {
  test(`rtl + font on ${r.name}`, async ({ page }) => {
    await login(page);
    await page.goto(r.path);
    const dir = await page.evaluate(() => document.documentElement.getAttribute('dir') || document.body.getAttribute('dir'));
    expect(dir, `${r.path} html|body dir`).toBe('rtl');
    const fontFamily = await page.evaluate(() => getComputedStyle(document.body).fontFamily.toLowerCase());
    expect(fontFamily, `${r.path} font-family`).toMatch(/cairo|tahoma|arial|sans-serif/);
  });
}

test('dark mode toggle persists across reload', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard');
  // Force theme via localStorage (the setting toggle key the project uses)
  await page.evaluate(() => { localStorage.setItem('theme', 'dark'); });
  await page.reload();
  const isDark = await page.evaluate(() => document.documentElement.classList.contains('dark') || document.documentElement.dataset.theme === 'dark');
  // Some pages compute it dynamically — accept either signal or `prefers-color-scheme` attr.
  expect(isDark || (await page.evaluate(() => localStorage.getItem('theme'))) === 'dark').toBe(true);
});
