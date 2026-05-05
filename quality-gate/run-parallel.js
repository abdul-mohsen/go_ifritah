// Run every QA suite in PARALLEL and aggregate failing tests per suite.
// Outputs per-suite stdout to logs/<suite>.log and a final report to
// logs/_failures.json + logs/_failures.md.
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const QG = __dirname;
const LOG_DIR = path.join(QG, 'logs');
fs.mkdirSync(LOG_DIR, { recursive: true });

// Each suite: { name, cwd, cmd, args, env?, phase? }
// Phase 1 = run alone (resource-heavy: parallel browser auth across many routes)
// Phase 2 = run together (the rest, won't fight each other for sessions)
const suites = [
  { name: 'playwright-e2e',  phase: 1, cwd: path.join(ROOT, 'e2e'), cmd: 'npx', args: ['playwright', 'test', '--project=parallel', '--reporter=list'], env: { PW_BASE_URL: 'http://127.0.0.1:8000', PW_USER: 'admin', PW_PASS: 'admin123', PW_WORKERS: '4' } },
  { name: 'go-test',         phase: 2, cwd: ROOT, cmd: 'go',  args: ['test', './handlers/...', './helpers/...', './config/...', '-count=1', '-timeout=600s', '-v'] },
  { name: 'go-vet',          phase: 2, cwd: ROOT, cmd: 'go',  args: ['vet', './handlers/...', './helpers/...', './config/...', './middleware/...'] },
  { name: 'pa11y',           phase: 2, cwd: QG,   cmd: 'node', args: ['pa11y-run.js'] },
  { name: 'lighthouse',      phase: 2, cwd: QG,   cmd: 'node', args: ['lighthouse-run.js'] },
  { name: 'htmlvalidate',    phase: 2, cwd: QG,   cmd: 'node', args: ['htmlvalidate-run.js'] },
  { name: 'animation-flash', phase: 2, cwd: QG,   cmd: 'node', args: ['animation-flash-run.js'] },
];

function runSuite(s) {
  return new Promise((resolve) => {
    const logFile = path.join(LOG_DIR, s.name + '.log');
    const out = fs.createWriteStream(logFile);
    const start = Date.now();
    console.log(`[start] ${s.name}`);
    const env = Object.assign({}, process.env, s.env || {});
    const child = spawn(s.cmd, s.args, { cwd: s.cwd, env, shell: process.platform === 'win32' });
    child.stdout.pipe(out);
    child.stderr.pipe(out);
    child.on('close', (code) => {
      const ms = Date.now() - start;
      console.log(`[done ] ${s.name} exit=${code} (${(ms/1000).toFixed(1)}s)`);
      resolve({ name: s.name, exit: code, ms, log: logFile });
    });
    child.on('error', (err) => {
      out.end('SPAWN ERROR: ' + err.message + '\n');
      resolve({ name: s.name, exit: -1, ms: Date.now() - start, log: logFile, error: err.message });
    });
  });
}

