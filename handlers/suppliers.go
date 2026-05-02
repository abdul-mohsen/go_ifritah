package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

	"github.com/gorilla/mux"
)

// saudiRegions is the list of Saudi Arabia administrative regions.
var saudiRegions = []string{
	"الرياض",
	"مكة المكرمة",
	"المدينة المنورة",
	"القصيم",
	"المنطقة الشرقية",
	"عسير",
	"تبوك",
	"حائل",
	"الحدود الشمالية",
	"جازان",
	"نجران",
	"الباحة",
	"الجوف",
}

// composeSupplierAddress builds a flat address string from the breakdown form fields.
// Format: "رقم المبنى XXXX، شارع، الحي، المدينة، المنطقة XXXXX، البلد"
func composeSupplierAddress(r *http.Request) string {
	parts := []string{}
	if v := strings.TrimSpace(r.FormValue("building_number")); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(r.FormValue("street_name")); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(r.FormValue("district")); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(r.FormValue("city")); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(r.FormValue("region")); v != "" {
		parts = append(parts, v)
	}
	postal := strings.TrimSpace(r.FormValue("postal_code"))
	additional := strings.TrimSpace(r.FormValue("additional_number"))
	if postal != "" && additional != "" {
		parts = append(parts, postal+"-"+additional)
	} else if postal != "" {
		parts = append(parts, postal)
	} else if additional != "" {
		parts = append(parts, additional)
	}
	if v := strings.TrimSpace(r.FormValue("unit_number")); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(r.FormValue("country")); v != "" {
		parts = append(parts, v)
	}
	if len(parts) > 0 {
		return strings.Join(parts, "، ")
	}
	// Fallback: use the plain address field if no breakdown was provided
	return strings.TrimSpace(r.FormValue("address"))
}

// buildSupplierPayload builds the supplier JSON payload from form fields.
func buildSupplierPayload(r *http.Request) map[string]interface{} {
	address := composeSupplierAddress(r)
	payload := map[string]interface{}{
		"name":                    r.FormValue("name"),
		"address":                 address,
		"short_address":           r.FormValue("short_address"),
		"phone_number":            r.FormValue("phone_number"),
		"number":                  r.FormValue("number"),
		"vat_number":              r.FormValue("vat_number"),
		"commercial_registration": r.FormValue("commercial_registration"),
		"bank_account":            r.FormValue("bank_account"),
	}
	if v := strings.TrimSpace(r.FormValue("email")); v != "" {
		payload["email"] = v
	}
	if v := strings.TrimSpace(r.FormValue("preferred_payment_method")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			payload["preferred_payment_method"] = n
		}
	}
	if v := strings.TrimSpace(r.FormValue("credit_limit")); v != "" {
		payload["credit_limit"] = v
	}
	if v := strings.TrimSpace(r.FormValue("payment_terms_days")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			payload["payment_terms_days"] = n
		}
	}
	return payload
}

// findSupplierByID fetches all suppliers and returns the one matching the given ID.
func findSupplierByID(token string, id string) (models.Supplier, bool) {
	suppliers, err := helpers.FetchSuppliers(token)
	if err != nil {
		return models.Supplier{}, false
	}
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return models.Supplier{}, false
	}
	for _, s := range suppliers {
		if s.ID == idInt {
			return s, true
		}
	}
	return models.Supplier{}, false
}

