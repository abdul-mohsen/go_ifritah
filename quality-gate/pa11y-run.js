// pa11y runner — uses cookie-header.txt for auth, writes per-route JSON to reports/pa11y/.
const fs = require('fs');
const path = require('path');
const pa11y = require('pa11y');
const { base, routes } = require('./routes');

const cookie = fs.readFileSync(path.join(__dirname, 'cookie-header.txt'), 'utf8').trim();
const outDir = path.join(__dirname, 'reports', 'pa11y');
fs.mkdirSync(outDir, { recursive: true });

const opts = {
  standard: 'WCAG2AA',
  timeout: 45000,
  chromeLaunchConfig: { args: ['--no-sandbox', '--ignore-certificate-errors'] },
  headers: { Cookie: cookie },
};

(async () => {
  const summary = [];
  for (const r of routes) {
    const url = base + r.path;
    process.stdout.write(`[pa11y] ${r.name.padEnd(14)} ${url} ... `);
    try {
      const res = await pa11y(url, opts);
      const errors = res.issues.filter(i => i.type === 'error').length;
      const warnings = res.issues.filter(i => i.type === 'warning').length;
      const notices = res.issues.filter(i => i.type === 'notice').length;
      fs.writeFileSync(path.join(outDir, r.name + '.json'),
        JSON.stringify({ route: r.path, errors, warnings, notices, issues: res.issues }, null, 2));
      summary.push({ route: r.path, errors, warnings, notices });
      console.log(`E:${errors} W:${warnings} N:${notices}`);
    } catch (e) {
      console.log('ERR ' + e.message);
      summary.push({ route: r.path, error: e.message });
    }
  }
  fs.writeFileSync(path.join(outDir, '_summary.json'), JSON.stringify(summary, null, 2));
  const totalE = summary.reduce((a, b) => a + (b.errors || 0), 0);
  const totalW = summary.reduce((a, b) => a + (b.warnings || 0), 0);
  const errored = summary.filter(s => s.error).length;
  console.log(`\n[pa11y] total errors=${totalE} warnings=${totalW} routes=${summary.length} fetchErrors=${errored}`);
  process.exit((totalE > 0 || errored > 0) ? 1 : 0);
})();