// Failure extractors per suite — return { failures: [strings], summary: string }
function extractGo(log) {
  const failures = [];
  const lines = log.split(/\r?\n/);
  for (const ln of lines) {
    const m = ln.match(/^--- FAIL: (\S+)/);
    if (m) failures.push(m[1]);
  }
  const buildErrs = lines.filter(l => /^\s*FAIL\s+\S+\s+\[build failed\]/.test(l)).map(l => l.trim());
  return { failures: failures.concat(buildErrs), summary: failures.length + ' failed test(s)' };
}
function extractPlaywright(log) {
  const failures = [];
  const re = /^\s*\d+\)\s+\[?[^\]]*\]?\s*(\S+\.spec\.\S+):\d+:\d+\s*[›>]\s*(.+?)\s*$/m;
  // Simpler: find lines starting with " ✘ " or "  1) " pattern.
  log.split(/\r?\n/).forEach(line => {
    let m = line.match(/^\s*\d+\)\s+(.*\.spec\.\S+:\d+:\d+.*)$/);
    if (m) failures.push(m[1].trim());
    else if (/^\s*✘\s+/.test(line) || /^\s*FAIL\s+/.test(line)) failures.push(line.trim());
  });
  return { failures, summary: failures.length + ' failed test(s)' };
}
function extractQG(log, prefix) {
  // Lines like "[pa11y] dashboard ... E:31 W:0 N:0" or "[hv] dashboard ... status=200 E:267 W:0" or "... ERR <msg>"
  const failures = [];
  const reLine = new RegExp('^\\[' + prefix + '\\]\\s+(\\S+)\\s+(\\S+)\\s+\\.\\.\\.\\s+(.*)$');
  log.split(/\r?\n/).forEach(line => {
    const m = line.match(reLine);
    if (!m) return;
    const route = m[1];
    const tail = m[3];
    const errMatch = tail.match(/E:(\d+)/);
    const errTxt = tail.match(/^ERR\s+(.+)$/) || tail.match(/\bERR\s+(.+)$/);
    if (errTxt) failures.push(`${route} → ERR ${errTxt[1]}`);
    else if (errMatch && +errMatch[1] > 0) failures.push(`${route} → ${errMatch[1]} error(s)`);
  });
  return { failures, summary: failures.length + ' route(s) with issues' };
}
function extractLighthouse(log) {
  const failures = [];
  log.split(/\r?\n/).forEach(line => {
    const m = line.match(/^\[lh\]\s+(\S+).*?P:(\d+)\s+A:(\d+)\s+BP:(\d+)\s+SEO:(\d+)/);
    if (m) {
      const [, route, p, a, bp, seo] = m;
      const scores = { perf: +p, a11y: +a, bp: +bp, seo: +seo };
      const bad = Object.entries(scores).filter(([, v]) => v < 90).map(([k, v]) => `${k}=${v}`);
      if (bad.length) failures.push(`${route} → ${bad.join(', ')}`);
    } else if (/^\[lh\].*ERR/.test(line)) failures.push(line.trim());
  });
  return { failures, summary: failures.length + ' route(s) below 90' };
}
function extractAnim(log) {
  const failures = [];
  log.split(/\r?\n/).forEach(line => {
    const m = line.match(/^\[anim\]\s+(\S+).*?offenders=(\d+)/);
    if (m && +m[2] > 0) failures.push(`${m[1]} → ${m[2]} offender(s)`);
    else if (/^\[anim\].*ERR/.test(line)) failures.push(line.trim());
  });
  return { failures, summary: failures.length + ' route(s) with flicker' };
}

const extractors = {
  'go-test': extractGo,
  'go-vet': (log) => {
    const failures = log.split(/\r?\n/).filter(l => /^\S+:\d+:\d+:/.test(l) || /^vet:/.test(l));
    return { failures, summary: failures.length + ' vet warning(s)' };
  },
  'playwright-e2e': extractPlaywright,
  'pa11y':       (log) => extractQG(log, 'pa11y'),
  'htmlvalidate':(log) => extractQG(log, 'hv'),
  'animation-flash': extractAnim,
  'lighthouse':  extractLighthouse,
};

(async () => {
  const t0 = Date.now();
  const phase1 = suites.filter(s => s.phase === 1);
  const phase2 = suites.filter(s => s.phase !== 1);
  console.log(`\n--- phase 1 (${phase1.length} suite, runs alone) ---`);
  const r1 = await Promise.all(phase1.map(runSuite));
  console.log(`\n--- phase 2 (${phase2.length} suites, parallel) ---`);
  const r2 = await Promise.all(phase2.map(runSuite));
  const results = r1.concat(r2);
  const wallS = ((Date.now() - t0) / 1000).toFixed(1);

  const report = { wallSeconds: +wallS, suites: [] };
  for (const r of results) {
    const log = fs.readFileSync(r.log, 'utf8');
    const ex = extractors[r.name](log);
    report.suites.push({
      name: r.name,
      exit: r.exit,
      seconds: +(r.ms / 1000).toFixed(1),
      summary: ex.summary,
      failures: ex.failures,
      log: path.relative(ROOT, r.log).replace(/\\/g, '/'),
    });
  }
  fs.writeFileSync(path.join(LOG_DIR, '_failures.json'), JSON.stringify(report, null, 2));

  // Markdown report
  const md = [];
  md.push(`# QA suite report (parallel run, ${wallS}s wall)\n`);
  md.push(`| Suite | Exit | Seconds | Result |\n|---|---|---|---|`);
  for (const s of report.suites) {
    md.push(`| ${s.name} | ${s.exit} | ${s.seconds} | ${s.summary} |`);
  }
  md.push('');
  for (const s of report.suites) {
    md.push(`\n## ${s.name} (exit=${s.exit})`);
    md.push(`- log: \`${s.log}\``);
    md.push(`- ${s.summary}`);
    if (s.failures.length) {
      md.push('');
      s.failures.forEach(f => md.push(`- ${f}`));
    }
  }
  fs.writeFileSync(path.join(LOG_DIR, '_failures.md'), md.join('\n'));
  console.log('\n=== summary ===');
  for (const s of report.suites) {
    console.log(`${s.name.padEnd(18)} exit=${s.exit}  ${s.seconds.toString().padStart(6)}s  ${s.summary}`);
  }
  console.log(`\nReport: ${path.relative(ROOT, path.join(LOG_DIR, '_failures.md'))}`);
  process.exit(0);
})();
