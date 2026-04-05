// ── i18n: Arabic/English Translation System ──
(function() {
    var translations = {
        ar: {
            // Navigation
            'nav.dashboard': 'لوحة التحكم',
            'nav.invoices': 'الفواتير',
            'nav.purchase_bills': 'فواتير الشراء',
            'nav.products': 'المنتجات',
            'nav.parts_search': 'البحث عن قطع',
            'nav.cars_search': 'البحث عن سيارات',
            'nav.suppliers': 'الموردين',
            'nav.clients': 'العملاء',
            'nav.orders': 'الطلبات',
            'nav.stores': 'المخازن',
            'nav.branches': 'الفروع',
            'nav.users': 'إدارة المستخدمين',
            'nav.settings': 'الإعدادات',
            'nav.logout': 'تسجيل خروج',
            'nav.cash_vouchers': 'سندات القبض والصرف',
            'nav.stock_adjustments': 'تعديل المخزون',

            // Settings sidebar
            'settings.dark_mode': 'الوضع الداكن',
            'settings.language': 'English',

            // Common
            'common.loading': 'جاري التحميل...',
            'common.save': 'حفظ',
            'common.cancel': 'إلغاء',
            'common.delete': 'حذف',
            'common.edit': 'تعديل',
            'common.add': 'إضافة',
            'common.search': 'بحث',
            'common.actions': 'إجراءات',
            'common.back': 'رجوع',
            'common.yes': 'نعم',
            'common.no': 'لا',
            'common.confirm_delete': 'هل أنت متأكد من الحذف؟',
            'common.no_results': 'لا توجد نتائج',
            'common.total': 'الإجمالي',
            'common.status': 'الحالة',
            'common.date': 'التاريخ',
            'common.amount': 'المبلغ',
            'common.name': 'الاسم',
            'common.phone': 'الهاتف',
            'common.email': 'البريد الإلكتروني',
            'common.address': 'العنوان',
            'common.notes': 'ملاحظات',
            'common.print': 'طباعة',
            'common.export': 'تصدير',
            'common.filter': 'تصفية',
            'common.clear': 'مسح',
            'common.from': 'من',
            'common.to': 'إلى',
            'common.all': 'الكل',

            // Dashboard
            'dashboard.title': 'لوحة التحكم',
            'dashboard.total_revenue': 'إجمالي الإيرادات',
            'dashboard.total_invoices': 'عدد الفواتير',
            'dashboard.total_clients': 'عدد العملاء',
            'dashboard.total_products': 'عدد المنتجات',
            'dashboard.recent_invoices': 'أحدث الفواتير',
            'dashboard.top_clients': 'أفضل العملاء',
            'dashboard.revenue_chart': 'الإيرادات',
            'dashboard.status_chart': 'حالة الفواتير',

            // Invoices
            'invoices.title': 'الفواتير',
            'invoices.add': 'إضافة فاتورة',
            'invoices.number': 'رقم الفاتورة',
            'invoices.client': 'العميل',
            'invoices.date': 'التاريخ',
            'invoices.total': 'الإجمالي',
            'invoices.status': 'الحالة',
            'invoices.paid': 'مدفوعة',
            'invoices.unpaid': 'غير مدفوعة',
            'invoices.partial': 'مدفوعة جزئياً',

            // Products
            'products.title': 'المنتجات',
            'products.add': 'إضافة منتج',
            'products.name': 'اسم المنتج',
            'products.code': 'كود المنتج',
            'products.price': 'السعر',
            'products.quantity': 'الكمية',
            'products.category': 'التصنيف',

            // Purchase Bills
            'purchase_bills.title': 'فواتير الشراء',
            'purchase_bills.add': 'إضافة فاتورة شراء',
            'purchase_bills.supplier': 'المورد',

            // Suppliers
            'suppliers.title': 'الموردين',
            'suppliers.add': 'إضافة مورد',

            // Clients
            'clients.title': 'العملاء',
            'clients.add': 'إضافة عميل',

            // Orders
            'orders.title': 'الطلبات',
            'orders.add': 'إضافة طلب',

            // Stores
            'stores.title': 'المخازن',
            'stores.add': 'إضافة مخزن',

            // Branches
            'branches.title': 'الفروع',
            'branches.add': 'إضافة فرع',

            // Users
            'users.title': 'إدارة المستخدمين',
            'users.add': 'إضافة مستخدم',
            'users.role': 'الدور',
            'users.admin': 'مدير النظام',
            'users.manager': 'مدير',
            'users.employee': 'موظف',

            // Settings page
            'settings.title': 'الإعدادات',
            'settings.company_name': 'اسم الشركة',
            'settings.company_email': 'بريد الشركة',
            'settings.vat_number': 'الرقم الضريبي',
            'settings.phone': 'رقم الجوال',
            'settings.tax_rate': 'نسبة الضريبة (%)',
            'settings.currency': 'العملة',
            'settings.invoice_footer': 'ملاحظة الفاتورة',
            'settings.appearance': 'المظهر',
            'settings.theme': 'السمة',
            'settings.theme_light': 'فاتح',
            'settings.theme_dark': 'داكن',
            'settings.theme_auto': 'تلقائي',
            'settings.language_label': 'اللغة',
            'settings.lang_ar': 'العربية',
            'settings.lang_en': 'الإنجليزية',
            'settings.notifications': 'الإشعارات',
            'settings.invoice_alerts': 'تنبيهات الفواتير',
            'settings.low_stock_alerts': 'تنبيهات المخزون المنخفض',
            'settings.save_success': 'تم حفظ الإعدادات بنجاح',

            // Search
            'search.parts_title': 'البحث عن قطع غيار',
            'search.cars_title': 'البحث عن سيارات',
            'search.vin': 'رقم الهيكل (VIN)',
            'search.query': 'كلمة البحث',
            'search.car_name': 'اسم السيارة أو الموديل',
            'search.year': 'سنة الصنع',
            'search.make': 'الشركة المصنعة',
            'search.model': 'الموديل',
            'search.category': 'التصنيف',
            'search.condition': 'الحالة',
            'search.condition_new': 'جديد',
            'search.condition_used': 'مستعمل',
            'search.condition_all': 'الكل',
            'search.price_range': 'نطاق السعر',
            'search.min_price': 'أقل سعر',
            'search.max_price': 'أعلى سعر',
            'search.search_btn': 'بحث',
            'search.reset': 'إعادة تعيين',
            'search.advanced': 'بحث متقدم',
            'search.results': 'النتائج',
            'search.no_results': 'لا توجد نتائج مطابقة',
            'search.quick_make': 'الشركة المصنعة',
            'search.quick_filters': 'فلاتر سريعة',
            'search.more_filters': 'فلاتر إضافية',
            'search.start_searching': 'ابدأ البحث عن قطع الغيار',
            'search.search_tip': 'أدخل اسم القطعة أو رقمها للبحث',
            'search.start_car_search': 'ابدأ البحث عن السيارات',
            'search.car_search_tip': 'أدخل اسم السيارة أو الموديل للبحث',
            'search.cars_query_placeholder': 'ابحث عن سيارة...',
            'search.parts_query_placeholder': 'ابحث عن قطعة...'
        },
        en: {
            // Navigation
            'nav.dashboard': 'Dashboard',
            'nav.invoices': 'Invoices',
            'nav.purchase_bills': 'Purchase Bills',
            'nav.products': 'Products',
            'nav.parts_search': 'Parts Search',
            'nav.cars_search': 'Cars Search',
            'nav.suppliers': 'Suppliers',
            'nav.clients': 'Clients',
            'nav.orders': 'Orders',
            'nav.stores': 'Stores',
            'nav.branches': 'Branches',
            'nav.users': 'User Management',
            'nav.settings': 'Settings',
            'nav.logout': 'Logout',
            'nav.cash_vouchers': 'Cash Vouchers',
            'nav.stock_adjustments': 'Stock Adjustments',

            // Settings sidebar
            'settings.dark_mode': 'Dark Mode',
            'settings.language': 'العربية',

            // Common
            'common.loading': 'Loading...',
            'common.save': 'Save',
            'common.cancel': 'Cancel',
            'common.delete': 'Delete',
            'common.edit': 'Edit',
            'common.add': 'Add',
            'common.search': 'Search',
            'common.actions': 'Actions',
            'common.back': 'Back',
            'common.yes': 'Yes',
            'common.no': 'No',
            'common.confirm_delete': 'Are you sure you want to delete?',
            'common.no_results': 'No results found',
            'common.total': 'Total',
            'common.status': 'Status',
            'common.date': 'Date',
            'common.amount': 'Amount',
            'common.name': 'Name',
            'common.phone': 'Phone',
            'common.email': 'Email',
            'common.address': 'Address',
            'common.notes': 'Notes',
            'common.print': 'Print',
            'common.export': 'Export',
            'common.filter': 'Filter',
            'common.clear': 'Clear',
            'common.from': 'From',
            'common.to': 'To',
            'common.all': 'All',

            // Dashboard
            'dashboard.title': 'Dashboard',
            'dashboard.total_revenue': 'Total Revenue',
            'dashboard.total_invoices': 'Total Invoices',
            'dashboard.total_clients': 'Total Clients',
            'dashboard.total_products': 'Total Products',
            'dashboard.recent_invoices': 'Recent Invoices',
            'dashboard.top_clients': 'Top Clients',
            'dashboard.revenue_chart': 'Revenue',
            'dashboard.status_chart': 'Invoice Status',

            // Invoices
            'invoices.title': 'Invoices',
            'invoices.add': 'Add Invoice',
            'invoices.number': 'Invoice Number',
            'invoices.client': 'Client',
            'invoices.date': 'Date',
            'invoices.total': 'Total',
            'invoices.status': 'Status',
            'invoices.paid': 'Paid',
            'invoices.unpaid': 'Unpaid',
            'invoices.partial': 'Partially Paid',

            // Products
            'products.title': 'Products',
            'products.add': 'Add Product',
            'products.name': 'Product Name',
            'products.code': 'Product Code',
            'products.price': 'Price',
            'products.quantity': 'Quantity',
            'products.category': 'Category',

            // Purchase Bills
            'purchase_bills.title': 'Purchase Bills',
            'purchase_bills.add': 'Add Purchase Bill',
            'purchase_bills.supplier': 'Supplier',

            // Suppliers
            'suppliers.title': 'Suppliers',
            'suppliers.add': 'Add Supplier',

            // Clients
            'clients.title': 'Clients',
            'clients.add': 'Add Client',

            // Orders
            'orders.title': 'Orders',
            'orders.add': 'Add Order',

            // Stores
            'stores.title': 'Stores',
            'stores.add': 'Add Store',

            // Branches
            'branches.title': 'Branches',
            'branches.add': 'Add Branch',

            // Users
            'users.title': 'User Management',
            'users.add': 'Add User',
            'users.role': 'Role',
            'users.admin': 'System Admin',
            'users.manager': 'Manager',
            'users.employee': 'Employee',

            // Settings page
            'settings.title': 'Settings',
            'settings.company_name': 'Company Name',
            'settings.company_email': 'Company Email',
            'settings.vat_number': 'VAT Number',
            'settings.phone': 'Phone Number',
            'settings.tax_rate': 'Tax Rate (%)',
            'settings.currency': 'Currency',
            'settings.invoice_footer': 'Invoice Note',
            'settings.appearance': 'Appearance',
            'settings.theme': 'Theme',
            'settings.theme_light': 'Light',
            'settings.theme_dark': 'Dark',
            'settings.theme_auto': 'Auto',
            'settings.language_label': 'Language',
            'settings.lang_ar': 'Arabic',
            'settings.lang_en': 'English',
            'settings.notifications': 'Notifications',
            'settings.invoice_alerts': 'Invoice Alerts',
            'settings.low_stock_alerts': 'Low Stock Alerts',
            'settings.save_success': 'Settings saved successfully',

            // Search
            'search.parts_title': 'Parts Search',
            'search.cars_title': 'Cars Search',
            'search.vin': 'VIN Number',
            'search.query': 'Search Query',
            'search.car_name': 'Car Name or Model',
            'search.year': 'Year',
            'search.make': 'Make',
            'search.model': 'Model',
            'search.category': 'Category',
            'search.condition': 'Condition',
            'search.condition_new': 'New',
            'search.condition_used': 'Used',
            'search.condition_all': 'All',
            'search.price_range': 'Price Range',
            'search.min_price': 'Min Price',
            'search.max_price': 'Max Price',
            'search.search_btn': 'Search',
            'search.reset': 'Reset',
            'search.advanced': 'Advanced Search',
            'search.results': 'Results',
            'search.no_results': 'No matching results',
            'search.quick_make': 'Make',
            'search.quick_filters': 'Quick Filters',
            'search.more_filters': 'More Filters',
            'search.start_searching': 'Start searching for parts',
            'search.search_tip': 'Enter part name or number to search',
            'search.start_car_search': 'Start searching for cars',
            'search.car_search_tip': 'Enter car name or model to search',
            'search.cars_query_placeholder': 'Search for a car...',
            'search.parts_query_placeholder': 'Search for a part...'
        }
    };

    // Get current language from localStorage
    function getLang() {
        return localStorage.getItem('afrita_lang') || 'ar';
    }

    // Set language and apply
    function setLang(lang) {
        localStorage.setItem('afrita_lang', lang);
        applyLang(lang);
    }

    // Translate a key
    function t(key) {
        var lang = getLang();
        return (translations[lang] && translations[lang][key]) || key;
    }

    // Apply language to all elements with data-i18n
    function applyLang(lang) {
        var html = document.documentElement;
        if (lang === 'en') {
            html.setAttribute('dir', 'ltr');
            html.setAttribute('lang', 'en');
            document.body.classList.remove('text-right');
            document.body.classList.add('text-left');
        } else {
            html.setAttribute('dir', 'rtl');
            html.setAttribute('lang', 'ar');
            document.body.classList.remove('text-left');
            document.body.classList.add('text-right');
        }

        var els = document.querySelectorAll('[data-i18n]');
        for (var i = 0; i < els.length; i++) {
            var key = els[i].getAttribute('data-i18n');
            var text = translations[lang] && translations[lang][key];
            if (text) {
                if (els[i].tagName === 'INPUT' || els[i].tagName === 'TEXTAREA') {
                    els[i].setAttribute('placeholder', text);
                } else {
                    els[i].textContent = text;
                }
            }
        }

        // Update lang toggle label
        var langLabel = document.getElementById('lang-label');
        if (langLabel) {
            langLabel.textContent = lang === 'ar' ? 'English' : 'العربية';
        }
    }

    // Toggle language
    window.toggleLang = function() {
        var current = getLang();
        var next = current === 'ar' ? 'en' : 'ar';
        setLang(next);
    };

    // Expose for use in other scripts
    window.i18n = { t: t, getLang: getLang, setLang: setLang, applyLang: applyLang };

    // Apply on load
    document.addEventListener('DOMContentLoaded', function() {
        var lang = getLang();
        if (lang !== 'ar') {
            applyLang(lang);
        }
    });
})();
