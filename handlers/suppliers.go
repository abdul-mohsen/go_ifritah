package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
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
		"name":         r.FormValue("name"),
		"address":      address,
		"short_address": r.FormValue("short_address"),
		"phone_number": r.FormValue("phone_number"),
		"number":       r.FormValue("number"),
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
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, "المورد غير موجود")
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
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحميل تقرير المورد")
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

	supplier, found := findSupplierByID(token, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, "المورد غير موجود")
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
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحميل تقرير المورد")
		return
	}

	filename := fmt.Sprintf("supplier_report_%d_%s_%s.csv", supplierID, dateFrom, dateTo)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header info
	_ = writer.Write([]string{"كشف حساب المورد", supplier.Name})
	_ = writer.Write([]string{"الفترة", dateFrom + " — " + dateTo})
	_ = writer.Write([]string{"إجمالي المشتريات", fmt.Sprintf("%.2f", report.Summary.TotalSpent)})
	_ = writer.Write([]string{"إجمالي المدفوعات", fmt.Sprintf("%.2f", report.Summary.TotalPayments)})
	_ = writer.Write([]string{"الرصيد الختامي", fmt.Sprintf("%.2f", report.Summary.ClosingBalance)})
	_ = writer.Write([]string{"غير مسدد", fmt.Sprintf("%.2f", report.Summary.UnpaidTotal)})
	_ = writer.Write([]string{"عدد الفواتير", fmt.Sprintf("%d", report.Summary.BillCount)})
	_ = writer.Write([]string{""})

	// Account statement (ledger)
	_ = writer.Write([]string{"كشف الحساب"})
	_ = writer.Write([]string{"التاريخ", "النوع", "المرجع", "الوصف", "مدين", "دائن", "الرصيد"})
	for _, entry := range report.Ledger {
		typeName := "فاتورة"
		if entry.Type == "payment" {
			typeName = "سند صرف"
		}
		_ = writer.Write([]string{
			entry.Date,
			typeName,
			entry.Reference,
			entry.Description,
			fmt.Sprintf("%.2f", entry.Debit),
			fmt.Sprintf("%.2f", entry.Credit),
			fmt.Sprintf("%.2f", entry.Balance),
		})
	}
	_ = writer.Write([]string{""})

	// Bills detail
	_ = writer.Write([]string{"تفاصيل الفواتير"})
	_ = writer.Write([]string{"رقم الفاتورة", "رقم المورد", "التاريخ", "الإجمالي", "قبل الضريبة", "ض.ق.م", "الخصم", "الحالة", "تاريخ الاستحقاق", "متأخر", "عدد الأصناف"})
	for _, b := range report.Bills {
		state := "مسودة"
		if b.State >= 1 {
			state = "مسدد"
		}
		overdue := ""
		if b.IsOverdue {
			overdue = fmt.Sprintf("%d يوم", b.DaysOverdue)
		}
		_ = writer.Write([]string{
			fmt.Sprintf("%d", b.SequenceNumber),
			b.SSN,
			safeDate(b.EffectiveDate),
			fmt.Sprintf("%.2f", b.Total),
			fmt.Sprintf("%.2f", b.TotalBeforeVAT),
			fmt.Sprintf("%.2f", b.TotalVAT),
			fmt.Sprintf("%.2f", b.Discount),
			state,
			b.PaymentDueDate,
			overdue,
			fmt.Sprintf("%d", b.ItemCount),
		})
	}
}

// safeDate extracts YYYY-MM-DD from a date string safely.
func safeDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
