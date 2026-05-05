// Shared route catalog for UI/UX specs — covers the public + dashboard surface.
const ROUTES_PUBLIC = [
  { name: 'home', path: '/' },
  { name: 'login', path: '/login' },
];

const ROUTES_DASHBOARD = [
  { name: 'dashboard', path: '/dashboard' },
  { name: 'invoices', path: '/dashboard/invoices' },
  { name: 'products', path: '/dashboard/products' },
  { name: 'clients', path: '/dashboard/clients' },
  { name: 'suppliers', path: '/dashboard/suppliers' },
  { name: 'branches', path: '/dashboard/branches' },
  { name: 'stores', path: '/dashboard/stores' },
  { name: 'orders', path: '/dashboard/orders' },
  { name: 'purchasebills', path: '/dashboard/purchase-bills' },
  { name: 'cashvouchers', path: '/dashboard/cash-vouchers' },
  { name: 'notifications', path: '/dashboard/notifications' },
  { name: 'settings', path: '/dashboard/settings' },
];

// List pages that expose the unified list-toolbar (q + sort + per-page).
const ROUTES_LIST = [
  { name: 'invoices', path: '/dashboard/invoices' },
  { name: 'products', path: '/dashboard/products' },
  { name: 'clients', path: '/dashboard/clients' },
  { name: 'suppliers', path: '/dashboard/suppliers' },
  { name: 'branches', path: '/dashboard/branches' },
  { name: 'stores', path: '/dashboard/stores' },
  { name: 'orders', path: '/dashboard/orders' },
  { name: 'purchasebills', path: '/dashboard/purchase-bills' },
  { name: 'cashvouchers', path: '/dashboard/cash-vouchers' },
];

module.exports = { ROUTES_PUBLIC, ROUTES_DASHBOARD, ROUTES_LIST };
