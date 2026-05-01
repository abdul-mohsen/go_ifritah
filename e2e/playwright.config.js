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

// Workers is a global Playwright option (not per-project). CI runs the two
// groups in separate `npx playwright test --project=…` invocations so each
// uses the correct worker count (see .github/workflows/e2e.yml).
//   --project=serial   → SERIAL_SPECS, run with --workers=1
//   --project=parallel → everything else, run with --workers=N
module.exports = defineConfig({
  testDir: './tests',
  timeout: 30000,
  retries: 0,
  fullyParallel: true,
  workers: parseInt(process.env.PW_WORKERS || '1', 10),
  use: {
    baseURL: 'http://localhost:8001',
    headless: true,
  },
  reporter: [
    ['list'],
    ['json', { outputFile: resultsFile }],
  ],
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
  ],
});