// HandleSuppliers displays the suppliers list page
func HandleSuppliers(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	suppliers, err := helpers.FetchSuppliers(token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	query := r.URL.Query().Get("q")
	if query != "" {
		filtered := make([]models.Supplier, 0)
		for _, supplier := range suppliers {
			if helpers.ContainsInsensitive(supplier.Name, query) || helpers.ContainsInsensitive(supplier.PhoneNumber, query) {
				filtered = append(filtered, supplier)
			}
		}
		suppliers = filtered
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedSuppliers, pagination := helpers.PaginateSlice(suppliers, page, perPage)
	prevPage := -1
	nextPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "suppliers", map[string]interface{}{
		"title":      "الموردين",
		"suppliers":  pagedSuppliers,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
		"query":      query,
	})
}

// HandleAddSupplier displays the add supplier form
func HandleAddSupplier(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	helpers.Render(w, r, "add-supplier", map[string]interface{}{
		"title":   "إضافة مورد",
		"regions": saudiRegions,
	})
}

// HandleCreateSupplier creates a new supplier
func HandleCreateSupplier(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: "اسم المورد"},
		{Field: "phone_number", Value: r.FormValue("phone_number"), Pattern: helpers.PatternSaudiPhone, Label: "الهاتف", PatternMsg: "رقم جوال سعودي يبدأ بـ 05 ويتكون من 10 أرقام"},
		{Field: "number", Value: r.FormValue("number"), MaxLen: 50, Label: "رقم المورد"},
		{Field: "vat_number", Value: r.FormValue("vat_number"), Pattern: helpers.PatternVATNumber, Label: "الرقم الضريبي", PatternMsg: "الرقم الضريبي يتكون من 15 رقم"},
		{Field: "bank_account", Value: r.FormValue("bank_account"), MaxLen: 30, Label: "الحساب البنكي"},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "phone_number", "number", "vat_number", "commercial_registration", "bank_account",
			"email", "short_address", "building_number", "street_name", "district", "city", "region", "postal_code",
			"additional_number", "unit_number", "country", "preferred_payment_method", "credit_limit", "payment_terms_days"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title":   "إضافة مورد",
			"regions": saudiRegions,
		}, errs, oldValues)
		helpers.Render(w, r, "add-supplier", data)
		return
	}

	payload := buildSupplierPayload(r)
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/supplier", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في إنشاء المورد")
		return
	}

	helpers.APICache.Delete("suppliers")
	helpers.WriteSuccessRedirect(w, "/dashboard/suppliers", "تم إنشاء المورد بنجاح")
}

// HandleSupplierDetail displays supplier details
func HandleSupplierDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	supplier, found := findSupplierByID(token, id)
	if !found {
		supplier = models.Supplier{ID: helpers.ParseIntValue(id), Name: "مورد #" + id}
	}

	helpers.Render(w, r, "supplier-detail", map[string]interface{}{
		"title":    "تفاصيل المورد",
		"supplier": supplier,
	})
}

// HandleEditSupplier displays the edit supplier form
func HandleEditSupplier(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	supplier, found := findSupplierByID(token, id)
	if !found {
		supplier = models.Supplier{ID: helpers.ParseIntValue(id)}
	}

	helpers.Render(w, r, "edit-supplier", map[string]interface{}{
		"title":    "تعديل المورد",
		"supplier": supplier,
		"regions":  saudiRegions,
	})
}

// HandleGetSupplier returns supplier data as JSON
func HandleGetSupplier(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	supplier, found := findSupplierByID(token, id)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "supplier not found"})
		return
	}

	_ = json.NewEncoder(w).Encode(supplier)
}

// HandleUpdateSupplier updates an existing supplier
func HandleUpdateSupplier(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: "اسم المورد"},
		{Field: "phone_number", Value: r.FormValue("phone_number"), Pattern: helpers.PatternSaudiPhone, Label: "الهاتف", PatternMsg: "رقم جوال سعودي يبدأ بـ 05 ويتكون من 10 أرقام"},
		{Field: "number", Value: r.FormValue("number"), MaxLen: 50, Label: "رقم المورد"},
		{Field: "vat_number", Value: r.FormValue("vat_number"), Pattern: helpers.PatternVATNumber, Label: "الرقم الضريبي", PatternMsg: "الرقم الضريبي يتكون من 15 رقم"},
		{Field: "bank_account", Value: r.FormValue("bank_account"), MaxLen: 30, Label: "الحساب البنكي"},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "phone_number", "number", "vat_number", "commercial_registration", "bank_account",
			"email", "short_address", "building_number", "street_name", "district", "city", "region", "postal_code",
			"additional_number", "unit_number", "country", "preferred_payment_method", "credit_limit", "payment_terms_days"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title": "تعديل المورد",
			"supplier": models.Supplier{
				ID: helpers.ParseIntValue(id),
			},
			"regions": saudiRegions,
		}, errs, oldValues)
		helpers.Render(w, r, "edit-supplier", data)
		return
	}

	payload := buildSupplierPayload(r)
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/supplier/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في تحديث المورد")
		return
	}

	// Clear cache so re-fetch hits backend
	helpers.APICache.Delete("suppliers")
	helpers.WriteSuccessRedirect(w, "/dashboard/suppliers", "تم تحديث المورد بنجاح")
}

// HandleDeleteSupplier deletes a supplier
func HandleDeleteSupplier(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/supplier/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("suppliers")
	helpers.WriteSuccessRedirect(w, "/dashboard/suppliers", "تم حذف المورد بنجاح")
}

