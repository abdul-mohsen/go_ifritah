const http = require('http');

const port = Number(process.env.MOCK_BACKEND_PORT || 19081);

function json(res, status, body) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
}

function token(role = 'admin') {
  const payload = Buffer.from(JSON.stringify({
    user_id: 1,
    username: 'admin',
    email: 'admin@example.com',
    role,
  })).toString('base64url');
  return `local.${payload}.signature`;
}

function list(items) {
  return { data: items, items, results: items };
}

const stores = [{ id: 4, name: 'Main Store' }];
const suppliers = [{
  id: 251,
  name: 'Local Supplier',
  preferred_payment_method: 10,
  payment_terms_days: 0,
  credit_limit: 0,
}];

http.createServer((req, res) => {
  if (req.method === 'POST' && req.url === '/api/v2/login') {
    json(res, 200, { access_token: token(), refresh_token: token('admin') });
    return;
  }

  if (req.method === 'GET' && req.url === '/api/v2/stores/all') {
    json(res, 200, list(stores));
    return;
  }

  if (req.method === 'POST' && req.url === '/api/v2/supplier/all') {
    json(res, 200, list(suppliers));
    return;
  }

  if (req.method === 'GET' && req.url === '/api/v2/notification/unread-count') {
    json(res, 200, { count: 0 });
    return;
  }

  if (req.method === 'POST' && req.url === '/api/v2/purchase_bill') {
    json(res, 500, { error: 'zero-total guard should stop this request first' });
    return;
  }

  json(res, 200, list([]));
}).listen(port, '127.0.0.1', () => {
  console.log(`mock backend listening on http://127.0.0.1:${port}`);
});