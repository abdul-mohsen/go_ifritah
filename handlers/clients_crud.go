package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
	"afrita/resources"

	"github.com/gorilla/mux"
)

// HandleClients displays the clients list page
func HandleClients(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	query := r.URL.Query().Get("q")
	typed := helpers.TypedListFilters("clients", r.URL.Query())

	clients, err := helpers.FetchClientsList(token, helpers.ListOpts{
		Page:    0,
		PerPage: 10000,
		Query:   query,
		Typed:   typed,
	})
	backendErr := ""
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		log.Printf("[clients] backend list fetch failed: %v", err)
		clients = []models.Client{}
		backendErr = resources.T(lang, "client.load_error_currently")
	}

	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedClients, pagination := helpers.PaginateSlice(clients, page, perPage)
	prevPage := -1
	nextPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "clients", map[string]interface{}{
		"title":      resources.T(lang, "client.list_title"),
		"clients":    pagedClients,
		"query":      query,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
		"error":      backendErr,
	})
}

// HandleAddClient displays the add client form
func HandleAddClient(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	lang := helpers.GetLang(r)
	helpers.Render(w, r, "add-client", map[string]interface{}{
		"title":   resources.T(lang, "client.add_title"),
		"regions": localizedSaudiRegions(lang),
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
		lang := helpers.GetLang(r)
		return strings.Join(parts, resources.T(lang, "ui.list_separator"))
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
	lang := helpers.GetLang(r)

	client, err := helpers.FetchClientByID(token, id)
	if err != nil {
		client = models.Client{ID: id, Name: resources.T(lang, "client.fallback_name") + id}
	}

	helpers.Render(w, r, "client-detail", map[string]interface{}{
		"title":  resources.T(lang, "client.detail_title"),
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
	lang := helpers.GetLang(r)

	client, err := helpers.FetchClientByID(token, id)
	if err != nil {
		client = models.Client{ID: id}
	}

	helpers.Render(w, r, "edit-client", map[string]interface{}{
		"title":   resources.T(lang, "client.edit_title"),
		"client":  client,
		"regions": localizedSaudiRegions(lang),
	})
}

// HandleCreateClient creates a new client
func HandleCreateClient(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: resources.T(lang, "client.label.name")},
		{Field: "company_name", Value: r.FormValue("company_name"), Required: true, MinLen: 2, MaxLen: 200, Label: resources.T(lang, "client.label.company")},
		{Field: "email", Value: r.FormValue("email"), Required: true, MaxLen: 254, Email: true, Label: resources.T(lang, "client.label.email")},
		{Field: "phone", Value: r.FormValue("phone"), Required: true, Pattern: helpers.PatternSaudiPhone, Label: resources.T(lang, "client.label.phone"), PatternMsg: resources.T(lang, "validation.saudi_phone_detailed")},
		{Field: "vat_number", Value: r.FormValue("vat_number"), Required: true, Pattern: helpers.PatternVATNumber, Label: resources.T(lang, "client.label.vat"), PatternMsg: resources.T(lang, "validation.vat_number")},
		{Field: "bank_account", Value: r.FormValue("bank_account"), MaxLen: 30, Label: resources.T(lang, "supplier.label.bank_account")},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "number", "company_name", "email", "phone", "address", "short_address", "vat_number", "commercial_registration", "bank_account",
			"building_number", "street_name", "district", "city", "region", "postal_code", "additional_number", "unit_number", "country",
			"preferred_payment_method", "credit_limit", "payment_terms_days"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title":   resources.T(lang, "client.add_title"),
			"regions": localizedSaudiRegions(lang),
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
			helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.T(lang, "client.duplicate_or_invalid"))
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, resources.T(lang, "client.create_error"))
		return
	}

	helpers.APICache.Delete("clients")
	helpers.WriteSuccessRedirect(w, "/dashboard/clients", resources.T(lang, "client.create_success"))
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
	lang := helpers.GetLang(r)

	// Server-side validation
	errs := helpers.Validate([]helpers.FieldRule{
		{Field: "name", Value: r.FormValue("name"), Required: true, MinLen: 2, MaxLen: 100, Label: resources.T(lang, "client.label.name")},
		{Field: "company_name", Value: r.FormValue("company_name"), Required: true, MinLen: 2, MaxLen: 200, Label: resources.T(lang, "client.label.company")},
		{Field: "phone", Value: r.FormValue("phone"), Required: true, Pattern: helpers.PatternSaudiPhone, Label: resources.T(lang, "client.label.phone_number"), PatternMsg: resources.T(lang, "validation.saudi_phone_detailed")},
		{Field: "email", Value: r.FormValue("email"), Required: true, MaxLen: 254, Email: true, Label: resources.T(lang, "client.label.email")},
		{Field: "vat_number", Value: r.FormValue("vat_number"), Required: true, Pattern: helpers.PatternVATNumber, Label: resources.T(lang, "client.label.vat"), PatternMsg: resources.T(lang, "validation.vat_number")},
		{Field: "bank_account", Value: r.FormValue("bank_account"), MaxLen: 30, Label: resources.T(lang, "supplier.label.bank_account")},
	})
	if errs != nil {
		oldValues := helpers.OldValues([]string{"name", "number", "company_name", "phone", "address", "short_address", "email", "vat_number", "commercial_registration", "bank_account",
			"building_number", "street_name", "district", "city", "region", "postal_code", "additional_number", "unit_number", "country",
			"preferred_payment_method", "credit_limit", "payment_terms_days"}, r.FormValue)
		data := helpers.RenderFormWithErrors(map[string]interface{}{
			"title":   resources.T(lang, "client.edit_title"),
			"regions": localizedSaudiRegions(lang),
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
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, resources.T(lang, "client.update_error"))
		return
	}

	helpers.APICache.Delete("clients")
	helpers.WriteSuccessRedirect(w, "/dashboard/clients", resources.T(lang, "client.update_success"))
}

// HandleDeleteClient deletes a client
func HandleDeleteClient(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/client/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, resources.T(lang, "client.delete_error"))
		return
	}

	helpers.APICache.Delete("clients")
	helpers.WriteSuccessRedirect(w, "/dashboard/clients", resources.T(lang, "client.delete_success"))
}