// HandleSupplierReport displays the supplier report page with purchase bill analytics.
// GET /dashboard/suppliers/{id}/report?from=&to=
func HandleSupplierReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	supplier, found := findSupplierByID(token, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, msgSupplierNotFound)
		return
	}

	// Parse date range — default: last 90 days
	now := time.Now()
	dateFrom := r.URL.Query().Get("from")
	dateTo := r.URL.Query().Get("to")
	if dateFrom == "" {
		dateFrom = now.AddDate(0, 0, -90).Format("2006-01-02")
	}
	if dateTo == "" {
		dateTo = now.Format("2006-01-02")
	}

	supplierID, _ := strconv.Atoi(id)
	report, err := helpers.FetchSupplierReport(token, supplierID, dateFrom, dateTo)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, msgSupplierReportFailed)
		return
	}

	// Compute credit utilization
	if supplier.CreditLimit > 0 {
		report.Summary.CreditUtilPct = math.Round(report.Summary.UnpaidTotal/float64(supplier.CreditLimit)*10000) / 100
		if report.Summary.CreditUtilPct > 100 {
			report.Summary.CreditUtilPct = 100
		}
	}

	helpers.Render(w, r, "supplier-report", map[string]interface{}{
		"title":            fmt.Sprintf("كشف حساب — %s", supplier.Name),
		"supplier":         supplier,
		"summary":          report.Summary,
		"bills":            report.Bills,
		"top_items":        report.TopItems,
		"ledger":           report.Ledger,
		"aging":            report.Aging,
		"payment_methods":  report.PaymentMethods,
		"monthly_spending": report.Monthly,
		"date_from":        dateFrom,
		"date_to":          dateTo,
	})
}

// HandleExportSupplierReportCSV exports the supplier report as CSV.
// GET /dashboard/suppliers/{id}/report/export-csv?from=&to=
func HandleExportSupplierReportCSV(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	_, found := findSupplierByID(token, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, msgSupplierNotFound)
		return
	}

	supplierID, _ := strconv.Atoi(id)

	now := time.Now()
	dateFrom := r.URL.Query().Get("from")
	dateTo := r.URL.Query().Get("to")
	if dateFrom == "" {
		dateFrom = now.AddDate(0, 0, -90).Format("2006-01-02")
	}
	if dateTo == "" {
		dateTo = now.Format("2006-01-02")
	}

	report, err := helpers.FetchSupplierReport(token, supplierID, dateFrom, dateTo)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, msgSupplierReportFailed)
		return
	}

	filename := fmt.Sprintf("supplier_report_%d_%s_%s.csv", supplierID, dateFrom, dateTo)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set(headerContentDisp, "attachment; filename="+filename)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"رقم الفاتورة", "التاريخ", "النوع", "المرجع", "الوصف", "مدين", "دائن", "الرصيد"})
	for _, entry := range report.Ledger {
		_ = writer.Write([]string{
			ledgerBillNo(entry),
			entry.Date,
			ledgerTypeName(entry),
			entry.Reference,
			entry.Description,
			fmt.Sprintf("%.2f", entry.Debit),
			fmt.Sprintf("%.2f", entry.Credit),
			fmt.Sprintf("%.2f", entry.Balance),
		})
	}
}

// ledgerTypeName returns the Arabic display label for a supplier ledger entry.
func ledgerTypeName(entry models.LedgerEntry) string {
	if entry.Type == "payment" {
		return "سند صرف"
	}
	return "فاتورة"
}

// ledgerBillNo picks the most informative identifier for a ledger row,
// falling back to a synthetic CV-/PB- reference when the supplier-supplied
// bill number is missing.
func ledgerBillNo(entry models.LedgerEntry) string {
	if entry.SupplierNo != "" {
		return entry.SupplierNo
	}
	if entry.Reference != "" {
		return entry.Reference
	}
	if entry.SystemID > 0 {
		if entry.Type == "payment" {
			return fmt.Sprintf("CV-%d", entry.SystemID)
		}
		return fmt.Sprintf("PB-%d", entry.SystemID)
	}
	return ""
}

func HandleExportSupplierReportExcel(w http.ResponseWriter, r *http.Request) {
	supplier, report, dateFrom, dateTo, ok := loadSupplierReportForDownload(w, r)
	if !ok {
		return
	}

	filename := fmt.Sprintf("supplier_report_%d_%s_%s.xls", supplier.ID, dateFrom, dateTo)
	w.Header().Set("Content-Type", "application/vnd.ms-excel; charset=utf-8")
	w.Header().Set(headerContentDisp, "attachment; filename="+filename)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writeSupplierReportDocument(w, supplier, report, dateFrom, dateTo, true)
}

