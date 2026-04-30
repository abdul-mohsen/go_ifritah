const { test, expect } = require('@playwright/test');

// Helper: login and return authenticated page
async function login(page) {
  await page.goto('/login');
  await page.fill('input[name="username"]', 'ssda');
  await page.fill('input[name="password"]', 'Qwerty123');
  await page.click('button[type="submit"]');
  await page.waitForURL('**/dashboard**');
}

module.exports = { login };
