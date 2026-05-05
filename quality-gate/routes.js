// Routes to scan. Keys mirror filenames in reports/<tool>/.
module.exports = {
  base: process.env.QG_BASE || 'http://127.0.0.1:8000',
  routes: [
    { name: 'home',          path: '/',                       auth: false },
    { name: 'login',         path: '/login',                  auth: false },
    { name: 'dashboard',     path: '/dashboard',              auth: true  },
    { name: 'invoices',      path: '/dashboard/invoices',     auth: true  },
    { name: 'products',      path: '/dashboard/products',     auth: true  },
    { name: 'clients',       path: '/dashboard/clients',      auth: true  },
    { name: 'suppliers',     path: '/dashboard/suppliers',    auth: true  },
    { name: 'branches',      path: '/dashboard/branches',     auth: true  },
    { name: 'stores',        path: '/dashboard/stores',       auth: true  },
    { name: 'orders',        path: '/dashboard/orders',       auth: true  },
    { name: 'purchasebills', path: '/dashboard/purchase-bills', auth: true },
    { name: 'cashvouchers',  path: '/dashboard/cash-vouchers',auth: true  },
    { name: 'notifications', path: '/dashboard/notifications',auth: true  },
    { name: 'settings',      path: '/dashboard/settings',     auth: true  },
  ],
};
