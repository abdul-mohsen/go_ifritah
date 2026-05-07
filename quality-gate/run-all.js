// Run all quality-gate suites in sequence and print a final pass/fail table.
const { spawnSync } = require('child_process');
const path = require('path');

const tools = ['pa11y', 'htmlvalidate', 'animation-flash', 'lighthouse'];
const results = {};
for (const t of tools) {
  console.log('\n=== ' + t + ' ===');
  const r = spawnSync(process.execPath, [path.join(__dirname, t + '-run.js')], { stdio: 'inherit' });
  results[t] = r.status;
}
console.log('\n=== quality-gate summary ===');
for (const t of tools) {
  console.log(`  ${t.padEnd(16)} exit=${results[t]}`);
}
const failed = Object.values(results).some(x => x !== 0);
process.exit(failed ? 1 : 0);
