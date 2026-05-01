const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './tests',
  timeout: 30000,
  retries: 0,
  // Settings (stock_enforcement, pb_pdf_required, etc.) are global server
  // state. Running spec files in parallel races on those values, causing
  // intermittent failures. Force a single worker for determinism.
  workers: 1,
  use: {
    baseURL: 'http://localhost:8001',
    headless: true,
  },
  reporter: [
    ['list'],
    ['json', { outputFile: 'playwright-results.json' }],
  ],
});
