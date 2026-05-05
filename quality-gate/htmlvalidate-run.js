// html-validate runner: snapshots HTML for each route via fetch (with auth cookie),
// runs html-validate, writes per-route JSON.
const fs = require('fs');
const path = require('path');
const http = require('http');
const { HtmlValidate } = require('html-validate');
const { base, routes } = require('./routes');

const cookie = fs.readFileSync(path.join(__dirname, 'cookie-header.txt'), 'utf8').trim();
const outDir = path.join(__dirname, 'reports', 'htmlvalidate');
fs.mkdirSync(outDir, { recursive: true });

function fetchHtml(url) {
  return new Promise((resolve, reject) => {
    const req = http.get(url, { headers: { Cookie: cookie, 'User-Agent': 'qg' } }, res => {
      let body = '';
      res.on('data', c => body += c);
      res.on('end', () => resolve({ status: res.statusCode, body }));
    });
    req.on('error', reject);
    req.setTimeout(15000, () => req.destroy(new Error('timeout')));
  });
}

(async () => {
  const hv = new HtmlValidate({
    extends: ['html-validate:recommended'],
    rules: {
      // Project conventions
      'void-style': 'off',
      'no-trailing-whitespace': 'off',
      'attr-quotes': 'off',
      'doctype-style': 'off',
    },
  });
  const summary = [];
  for (const r of routes) {
    const url = base + r.path;
    process.stdout.write(`[hv] ${r.name.padEnd(14)} ${url} ... `);
    try {
      const { status, body } = await fetchHtml(url);
      const report = await hv.validateString(body);
      const errors = report.results.reduce((s, r) => s + r.errorCount, 0);
      const warnings = report.results.reduce((s, r) => s + r.warningCount, 0);
      fs.writeFileSync(path.join(outDir, r.name + '.json'),
        JSON.stringify({ route: r.path, status, errors, warnings, results: report.results }, null, 2));
      summary.push({ route: r.path, status, errors, warnings });
      console.log(`status=${status} E:${errors} W:${warnings}`);
    } catch (e) {
      console.log('ERR ' + e.message);
      summary.push({ route: r.path, error: e.message });
    }
  }
  fs.writeFileSync(path.join(outDir, '_summary.json'), JSON.stringify(summary, null, 2));
  const totalE = summary.reduce((a, b) => a + (b.errors || 0), 0);
  console.log(`\n[hv] total errors=${totalE} routes=${summary.length}`);
  process.exit(0);
})();
