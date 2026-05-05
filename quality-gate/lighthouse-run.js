// lighthouse runner. Spawns chrome via chrome-launcher (bundled with lighthouse).
const fs = require('fs');
const path = require('path');
const lighthouse = require('lighthouse').default || require('lighthouse');
const chromeLauncher = require('chrome-launcher');
const { base, routes } = require('./routes');

const cookieHeader = fs.readFileSync(path.join(__dirname, 'cookie-header.txt'), 'utf8').trim();
const outDir = path.join(__dirname, 'reports', 'lighthouse');
fs.mkdirSync(outDir, { recursive: true });

(async () => {
  const chrome = await chromeLauncher.launch({ chromeFlags: ['--headless=new', '--no-sandbox', '--ignore-certificate-errors'] });
  const summary = [];
  try {
    for (const r of routes) {
      const url = base + r.path;
      process.stdout.write(`[lh] ${r.name.padEnd(14)} ${url} ... `);
      try {
        const result = await lighthouse(url, {
          port: chrome.port,
          output: 'json',
          logLevel: 'error',
          extraHeaders: { Cookie: cookieHeader },
          onlyCategories: ['performance', 'accessibility', 'best-practices', 'seo'],
        });
        const c = result.lhr.categories;
        const row = {
          route: r.path,
          performance: Math.round((c.performance?.score ?? 0) * 100),
          accessibility: Math.round((c.accessibility?.score ?? 0) * 100),
          bestPractices: Math.round((c['best-practices']?.score ?? 0) * 100),
          seo: Math.round((c.seo?.score ?? 0) * 100),
        };
        fs.writeFileSync(path.join(outDir, r.name + '.json'), JSON.stringify(row, null, 2));
        summary.push(row);
        console.log(`P:${row.performance} A:${row.accessibility} BP:${row.bestPractices} SEO:${row.seo}`);
      } catch (e) {
        console.log('ERR ' + e.message);
        summary.push({ route: r.path, error: e.message });
      }
    }
  } finally {
    await chrome.kill();
  }
  fs.writeFileSync(path.join(outDir, '_summary.json'), JSON.stringify(summary, null, 2));
  console.log(`\n[lh] routes=${summary.length}`);
  process.exit(0);
})();
