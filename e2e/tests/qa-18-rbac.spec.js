// QA-18: RBAC role matrix.
//
// Logs in as each of the three demo users (admin / manager / employee) and
// verifies what each role can reach. Admin can hit every page; manager is
// blocked from /dashboard/settings; employee is additionally blocked from
// branches/stores/cash_vouchers approval routes (per handlers/rbac.go).
//
// NOTE: previous version of this file also exercised /dashboard/users — that
// page was removed (no real backend CRUD exists). When backend exposes
// /api/v2/users we'll re-add coverage here.

const { test, expect } = require('@playwright/test');

const USERS = [
  { role: 'admin', user: 'admin', pass: 'admin' },
  { role: 'manager', user: 'manager', pass: 'manager' },
  { role: 'employee', user: 'employee', pass: 'employee' },
];

async function loginAs(page, user, pass) {
  await page.goto('/login');
  await page.fill('input[name="username"]', user);
  await page.fill('input[name="password"]', pass);
  await Promise.all([
    page.waitForURL('**/dashboard**', { timeout: 15000 }),
    page.click('button[type="submit"]'),
  ]);
}

// Pages everyone authenticated should reach (no resource-specific RBAC).
const COMMON_PAGES = [
  '/dashboard',
  '/dashboard/notifications',
];

// Role expectation matrix. true = should be reachable (HTTP < 400),
// false = blocked. Admins always pass-through; manager is blocked from
// settings only; employee starts with NO resource permissions seeded so
// almost everything is 403 except the always-allowed pages above.
const PAGE_MATRIX = {
  '/dashboard/settings': { admin: true, manager: false, employee: false },
  '/dashboard/branches': { admin: true, manager: true, employee: false },
  '/dashboard/stores': { admin: true, manager: true, employee: false },
  '/dashboard/suppliers': { admin: true, manager: true, employee: false },
  '/dashboard/purchase-bills': { admin: true, manager: true, employee: false },
  '/dashboard/cash-vouchers': { admin: true, manager: true, employee: false },
  '/dashboard/invoices': { admin: true, manager: true, employee: false },
  '/dashboard/products': { admin: true, manager: true, employee: false },
  '/dashboard/clients': { admin: true, manager: true, employee: false },
  '/dashboard/orders': { admin: true, manager: true, employee: false },
};

for (const u of USERS) {
  test.describe(`role=${u.role}`, () => {
    test.beforeEach(async ({ page }) => {
      await loginAs(page, u.user, u.pass);
    });

    test(`${u.role}: common authenticated pages all render`, async ({ page }) => {
      for (const path of COMMON_PAGES) {
        const resp = await page.goto(path);
        expect(resp.status(), `${u.role} → ${path}`).toBeLessThan(400);
      }
    });

    test(`${u.role}: page matrix matches RBAC`, async ({ page }) => {
      for (const [path, expectations] of Object.entries(PAGE_MATRIX)) {
        const allowed = expectations[u.role];
        const resp = await page.goto(path);
        const status = resp.status();
        if (allowed) {
          expect(status, `${u.role} should reach ${path}`).toBeLessThan(400);
        } else {
          // Either explicit 403, or redirect away from the protected route.
          const url = page.url();
          const blocked = status >= 400 || !url.includes(path);
          expect(blocked, `${u.role} should be blocked from ${path} (got ${status} at ${url})`).toBe(true);
        }
      }
    });
  });
}

// Sidebar sanity: admin must see settings link, others must not.
test.describe('sidebar visibility', () => {
  test('admin sees settings link', async ({ page }) => {
    await loginAs(page, 'admin', 'admin');
    const link = page.locator('a[href="/dashboard/settings"]').first();
    await expect(link, 'admin should have settings link in sidebar').toBeVisible();
  });

  test('manager does not see settings link', async ({ page }) => {
    await loginAs(page, 'manager', 'manager');
    // The link is rendered but should be hidden by data-role="admin" filter.
    const visible = await page.locator('a[href="/dashboard/settings"]:visible').count();
    expect(visible, 'manager must not have visible settings link').toBe(0);
  });

  test('employee does not see settings link', async ({ page }) => {
    await loginAs(page, 'employee', 'employee');
    const visible = await page.locator('a[href="/dashboard/settings"]:visible').count();
    expect(visible, 'employee must not have visible settings link').toBe(0);
  });
});

// Unauthenticated paths.
test.describe('unauthenticated', () => {
  // Override the project storageState (which is authenticated) with an
  // anonymous one so the page fixture starts logged-out.
  test.use({ storageState: { cookies: [], origins: [] } });

  test('hitting protected page redirects to login', async ({ page }) => {
    await page.goto('/dashboard/settings');
    expect(page.url()).toMatch(/\/(login)?$/);
  });

  test('tampered session cookie is treated as unauthenticated', async ({ context, page }) => {
    await context.addCookies([
      { name: 'session_id', value: 'not-real', domain: 'localhost', path: '/' },
    ]);
    await page.goto('/dashboard/settings');
    expect(page.url()).toMatch(/\/(login)?$/);
  });
});
