// Playwright config — covers existing zatca specs and the broad UI/UX suite
// under tests/ui-ux/. The `parallel` project is what scripts/run_e2e_ui_ux.ps1
// invokes.
const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './tests',
  globalSetup: require.resolve('./global-setup.js'),
  timeout: 60_000,
  expect: { timeout: 7_000 },
  fullyParallel: true,
  retries: 0,
  workers: process.env.PW_WORKERS ? parseInt(process.env.PW_WORKERS, 10) : 4,
  reporter: 'list',
  use: {
    baseURL: process.env.PW_BASE_URL || 'http://127.0.0.1:8000',
    headless: true,
    ignoreHTTPSErrors: true,
    actionTimeout: 7_000,
    navigationTimeout: 20_000,
    viewport: { width: 1280, height: 800 },
    locale: 'ar-SA',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'parallel',
      use: { storageState: '.auth/storageState.json' },
    },
    {
      name: 'mobile',
      use: { viewport: { width: 390, height: 844 } },
      testMatch: /tests[\\/]ui-ux[\\/].*\.spec\.js$/,
    },
  ],
});
