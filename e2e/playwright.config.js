const { defineConfig } = require('@playwright/test');

// Specs that mutate global server state (settings, ZATCA per-branch config)
// MUST run sequentially. Everything else is independent of shared state and
// can be parallelised safely.
const SERIAL_SPECS = [
  '**/qa-01-stock-enforcement.spec.js',
  '**/qa-03-settings.spec.js',
  '**/qa-14-concurrency.spec.js',
  '**/qa-15-stock-enforcement-behavior.spec.js',
  '**/qa-19-pb-pdf-required.spec.js',
  '**/zatca-settings.spec.js',
  '**/zatca-save-qa.spec.js',
];

const resultsFile = process.env.PW_RESULTS_FILE || 'playwright-results.json';
const MOBILE_MATCH = /tests[\\/]ui-ux[\\/].*\.spec\.js$/;

// Workers is a global Playwright option (not per-project). CI runs each
// project in its own `npx playwright test --project=…` invocation so each
// uses the correct worker count (see .github/workflows/e2e.yml).
//   --project=serial   → SERIAL_SPECS, run with --workers=1
//   --project=parallel → everything else, run with --workers=N (auth via storageState)
//   --project=mobile   → tests/ui-ux/* on a phone viewport
module.exports = defineConfig({
  testDir: './tests',
  globalSetup: require.resolve('./global-setup.js'),
  timeout: 60_000,
  expect: { timeout: 7_000 },
  retries: 0,
  fullyParallel: true,
  workers: parseInt(process.env.PW_WORKERS || '1', 10),
  reporter: [
    ['list'],
    ['json', { outputFile: resultsFile }],
  ],
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
      name: 'serial',
      testMatch: SERIAL_SPECS,
      fullyParallel: false,
    },
    {
      name: 'parallel',
      testIgnore: SERIAL_SPECS,
      fullyParallel: true,
      use: { storageState: '.auth/storageState.json' },
    },
    {
      name: 'mobile',
      testMatch: MOBILE_MATCH,
      use: {
        storageState: '.auth/storageState.json',
        viewport: { width: 390, height: 844 },
      },
    },
  ],
});
