const { defineConfig } = require('@playwright/test');

// Specs that mutate global server state (settings, ZATCA per-branch config)
// MUST run sequentially. Everything else is independent of shared state and
// can be parallelised safely.
//
// zatca-settings.spec.js and zatca-save-qa.spec.js used to belong here too,
// but neither actually persists to the shared backend anymore: every test
// that saves ZATCA config intercepts the PUT via page.route() (see
// mockSaveSuccess/mockSaveFail in zatca-save-qa.spec.js and the equivalent
// pattern in qa-27-zatca-connect-matrix.spec.js, which already runs in the
// parallel project against the same endpoints without incident), and
// zatca-settings.spec.js never clicks the save button at all — it only
// asserts client-side DOM/validation behaviour. Verified no cross-worker
// interference running both under --project=parallel --workers=4 against
// the real dev backend before moving them here.
const SERIAL_SPECS = [
  '**/qa-01-stock-enforcement.spec.js',
  '**/qa-03-settings.spec.js',
  '**/qa-14-concurrency.spec.js',
  '**/qa-15-stock-enforcement-behavior.spec.js',
  '**/qa-19-pb-pdf-required.spec.js',
];

const resultsFile = process.env.PW_RESULTS_FILE || 'playwright-results.json';
const MOBILE_MATCH = /tests[\\/]ui-ux[\\/].*\.spec\.js$/;
const STORAGE_STATE = '.auth/storageState.json';

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
    storageState: STORAGE_STATE,
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
    },
    {
      name: 'mobile',
      testMatch: MOBILE_MATCH,
      use: {
        viewport: { width: 390, height: 844 },
      },
    },
  ],
});
