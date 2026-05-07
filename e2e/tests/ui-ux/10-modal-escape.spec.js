// Modals: opening any modal and pressing Escape must close it (script.js Escape handler).
const { test, expect } = require('@playwright/test');
const { login } = require('../../helpers/auth');

test.describe.configure({ mode: 'parallel' });

test('Escape closes open modal on dashboard', async ({ page }) => {
  await login(page);
  await page.goto('/dashboard');
  const modals = page.locator('.fixed:not(.hidden)');
  const count = await modals.count();
  // Inject a sacrificial test modal so the test is deterministic regardless of page chrome.
  await page.evaluate(() => {
    const m = document.createElement('div');
    m.id = 'test-modal-x';
    m.className = 'fixed';
    m.style.cssText = 'inset:0;background:#0008;z-index:9999';
    document.body.appendChild(m);
  });
  expect(await page.locator('#test-modal-x').isVisible()).toBe(true);
  await page.keyboard.press('Escape');
  // Either the element is removed, hidden, or has the .hidden class.
  await page.waitForFunction(() => {
    const m = document.getElementById('test-modal-x');
    return !m || m.classList.contains('hidden') || getComputedStyle(m).display === 'none';
  }, null, { timeout: 5000 });
});
