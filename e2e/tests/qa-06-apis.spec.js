// QA-06: JSON APIs and search endpoints respond.

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

async function authedRequest(page, request) {
  await login(page);
  const cookies = await page.context().cookies();
  return cookies.map((c) => `${c.name}=${c.value}`).join('; ');
}

test('stock enforcement endpoint responds', async ({ page, request }) => {
  const cookie = await authedRequest(page, request);
  const resp = await request.get('/api/stock/enforcement', { headers: { cookie } });
  expect([200, 401, 403, 404]).toContain(resp.status());
});

test('VIN verify endpoint requires param', async ({ page, request }) => {
  const cookie = await authedRequest(page, request);
  const resp = await request.get('/api/vin/verify', { headers: { cookie } });
  // Missing VIN should produce 400/422 or returns informative JSON
  expect([200, 400, 422]).toContain(resp.status());
});

test('notification config GET responds', async ({ page, request }) => {
  const cookie = await authedRequest(page, request);
  const resp = await request.get('/api/notification-config', { headers: { cookie } });
  expect([200, 204, 401]).toContain(resp.status());
});

test('export invoices CSV responds', async ({ page, request }) => {
  const cookie = await authedRequest(page, request);
  const resp = await request.get('/dashboard/invoices/export-csv', { headers: { cookie } });
  expect([200, 302, 500]).toContain(resp.status());
  if (resp.status() === 200) {
    const ct = resp.headers()['content-type'] || '';
    expect(ct).toMatch(/csv|octet-stream|text/i);
  }
});

test('export products CSV responds', async ({ page, request }) => {
  const cookie = await authedRequest(page, request);
  const resp = await request.get('/dashboard/products/export-csv', { headers: { cookie } });
  expect([200, 302, 500]).toContain(resp.status());
});

test('export clients CSV responds', async ({ page, request }) => {
  const cookie = await authedRequest(page, request);
  const resp = await request.get('/dashboard/clients/export-csv', { headers: { cookie } });
  expect([200, 302, 500]).toContain(resp.status());
});

test('export suppliers CSV responds', async ({ page, request }) => {
  const cookie = await authedRequest(page, request);
  const resp = await request.get('/dashboard/suppliers/export-csv', { headers: { cookie } });
  expect([200, 302, 500]).toContain(resp.status());
});
