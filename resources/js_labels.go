package resources

// jsLabels holds the small set of bilingual strings consumed by client-side
// JavaScript (static/js/script.js, static/js/smart-search.js) rather than by
// server-rendered templates. These are kept separate from the much larger
// `labels` map in labels.go so that only this curated subset is ever
// serialized into the per-page `window.I18N` blob (see JSLabels below) —
// shipping the full template label map to the browser on every page load
// would be wasteful and is unnecessary since templates render server-side.
var jsLabels = map[string]Label{
	// Generic AJAX / request error messages (script.js)
	"generic_error":       {Ar: "حدث خطأ، يرجى المحاولة مرة أخرى", En: "An error occurred, please try again"},
	"pdf_load_failed":     {Ar: "تعذر تحميل ملف PDF، يرجى المحاولة لاحقاً", En: "Failed to load the PDF file, please try again later"},
	"server_unreachable":  {Ar: "تعذر الاتصال بالخادم، يرجى المحاولة لاحقاً", En: "Could not reach the server, please try again later"},
	"network_check_error": {Ar: "تعذر الاتصال بالخادم، يرجى التحقق من الاتصال والمحاولة لاحقاً", En: "Could not reach the server, please check your connection and try again"},
	"request_failed":      {Ar: "فشل إرسال الطلب، يرجى التحقق من اتصالك بالإنترنت", En: "Failed to send the request, please check your internet connection"},
	"confirm_delete":      {Ar: "هل أنت متأكد من الحذف؟", En: "Are you sure you want to delete this?"},
	"delete_success":      {Ar: "تم الحذف بنجاح", En: "Deleted successfully"},
	"http_400":            {Ar: "البيانات المرسلة غير صحيحة", En: "The submitted data is invalid"},
	"http_401":            {Ar: "انتهت صلاحية الجلسة، يرجى تسجيل الدخول مجدداً", En: "Your session has expired, please sign in again"},
	"http_403":            {Ar: "ليس لديك صلاحية لتنفيذ هذا الإجراء", En: "You do not have permission to perform this action"},
	"http_404":            {Ar: "العنصر المطلوب غير موجود", En: "The requested item was not found"},
	"http_409":            {Ar: "يوجد تعارض في البيانات، يرجى المحاولة مجدداً", En: "There is a data conflict, please try again"},
	"http_422":            {Ar: "البيانات المرسلة غير مكتملة", En: "The submitted data is incomplete"},
	"http_429":            {Ar: "طلبات كثيرة جداً، يرجى الانتظار قليلاً", En: "Too many requests, please wait a moment"},
	"http_500":            {Ar: "حدث خطأ في الخادم، يرجى المحاولة لاحقاً", En: "A server error occurred, please try again later"},
	"http_502":            {Ar: "الخادم غير متاح حالياً", En: "The server is currently unavailable"},
	"http_503":            {Ar: "الخدمة غير متاحة مؤقتاً، يرجى المحاولة لاحقاً", En: "The service is temporarily unavailable, please try again later"},

	// Smart-search widget (smart-search.js): generic UI chrome
	"remove":                {Ar: "إزالة", En: "Remove"},
	"clear":                 {Ar: "مسح", En: "Clear"},
	"add_filter_button":     {Ar: "+ فلتر", En: "+ Filter"},
	"add_filter_aria":       {Ar: "إضافة فلتر", En: "Add filter"},
	"search_specific_field": {Ar: "بحث في حقل محدد", En: "Search a specific field"},
	"all_fields_added":      {Ar: "كل الحقول مضافة", En: "All fields already added"},
	"recent_searches":       {Ar: "عمليات بحث سابقة", En: "Recent searches"},
	"type_value_hint":       {Ar: "اكتب القيمة (البحث مباشر)", En: "Type a value (live search)"},
	"search_by_hint":        {Ar: "بحث بـ", En: "Search by"},
	"count_of":              {Ar: "من", En: "of"},

	// Smart-search widget: per-resource field labels for the "+ Filter" menu
	"field_invoice_number":          {Ar: "رقم الفاتورة", En: "Invoice Number"},
	"field_phone":                   {Ar: "رقم الهاتف", En: "Phone Number"},
	"field_vin":                     {Ar: "رقم الهيكل", En: "VIN"},
	"field_supplier_invoice_number": {Ar: "رقم فاتورة المورد", En: "Supplier Invoice Number"},
	"field_part_number":             {Ar: "رقم القطعة", En: "Part Number"},
	"field_barcode":                 {Ar: "الباركود", En: "Barcode"},
	"field_vat_number":              {Ar: "الرقم الضريبي", En: "VAT Number"},
	"field_commercial_registration": {Ar: "السجل التجاري", En: "Commercial Registration"},
	"field_order_number":            {Ar: "رقم الطلب", En: "Order Number"},
	"field_voucher_number":          {Ar: "رقم السند", En: "Voucher Number"},
	"field_email":                   {Ar: "البريد الإلكتروني", En: "Email"},

	// Settings page: ZATCA connection status pills (settings.html)
	"zatca_status_deleted":       {Ar: "محذوف", En: "Deleted"},
	"zatca_status_connected":     {Ar: "متصل ✓", En: "Connected ✓"},
	"zatca_status_expired":       {Ar: "منتهي ⚠", En: "Expired ⚠"},
	"zatca_status_disconnected":  {Ar: "غير متصل", En: "Not Connected"},
	"zatca_status_expiring_soon": {Ar: "ينتهي قريباً ⏰", En: "Expiring Soon ⏰"},
	"otp_invalid_length":         {Ar: "OTP يجب أن يكون 6 أرقام", En: "OTP must be 6 digits"},
	"zatca_link_failed":          {Ar: "فشل الربط", En: "Linking failed"},
	"server_connection_error":    {Ar: "خطأ في الاتصال بالخادم", En: "Error connecting to the server"},

	// Settings page: Data Import wizard (settings.html) — column-mapping
	// field labels and status/progress text.
	"field_part_name":            {Ar: "اسم القطعة", En: "Part Name"},
	"field_price":                {Ar: "السعر", En: "Price"},
	"field_quantity":             {Ar: "الكمية", En: "Quantity"},
	"field_discount":             {Ar: "الخصم", En: "Discount"},
	"field_customer_name":        {Ar: "اسم العميل", En: "Customer Name"},
	"field_chassis_number":       {Ar: "رقم الشاصي", En: "Chassis Number"},
	"field_note":                 {Ar: "ملاحظة", En: "Note"},
	"field_date":                 {Ar: "التاريخ", En: "Date"},
	"field_bill_group":           {Ar: "رقم الفاتورة (تجميع)", En: "Invoice Number (grouping)"},
	"field_supplier_number":      {Ar: "رقم المورد", En: "Supplier Number"},
	"field_sell_price":           {Ar: "سعر البيع", En: "Sell Price"},
	"field_cost_price":           {Ar: "سعر التكلفة", En: "Cost Price"},
	"field_shelf_number":         {Ar: "رقم الرف", En: "Shelf Number"},
	"select_file_prompt":         {Ar: "يرجى اختيار ملف", En: "Please select a file"},
	"analyzing_ellipsis":         {Ar: "جاري التحليل...", En: "Analyzing..."},
	"preview_label":              {Ar: "معاينة", En: "Preview"},
	"analyze_failed_prefix":      {Ar: "فشل في تحليل الملف: ", En: "Failed to analyze the file: "},
	"skip_option":                {Ar: "— تخطي —", En: "— Skip —"},
	"map_required_fields_prefix": {Ar: "يرجى تعيين الحقول المطلوبة: ", En: "Please map the required fields: "},
	"list_separator":             {Ar: "، ", En: ", "},
	"importing_ellipsis":         {Ar: "جاري الاستيراد...", En: "Importing..."},
	"start_import_label":         {Ar: "بدء الاستيراد", En: "Start Import"},
	"import_failed_prefix":       {Ar: "فشل في الاستيراد: ", En: "Failed to import: "},
	"summary_total":              {Ar: "الإجمالي", En: "Total"},
	"status_success":             {Ar: "نجح", En: "Success"},
	"status_failed":              {Ar: "فشل", En: "Failed"},
	"table_header_row":           {Ar: "الصف", En: "Row"},
	"table_header_status":        {Ar: "الحالة", En: "Status"},
	"table_header_details":       {Ar: "التفاصيل", En: "Details"},
	"import_success_message":     {Ar: "تمت العملية بنجاح", En: "Operation completed successfully"},
}

// JSLabels returns the jsLabels map translated into the requested language,
// ready to be JSON-marshaled into a page's `window.I18N` blob (see the
// "jsI18N" template function bound in config.BindLang). Client scripts read
// window.I18N.<key> instead of hardcoding Arabic text, mirroring how server
// templates use the "L" function with the labels.go map.
func JSLabels(lang string) map[string]string {
	out := make(map[string]string, len(jsLabels))
	for k, v := range jsLabels {
		if lang == LangEn {
			out[k] = v.En
		} else {
			out[k] = v.Ar
		}
	}
	return out
}
