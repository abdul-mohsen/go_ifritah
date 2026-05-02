// QA-23: /api/branch/{id}/store-address bridge.
//
// The ZATCA daemon reads Taxpayer.City directly from store.city with NO
// fallback; branch_zatca_config has no city column. The Go bridge handler
// at /api/branch/{id}/store-address proxies a per-branch store row:
//   GET  → fetches branch detail, finds first linked store, returns its
//          address fields (city, street_name, building_number, district,
//          postal_code, country).
//   PUT  → validates city required; fetches branch detail; if a store
//          exists it preserves the name and PUTs; otherwise it POSTs a
//          new store named "<branch.name> - Main".

const { test, expect } = require('@playwright/test');
const { login } = require('../helpers/qa');

async function firstBranchId(page) {
  // The frontend does not expose /api/branches/all; scrape the rendered
  // /dashboard/branches HTML for the first branch's edit link.
  const r = await page.request.get('/dashboard/branches');
  if (!r.ok()) return null;
  const html = await r.text();
  // Match the per-row edit link pattern emitted by templates/branches.html.
  const m = html.match(/\/dashboard\/branches\/(\d+)\/edit/);
  return m ? m[1] : null;
}

// Read csrf_token cookie set by the FE CSRFMiddleware so PUT/POST requests
// pass the double-submit check (X-CSRF-Token must equal cookie value).
async function csrfHeader(page) {
  const cookies = await page.context().cookies();
  const c = cookies.find((x) => x.name === 'csrf_token');
  return c ? { 'X-CSRF-Token': c.value } : {};
}

// The /api/branch/{id}/store-address GET response is wrapped:
//   { detail: { city, street_name, ... }, linked: bool, store_id: number }
// Unwrap to the address fields.
function addressFrom(body) {
  if (body && typeof body === 'object' && body.detail && typeof body.detail === 'object') {
    return body.detail;
  }
  return body || {};
}

test.describe('Store-address bridge', () => {
  test('GET returns address shape with required keys', async ({ page }) => {
    await login(page);
    const id = await firstBranchId(page);
    if (!id) test.skip(true, 'No branch available on dev backend.');
    const r = await page.request.get(`/api/branch/${id}/store-address`);
    if (r.status() === 404) test.skip(true, 'No linked store yet.');
    expect(r.ok()).toBeTruthy();
    const body = await r.json();
    const address = addressFrom(body);
    // Body MUST expose city (the daemon-required field)
    expect(address).toHaveProperty('city');
  });

  test('PUT with empty city is rejected', async ({ page }) => {
    await login(page);
    const id = await firstBranchId(page);
    if (!id) test.skip(true, 'No branch available.');
    const r = await page.request.put(`/api/branch/${id}/store-address`, {
      data: { city: '', street_name: 'X' },
      headers: { 'Content-Type': 'application/json', ...(await csrfHeader(page)) },
    });
    expect(r.status()).toBeGreaterThanOrEqual(400);
    expect(r.status()).toBeLessThan(500);
  });

  test('PUT round-trips: write a city then GET reads it back', async ({ page }) => {
    await login(page);
    const id = await firstBranchId(page);
    if (!id) test.skip(true, 'No branch available.');

    const csrf = await csrfHeader(page);

    // Read existing
    const before = await page.request.get(`/api/branch/${id}/store-address`);
    const beforeBody = before.ok() ? addressFrom(await before.json()) : {};
    const cityProbe = 'الرياض-QA23';

    const put = await page.request.put(`/api/branch/${id}/store-address`, {
      data: {
        city: cityProbe,
        street_name: beforeBody.street_name || 'شارع الاختبار',
        building_number: beforeBody.building_number || '0001',
        district: beforeBody.district || 'العليا',
        postal_code: beforeBody.postal_code || '12345',
        country: beforeBody.country || 'SA',
      },
      headers: { 'Content-Type': 'application/json', ...csrf },
    });
    expect(put.ok(), `PUT must succeed (status ${put.status()})`).toBeTruthy();

    const after = await page.request.get(`/api/branch/${id}/store-address`);
    expect(after.ok()).toBeTruthy();
    const afterBody = addressFrom(await after.json());
    expect(afterBody.city).toBe(cityProbe);

    // Restore previous city if there was one
    if (beforeBody.city && beforeBody.city !== cityProbe) {
      await page.request.put(`/api/branch/${id}/store-address`, {
        data: { ...beforeBody },
        headers: { 'Content-Type': 'application/json', ...csrf },
      });
    }
  });
});
