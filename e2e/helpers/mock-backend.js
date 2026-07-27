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

let notifications = [{
  id: 7,
  user_id: 1,
  type: 'low_stock',
  title: 'مخزون منخفض',
  message: 'فلتر زيت — الكمية المتبقية: 2',
  resource: 'product',
  resource_id: '12',
  read: false,
  created_at: '2026-02-14T10:00:00+03:00',
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
    json(res, 200, { count: notifications.filter((item) => !item.read).length });
    return;
  }

  if (req.method === 'GET' && req.url.startsWith('/api/v2/notification?')) {
    json(res, 200, {
      items: notifications,
      data: notifications,
      next_cursor: '',
      prev_cursor: '',
      has_more: false,
    });
    return;
  }

  const markRead = req.url.match(/^\/api\/v2\/notification\/(\d+)\/read$/);
  if (req.method === 'PUT' && markRead) {
    const item = notifications.find((notification) => notification.id === Number(markRead[1]));
    if (!item) {
      json(res, 404, { detail: 'notification not found' });
      return;
    }
    item.read = true;
    json(res, 200, { detail: 'success' });
    return;
  }

  if (req.method === 'PUT' && req.url === '/api/v2/notification/read-all') {
    const count = notifications.filter((item) => !item.read).length;
    notifications = notifications.map((item) => ({ ...item, read: true }));
    json(res, 200, { detail: 'success', count });
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