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

// HandleClients displays the clients list page
func HandleClients(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	clients, err := helpers.FetchClients(token)
	if err != nil {
		clients = []models.Client{}
	}

	query := r.URL.Query().Get("q")
	if query != "" {
		filtered := make([]models.Client, 0)
		for _, c := range clients {
			if helpers.ContainsInsensitive(c.Name, query) ||
				helpers.ContainsInsensitive(c.Email, query) ||
				helpers.ContainsInsensitive(c.Phone, query) {
				filtered = append(filtered, c)
			}
		}
		clients = filtered
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedClients, pagination := helpers.PaginateSlice(clients, page, perPage)
	prevPage := 0
	nextPage := 0
	if pagination.Page > 1 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "clients", map[string]interface{}{
		"title":      "العملاء",
		"clients":    pagedClients,
		"query":      query,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
	})
}

// HandleAddClient displays the add client form
func HandleAddClient(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	helpers.Render(w, r, "add-client", map[string]interface{}{
		"title":   "إضافة عميل",
		"regions": saudiRegions,
	})
}

// composeClientAddress builds a structured address string from form fields.
func composeClientAddress(r *http.Request) string {
	parts := []string{}
	for _, f := range []string{"building_number", "street_name", "district", "city", "region"} {
		if v := strings.TrimSpace(r.FormValue(f)); v != "" {
			parts = append(parts, v)
		}
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
	return strings.TrimSpace(r.FormValue("address"))
}

// buildClientPayload builds the client JSON payload from form fields.
func buildClientPayload(r *http.Request) map[string]interface{} {
	address := composeClientAddress(r)
	payload := map[string]interface{}{
		"name":                    r.FormValue("name"),
		"number":                  r.FormValue("number"),
		"company_name":            r.FormValue("company_name"),
		"phone":                   r.FormValue("phone"),
		"address":                 address,
		"short_address":           r.FormValue("short_address"),
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

// HandleClientDetail displays client details
func HandleClientDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	client, err := helpers.FetchClientByID(token, id)
	if err != nil {
		client = models.Client{ID: id, Name: "عميل #" + id}
	}

	helpers.Render(w, r, "client-detail", map[string]interface{}{
		"title":  "تفاصيل العميل",
		"client": client,
	})
}

// HandleEditClient displays the edit client form
func HandleEditClient(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	client, err := helpers.FetchClientByID(token, id)
	if err != nil {
		client = models.Client{ID: id}
	}

	helpers.Render(w, r, "edit-client", map[string]interface{}{
		"title":   "تعديل العميل",
		"client":  client,
		"regions": saudiRegions,
	})
}

// HandleCreateClient creates a new client
func HandleCreateClient(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: "اسم العميل"},
		{Field: "company_name", Value: r.FormValue("company_name"), Required: true, MinLen: 2, MaxLen: 200, Label: "اسم الشركة"},
		{Field: "email", Value: r.FormValue("email"), Required: true, MaxLen: 254, Email: true, Label: "البريد الإلكتروني"},
		{Field: "phone", Value: r.FormValue("phone"), Required: true, Pattern: helpers.PatternSaudiPhone, Label: "الهاتف", PatternMsg: "رقم جوال سعودي يبدأ بـ 05 ويتكون من 10 أرقام"},
		{Field: "vat_number", Value: r.FormValue("vat_number"), Required: true, Pattern: helpers.PatternVATNumber, Label: "الرقم الضريبي", PatternMsg: "الرقم الضريبي يتكون من 15 رقم"},
		{Field: "bank_account", Value: r.FormValue("bank_account"), MaxLen: 30, Label: "الحساب البنكي"},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "number", "company_name", "email", "phone", "address", "short_address", "vat_number", "commercial_registration", "bank_account",
			"building_number", "street_name", "district", "city", "region", "postal_code", "additional_number", "unit_number", "country",
			"preferred_payment_method", "credit_limit", "payment_terms_days"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title":   "إضافة عميل",
			"regions": saudiRegions,
		}, errs, oldValues)
		helpers.Render(w, r, "add-client", data)
		return
	}

	payload := buildClientPayload(r)
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/client", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusBadRequest {
			helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "هذا العميل موجود بالفعل أو البيانات غير صحيحة")
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "حدث خطأ في إنشاء العميل")
		return
	}

	helpers.APICache.Delete("clients")
	helpers.WriteSuccessRedirect(w, "/dashboard/clients", "تم إنشاء العميل بنجاح")
}

// HandleUpdateClient updates an existing client
func HandleUpdateClient(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: "اسم العميل"},
		{Field: "company_name", Value: r.FormValue("company_name"), Required: true, MinLen: 2, MaxLen: 200, Label: "اسم الشركة"},
		{Field: "phone", Value: r.FormValue("phone"), Required: true, Pattern: helpers.PatternSaudiPhone, Label: "رقم الهاتف", PatternMsg: "رقم جوال سعودي يبدأ بـ 05 ويتكون من 10 أرقام"},
		{Field: "email", Value: r.FormValue("email"), Required: true, MaxLen: 254, Email: true, Label: "البريد الإلكتروني"},
		{Field: "vat_number", Value: r.FormValue("vat_number"), Required: true, Pattern: helpers.PatternVATNumber, Label: "الرقم الضريبي", PatternMsg: "الرقم الضريبي يتكون من 15 رقم"},
		{Field: "bank_account", Value: r.FormValue("bank_account"), MaxLen: 30, Label: "الحساب البنكي"},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "number", "company_name", "phone", "address", "short_address", "email", "vat_number", "commercial_registration", "bank_account",
			"building_number", "street_name", "district", "city", "region", "postal_code", "additional_number", "unit_number", "country",
			"preferred_payment_method", "credit_limit", "payment_terms_days"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title":   "تعديل العميل",
			"regions": saudiRegions,
			"client": models.Client{
				ID: id,
			},
		}, errs, oldValues)
		helpers.Render(w, r, "edit-client", data)
		return
	}

	payload := buildClientPayload(r)
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/client/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "حدث خطأ في تحديث العميل")
		return
	}

	helpers.APICache.Delete("clients")
	helpers.WriteSuccessRedirect(w, "/dashboard/clients", "تم تحديث العميل بنجاح")
}

// HandleDeleteClient deletes a client
func HandleDeleteClient(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/client/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في حذف العميل")
		return
	}

	helpers.APICache.Delete("clients")
	helpers.WriteSuccessRedirect(w, "/dashboard/clients", "تم حذف العميل بنجاح")
}
