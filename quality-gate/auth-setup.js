// Login against the local FE and write cookie-header.txt for the
// quality-gate runners (pa11y / lighthouse / htmlvalidate /
// animation-flash). Idempotent.
//
//   QG_BASE       base URL of the FE                (default http://127.0.0.1:8000)
//   QG_USER       login username                    (default admin)
//   QG_PASS       login password                    (default admin)
//
// Exits 0 on success, 1 on auth failure. Writes:
//   - cookie-header.txt  – the literal "Cookie:" header value
//   - storage.json       – { cookies: [{name, value}, ...] } for tools
//                          that want JSON (lighthouse passes it through).
const http = require('http');
const https = require('https');
const fs   = require('fs');
const path = require('path');
const { URL } = require('url');

const BASE = process.env.QG_BASE || 'http://127.0.0.1:8000';
const USER = process.env.QG_USER || 'admin';
const PASS = process.env.QG_PASS || 'admin';

function post(url, formBody) {
  return new Promise((resolve, reject) => {
    const u = new URL(url);
    const lib = u.protocol === 'https:' ? https : http;
    const body = Object.entries(formBody)
      .map(([k, v]) => encodeURIComponent(k) + '=' + encodeURIComponent(v))
      .join('&');
    const req = lib.request({
      method: 'POST',
      hostname: u.hostname,
      port: u.port || (u.protocol === 'https:' ? 443 : 80),
      path: u.pathname + u.search,
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Content-Length': Buffer.byteLength(body),
      },
    }, (res) => {
      const chunks = [];
      res.on('data', (c) => chunks.push(c));
      res.on('end', () => resolve({
        status: res.statusCode,
        headers: res.headers,
        body: Buffer.concat(chunks).toString('utf8'),
      }));
    });
    req.on('error', reject);
    req.write(body);
    req.end();
  });
}

(async () => {
  let attempts = 0;
  let lastErr = '';
  while (attempts < 5) {
    attempts++;
    try {
      const r = await post(BASE + '/login', { username: USER, password: PASS });
      const setCookies = r.headers['set-cookie'] || [];
      if (setCookies.length === 0) {
        lastErr = `attempt ${attempts}: status ${r.status}, no Set-Cookie`;
        await new Promise(r => setTimeout(r, 800));
        continue;
      }
      const pairs = setCookies.map((c) => c.split(';')[0]).filter(Boolean);
      const header = pairs.join('; ');
      fs.writeFileSync(path.join(__dirname, 'cookie-header.txt'), header);
      const cookies = pairs.map((p) => {
        const [name, ...rest] = p.split('=');
        return { name, value: rest.join('=') };
      });
      fs.writeFileSync(path.join(__dirname, 'storage.json'),
        JSON.stringify({ cookies }, null, 2));
      console.log(`[qg-auth] OK status=${r.status} cookies=${pairs.length}`);
      process.exit(0);
    } catch (e) {
      lastErr = `attempt ${attempts}: ${e.message}`;
      await new Promise(r => setTimeout(r, 800));
    }
  }
  console.error(`[qg-auth] FAIL: ${lastErr}`);
  process.exit(1);
})();
