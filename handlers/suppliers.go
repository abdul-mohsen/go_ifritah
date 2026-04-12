package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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