func HandleExportSupplierReportPDF(w http.ResponseWriter, r *http.Request) {
	supplier, report, dateFrom, dateTo, ok := loadSupplierReportForDownload(w, r)
	if !ok {
		return
	}

	filename := fmt.Sprintf("supplier_report_%d_%s_%s.html", supplier.ID, dateFrom, dateTo)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set(headerContentDisp, "inline; filename="+filename)
	writeSupplierReportDocument(w, supplier, report, dateFrom, dateTo, false)
}

func loadSupplierReportForDownload(w http.ResponseWriter, r *http.Request) (models.Supplier, helpers.SupplierReportResult, string, string, bool) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return models.Supplier{}, helpers.SupplierReportResult{}, "", "", false
	}

	supplier, found := findSupplierByID(token, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, msgSupplierNotFound)
		return models.Supplier{}, helpers.SupplierReportResult{}, "", "", false
	}

	supplierID, err := strconv.Atoi(id)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "رقم المورد غير صحيح")
		return models.Supplier{}, helpers.SupplierReportResult{}, "", "", false
	}

	dateFrom, dateTo := supplierReportDateRange(r)
	report, err := helpers.FetchSupplierReport(token, supplierID, dateFrom, dateTo)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, msgSupplierReportFailed)
		return models.Supplier{}, helpers.SupplierReportResult{}, "", "", false
	}
	applySupplierCreditUtilization(&report, supplier)

	return supplier, report, dateFrom, dateTo, true
}

func supplierReportDateRange(r *http.Request) (string, string) {
	now := time.Now()
	dateFrom := r.URL.Query().Get("from")
	dateTo := r.URL.Query().Get("to")
	if dateFrom == "" {
		dateFrom = now.AddDate(0, 0, -90).Format("2006-01-02")
	}
	if dateTo == "" {
		dateTo = now.Format("2006-01-02")
	}
	return dateFrom, dateTo
}

func applySupplierCreditUtilization(report *helpers.SupplierReportResult, supplier models.Supplier) {
	if supplier.CreditLimit <= 0 {
		return
	}
	report.Summary.CreditUtilPct = math.Round(report.Summary.UnpaidTotal/float64(supplier.CreditLimit)*10000) / 100
	if report.Summary.CreditUtilPct > 100 {
		report.Summary.CreditUtilPct = 100
	}
}

func writeSupplierReportDocument(w http.ResponseWriter, supplier models.Supplier, report helpers.SupplierReportResult, dateFrom, dateTo string, excel bool) {
	title := fmt.Sprintf("Supplier Account Statement - %s", supplier.Name)
	media := "screen"
	if !excel {
		media = "print"
	}
	fmt.Fprintf(w, `<!doctype html><html lang="ar" dir="rtl"><head><meta charset="utf-8"><title>%s</title><style>
	body{font-family:Arial,Tahoma,sans-serif;color:#111827;margin:24px}h1{font-size:22px;margin:0 0 6px}h2{font-size:16px;margin:24px 0 8px}.meta{color:#4b5563;margin-bottom:18px}.grid{display:grid;grid-template-columns:repeat(4,1fr);gap:8px}.metric{border:1px solid #d1d5db;padding:8px}.label{color:#4b5563;font-size:12px}.value{font-weight:700}table{border-collapse:collapse;width:100%%;margin-bottom:16px}th,td{border:1px solid #d1d5db;padding:6px;text-align:right;vertical-align:top}th{background:#f3f4f6}@media %s{body{margin:12mm}.no-print{display:none}}
	</style></head><body>`, html.EscapeString(title), media)
	if !excel {
		fmt.Fprint(w, `<button class="no-print" onclick="window.print()" style="margin-bottom:16px">Print / Save PDF</button>`)
	}
	fmt.Fprintf(w, `<h1>%s</h1><div class="meta">Supplier: %s | From: %s | To: %s</div>`, html.EscapeString(title), html.EscapeString(supplier.Name), html.EscapeString(dateFrom), html.EscapeString(dateTo))

	fmt.Fprint(w, `<h2>Summary - الملخص</h2><div class="grid">`)
	writeMetric(w, "Closing Balance", report.Summary.ClosingBalance)
	writeMetric(w, "Total Spent", report.Summary.TotalSpent)
	writeMetric(w, "Total Payments", report.Summary.TotalPayments)
	writeMetric(w, "Bill Count", report.Summary.BillCount)
	writeMetric(w, "Overdue Count", report.Summary.OverdueCount)
	writeMetric(w, "Average Bill", report.Summary.AvgBill)
	writeMetric(w, "Total VAT", report.Summary.TotalVAT)
	writeMetric(w, "Payment Count", report.Summary.PaymentCount)
	fmt.Fprint(w, `</div>`)

	fmt.Fprint(w, `<h2>Account Ledger - دفتر الأستاذ</h2><table><thead><tr><th>System ID</th><th>Supplier No</th><th>Date</th><th>Type</th><th>Reference</th><th>Description</th><th>Debit</th><th>Credit</th><th>Balance</th></tr></thead><tbody>`)
	for _, entry := range report.Ledger {
		entryType := "Bill"
		if entry.Type == "payment" {
			entryType = "Payment"
		}
		sysID := ""
		if entry.SystemID > 0 {
			sysID = fmt.Sprintf("%d", entry.SystemID)
		}
		fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			esc(sysID), esc(entry.SupplierNo), esc(entry.Date), esc(entryType), esc(entry.Reference), esc(entry.Description), money(entry.Debit), money(entry.Credit), money(entry.Balance))
	}
	fmt.Fprint(w, `</tbody></table>`)

	fmt.Fprint(w, `<h2>Bills - الفواتير</h2><table><thead><tr><th>System ID</th><th>Supplier No</th><th>Date</th><th>Total</th><th>Before VAT</th><th>VAT</th><th>Discount</th><th>Status</th><th>Due Date</th><th>Items</th></tr></thead><tbody>`)
	for _, bill := range report.Bills {
		fmt.Fprintf(w, `<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td><td>%d</td></tr>`,
			bill.ID, esc(bill.SSN), esc(safeDate(bill.EffectiveDate)), money(bill.Total), money(bill.TotalBeforeVAT), money(bill.TotalVAT), money(bill.Discount), bill.State, esc(safeDate(bill.PaymentDueDate)), bill.ItemCount)
	}
	fmt.Fprint(w, `</tbody></table>`)

	fmt.Fprint(w, `<h2>Monthly Spending - المصروف الشهري</h2><table><thead><tr><th>Month</th><th>Purchases</th><th>Payments</th><th>Net</th></tr></thead><tbody>`)
	for _, month := range report.Monthly {
		fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`, esc(month.Month), money(month.Amount), money(month.Payments), money(month.Amount-month.Payments))
	}
	fmt.Fprint(w, `</tbody></table>`)

	fmt.Fprint(w, `<h2>Payment Methods - طرق الدفع</h2><table><thead><tr><th>Method</th><th>Amount</th><th>Count</th></tr></thead><tbody>`)
	for _, method := range report.PaymentMethods {
		fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%d</td></tr>`, esc(method.Method), money(method.Amount), method.Count)
	}
	fmt.Fprint(w, `</tbody></table>`)

	fmt.Fprint(w, `<h2>Aging - أعمار الديون</h2><table><thead><tr><th>Bucket</th><th>Amount</th><th>Count</th></tr></thead><tbody>`)
	for _, bucket := range report.Aging {
		fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%d</td></tr>`, esc(bucket.Label), money(bucket.Amount), bucket.Count)
	}
	fmt.Fprint(w, `</tbody></table>`)

	fmt.Fprint(w, `<h2>Top Items - أفضل الأصناف</h2><table><thead><tr><th>Item</th><th>Quantity</th><th>Total Value</th><th>Average Price</th><th>Bill Count</th></tr></thead><tbody>`)
	for _, item := range report.TopItems {
		fmt.Fprintf(w, `<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td><td>%d</td></tr>`, esc(item.Name), item.TotalQty, money(item.TotalVal), money(item.AvgPrice), item.BillCount)
	}
	fmt.Fprint(w, `</tbody></table></body></html>`)
}

func writeMetric(w http.ResponseWriter, label string, value interface{}) {
	fmt.Fprintf(w, `<div class="metric"><div class="label">%s</div><div class="value">%s</div></div>`, esc(label), esc(fmt.Sprint(value)))
}

func esc(value string) string {
	return html.EscapeString(value)
}

func money(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

// safeDate extracts YYYY-MM-DD from a date string safely.
func safeDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
