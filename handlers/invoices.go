package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
	"afrita/resources"

	"github.com/gorilla/mux"
)

// Note: These handlers depend on functions from api_helpers.go, form_helpers.go, string_helpers.go, dashboard_helpers.go
// These will need to be imported or moved to a helpers package

// HandleInvoices displays the invoices list page with pagination
func HandleInvoices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	// Read pagination and filter parameters
	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	if page < 0 {
		page = 0
	}
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	if perPage < 1 {
		perPage = 10
	}
	stateFilter := r.URL.Query().Get("state")
	query := r.URL.Query().Get("q")
	typed := helpers.TypedListFilters("invoices", r.URL.Query())

	// Search + state filter are backend-driven. Sort is FE-only (BE returns
	// rows in canonical keyset order); see static/js/script.js sortable
	// table init for the client-side comparator.
	invoices, err := helpers.FetchInvoicesAllWithTyped(token, page, query, stateFilter, typed)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		// Soft-fail: render the page with an empty list and a banner so the
		// user lands on a usable page (and the test selector for
		// `/dashboard/invoices/add-invoice` still resolves) instead of seeing
		// a generic 500 stub when the upstream bill list is hiccuping.
		log.Printf("[invoices] backend list fetch failed: %v", err)
		helpers.Render(w, r, "invoices", map[string]interface{}{
			"title":      resources.T(lang, "invoice.list_title"),
			"invoices":   []map[string]interface{}{},
			"pagination": helpers.Pagination{Page: 0, PerPage: 10, Total: 0, TotalPages: 0},
			"prev_page":  -1,
			"next_page":  -1,
			"query":      query,
			"state":      stateFilter,
			"error":      resources.T(lang, "invoice.load_error_currently"),
		})
		return
	}
	log.Printf("Fetched %d invoices from backend (page %d)", len(invoices), page)

	// Transform to display format
	displayInvoices := make([]map[string]interface{}, 0, len(invoices))
	for _, inv := range invoices {
		totalDisplay := fmt.Sprintf("%.2f", inv.Total)
		status, statusClass := helpers.InvoiceStatus(inv)
		status = helpers.TranslateInvoiceStatus(status)
		invoiceType := helpers.InvoiceTypeLabel(inv)
		if inv.State == 0 {
			invoiceType = fmt.Sprintf("%s %s", invoiceType, resources.T(lang, "invoice.draft_suffix"))
		}

		displayInvoices = append(displayInvoices, map[string]interface{}{
			"id":              inv.ID,
			"sequence_number": inv.SequenceNumber,
			"total":           totalDisplay,
			"subtotal":        fmt.Sprintf("%.2f", inv.TotalBeforeVAT),
			"vat":             fmt.Sprintf("%.2f", inv.TotalVAT),
			"discount":        fmt.Sprintf("%.2f", inv.Discount),
			"date":            helpers.ToDisplayDate(inv.EffectiveDate.Time),
			"type":            invoiceType,
			"status":          status,
			"status_class":    statusClass,
			"state":           inv.State,
			"credit_state":    inv.CreditState,
			"is_credit":       inv.CreditState > 0,
		})
	}

	// Paginate client-side
	pagedInvoices, pagination := helpers.PaginateSlice(displayInvoices, page, perPage)

	// Add order numbers
	offset := page * perPage
	for i := range pagedInvoices {
		pagedInvoices[i]["order"] = offset + i + 1
	}

	prevPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	nextPage := -1
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	data := map[string]interface{}{
		"title":      resources.T(lang, "invoice.list_title"),
		"invoices":   pagedInvoices,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
		"query":      query,
		"state":      stateFilter,
	}

	// Fetch clients and stores for the company bill modal
	clients, _ := helpers.FetchClients(token)
	stores, _ := helpers.FetchStores(token)
	data["Clients"] = clients
	data["Stores"] = stores

	helpers.Render(w, r, "invoices", data)
}

// HandleAddInvoice displays the add invoice form
func HandleAddInvoice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	stores, _ := helpers.FetchStores(token)
	branches, _ := helpers.FetchBranches(token)
	today := time.Now().Format("2006-01-02")

	isCompany := r.URL.Query().Get("type") == "company"

	data := map[string]interface{}{
		"title":      resources.T(lang, "invoice.add_title"),
		"stores":     stores,
		"branches":   branches,
		"today":      today,
		"is_company": isCompany,
	}

	if isCompany {
		clients, _ := helpers.FetchClients(token)
		data["clients"] = clients
	}

	helpers.Render(w, r, "add-invoice", data)
}

// HandleCreateDraftInvoice creates a new draft invoice
func HandleCreateDraftInvoice(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	stores, err := helpers.FetchStores(token)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	if len(stores) == 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.T(lang, "invoice.no_stores"))
		return
	}

	payload := models.BillPayload{
		StoreID:  stores[0].ID,
		Products: []models.BillProductItem{},
		ManualProducts: []models.BillManualItem{
			{
				PartName: "Draft Item",
				Price:    "1",
				Quantity: "1",
			},
		},
		TotalAmount:     1,
		Discount:        "0",
		MaintenanceCost: "0",
		State:           0,
		UserName:        "Draft",
		UserPhoneNumber: "",
		Note:            "Auto draft created from invoices page",
	}

	jsonPayload, _ := json.Marshal(payload)
	log.Printf("Draft invoice payload: %s", string(jsonPayload))
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("Draft invoice creation failed: status %d", resp.StatusCode)
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, resources.T(lang, "invoice.draft_create_error"))
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", resources.T(lang, "invoice.draft_create_success"))
}

// HandleAddCreditNote displays the add credit note form
func HandleAddCreditNote(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	lang := helpers.GetLang(r)

	vars := mux.Vars(r)
	id := vars["id"]
	data := map[string]interface{}{
		"title": resources.T(lang, "invoice.add_credit_note_title"),
		"id":    id,
	}
	helpers.Render(w, r, "add-credit-note", data)
}

// HandleCreateInvoice creates a new invoice
func HandleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	payload := helpers.BuildBillPayload(r)
	jsonPayload, _ := json.Marshal(payload)

	log.Printf("[CREATE INVOICE] Payload: %s", string(jsonPayload))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[CREATE INVOICE] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, resources.T(lang, "invoice.create_error"))
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", resources.T(lang, "invoice.create_success"))
}

// HandleCreateCreditNote creates a new credit note
func HandleCreateCreditNote(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	if err := r.ParseForm(); err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.T(lang, "invoice.invalid_form"))
		return
	}

	billID := helpers.ParseIntValue(r.FormValue("bill_id"))
	note := r.FormValue("note")

	payload, _ := json.Marshal(map[string]interface{}{
		"bill_id": billID,
		"note":    note,
	})

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill/credit", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "")
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", resources.T(lang, "invoice.credit_note_create_success"))
}

// HandleGetInvoice displays invoice detail page
func HandleGetInvoice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)
	loadSettingsFromBackend(token)

	// Fetch the full raw bill data from backend
	raw, err := helpers.FetchBillRaw(token, id)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	// Also parse into Invoice for status/type helpers
	invoice, products, manualProducts, extra, _ := helpers.ParseBillRaw(raw, id)

	status, statusClass := helpers.InvoiceStatus(invoice)
	invoiceType := helpers.InvoiceTypeLabel(invoice)
	if invoice.State == 0 {
		invoiceType = fmt.Sprintf("%s %s", invoiceType, resources.T(lang, "invoice.draft_suffix"))
	}

	// Format dates
	effectiveDate := ""
	if invoice.EffectiveDate.Valid && invoice.EffectiveDate.Time != "" {
		if len(invoice.EffectiveDate.Time) >= 10 {
			effectiveDate = invoice.EffectiveDate.Time[:10]
		}
	}
	paymentDueDate := ""
	if v, ok := extra["payment_due_date"].(string); ok && len(v) >= 10 {
		paymentDueDate = v[:10]
	}

	// Maintenance cost
	maintenanceCost := 0.0
	if v, ok := helpers.CoerceFloat(extra["maintenance_cost"]); ok {
		maintenanceCost = v
	}

	// Resolve branch name from branch_id
	branchName := ""
	if branchIDVal, ok := extra["branch_id"]; ok {
		if bid, ok := helpers.CoerceFloat(branchIDVal); ok && bid > 0 {
			branches, _ := helpers.FetchBranches(token)
			for _, b := range branches {
				if b.ID == int(bid) {
					branchName = b.Name
					break
				}
			}
		}
	}

	data := map[string]interface{}{
		"title":            resources.T(lang, "invoice.detail_title"),
		"invoice":          invoice,
		"invoice_id":       id,
		"total_display":    fmt.Sprintf("%.2f", invoice.Total),
		"status":           status,
		"status_class":     statusClass,
		"type":             invoiceType,
		"products":         products,
		"manual_products":  manualProducts,
		"effective_date":   effectiveDate,
		"payment_due_date": paymentDueDate,
		"store_name":       helpers.SafeString(extra["store_name"]),
		"branch_name":      branchName,
		"company_name":     helpers.SafeString(extra["company_name"]),
		"address":          helpers.SafeString(extra["address"]),
		"vat_registration": helpers.SafeString(extra["vat_registration"]),
		"commercial_reg":   helpers.SafeString(extra["CommercialRegistrationNumber"]),
		"user_name":        helpers.SafeString(extra["user_name"]),
		"user_phone":       helpers.SafeString(extra["user_phone_number"]),
		"note":             helpers.SafeString(extra["note"]),
		"maintenance_cost": maintenanceCost,
		"url":              helpers.SafeString(extra["url"]),
		"is_draft":         invoice.State == 0,
		"is_credit":        false,
		"is_standard":      invoice.Type,
		"bill_type_label":  helpers.InvoiceTypeLabel(invoice),
		"whatsapp_enabled": GetSettingValue(token, "whatsapp_enabled") == "true",
	}
	helpers.Render(w, r, "invoice-detail", data)
}

// HandleSendInvoiceWhatsApp proxies invoice PDF WhatsApp sending to the backend.
func HandleSendInvoiceWhatsApp(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	id := mux.Vars(r)["id"]
	if strings.TrimSpace(id) == "" {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.T(lang, "invoice.invalid_number"))
		return
	}

	req, err := http.NewRequest(http.MethodPost, config.BackendDomain+"/api/v2/bill/"+id+"/whatsapp", nil)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, resources.T(lang, "invoice.whatsapp_send_error"))
		return
	}

	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		log.Printf("[WHATSAPP] send failed: %v", err)
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, resources.T(lang, "invoice.whatsapp_service_error"))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		helpers.HandleUnauthorized(w, r)
		return
	}
	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, body, resources.T(lang, "invoice.whatsapp_send_error"))
		return
	}

	msg := helpers.ExtractMessageFromBytes(body)
	if msg == "" || strings.EqualFold(msg, "sent") {
		msg = resources.T(lang, "invoice.whatsapp_send_success")
	}
	helpers.WriteSuccessToast(w, msg)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": msg})
}

// HandleInvoicePreview displays invoice preview page
func HandleInvoicePreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	invoice, _, _, _, err := helpers.FetchBillDetail(token, id)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	status, statusClass := helpers.InvoiceStatus(invoice)
	invoiceType := helpers.InvoiceTypeLabel(invoice)
	if invoice.State == 0 {
		invoiceType = fmt.Sprintf("%s %s", invoiceType, resources.T(lang, "invoice.draft_suffix"))
	}

	data := map[string]interface{}{
		"title":         resources.T(lang, "invoice.preview_title"),
		"invoice":       invoice,
		"total_display": fmt.Sprintf("%.2f", invoice.Total),
		"status":        status,
		"status_class":  statusClass,
		"type":          invoiceType,
	}
	helpers.RenderStandalone(w, r, "invoice-preview", data)
}

// HandleInvoicePrint redirects to the backend PDF for printing.
func HandleInvoicePrint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	http.Redirect(w, r, "/bill/pdf/"+id, http.StatusFound)
}

// HandleEditInvoice displays the edit invoice form
func HandleEditInvoice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	inv, products, manualProducts, extra, err := helpers.FetchBillDetail(token, id)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	// Build BillDetail from the parsed data
	maintCost := 0.0
	if v, ok := helpers.CoerceFloat(extra["maintenance_cost"]); ok {
		maintCost = v
	}
	bill := models.BillDetail{
		ID:              inv.ID,
		SequenceNumber:  inv.SequenceNumber,
		Products:        products,
		ManualProducts:  manualProducts,
		TotalAmount:     inv.Total,
		Discount:        inv.Discount,
		MaintenanceCost: maintCost,
		State:           inv.State,
	}
	if v, ok := extra["store_id"]; ok {
		if f, ok := helpers.CoerceFloat(v); ok {
			bill.StoreID = int(f)
		}
	}
	if v, ok := extra["branch_id"]; ok {
		if f, ok := helpers.CoerceFloat(v); ok {
			bill.BranchID = int(f)
		}
	}
	if v, ok := extra["user_name"].(string); ok {
		bill.UserName = v
	}
	if v, ok := extra["user_phone_number"].(string); ok {
		bill.UserPhoneNumber = v
	}
	if v, ok := extra["note"].(string); ok {
		bill.Note = v
	}

	stores, _ := helpers.FetchStores(token)
	branches, _ := helpers.FetchBranches(token)
	allProducts, _ := helpers.FetchProducts(token)

	// Extract payment_method for template
	billPaymentMethod := ""
	if v, ok := helpers.CoerceFloat(extra["payment_method"]); ok {
		billPaymentMethod = fmt.Sprintf("%d", int(v))
	}

	// Extract payment_due_date for template (format as YYYY-MM-DD)
	paymentDueDate := ""
	if v, ok := extra["payment_due_date"].(string); ok && len(v) >= 10 {
		paymentDueDate = v[:10]
	}

	// Extract deliver_date for template (format as YYYY-MM-DD)
	deliverDate := ""
	if v, ok := extra["deliver_date"].(string); ok && len(v) >= 10 {
		deliverDate = v[:10]
	}

	data := map[string]interface{}{
		"title":               resources.T(lang, "invoice.edit_title"),
		"id":                  id,
		"invoice":             bill,
		"stores":              stores,
		"branches":            branches,
		"all_products":        allProducts,
		"bill_payment_method": billPaymentMethod,
		"payment_due_date":    paymentDueDate,
		"deliver_date":        deliverDate,
		"is_company":          inv.Type,
	}

	if inv.Type {
		clientID := ""
		if v, ok := helpers.CoerceFloat(extra["client_id"]); ok && v > 0 {
			clientID = fmt.Sprintf("%d", int(v))
		} else if s, ok := extra["client_id"].(string); ok {
			clientID = s
		}
		data["client_id"] = clientID
		clients, _ := helpers.FetchClients(token)
		// Ensure the bill's client is in the dropdown even if it was excluded
		// from /client/all (e.g. soft-deleted or filtered). Otherwise the
		// <select> renders with no selected option and the form would
		// submit an empty client_id.
		if clientID != "" {
			found := false
			for _, c := range clients {
				if c.ID == clientID {
					found = true
					break
				}
			}
			if !found {
				if c, err := helpers.FetchClientByID(token, clientID); err == nil && c.ID != "" {
					clients = append([]models.Client{c}, clients...)
				} else {
					// Detail endpoint may 404/500 for soft-deleted clients.
					// Synthesize a placeholder option so the round-trip
					// preserves the bill's current client_id.
					clients = append([]models.Client{{ID: clientID, Name: "#" + clientID}}, clients...)
				}
			}
		}
		data["clients"] = clients
	}

	helpers.Render(w, r, "edit-invoice", data)
}

// HandleUpdateInvoice updates an existing invoice
func HandleUpdateInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	payload := helpers.BuildBillPayload(r)
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/bill/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, resources.T(lang, "invoice.update_error"))
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", resources.T(lang, "invoice.update_success"))
}

// HandleSubmitDraftInvoice converts a draft bill into a real bill by PUTing to /api/v2/bill/{id}.
// The backend expects the same payload as creating a new bill.
func HandleSubmitDraftInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	// Fetch current bill data to build the payload
	inv, products, manualProducts, extra, err := helpers.FetchBillDetail(token, id)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	if inv.State != 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, resources.T(lang, "invoice.not_draft"))
		return
	}

	// Build product items for payload
	prodItems := buildSubmitProductItems(products)
	manualItems := buildSubmitManualItems(manualProducts)

	payload := buildSubmitDraftPayload(inv, extra, prodItems, manualItems)
	if date := helpers.SafeString(extra["payment_due_date"]); date != "" {
		payload.PaymentDueDate = helpers.ToBackendDatePtr(date)
	}
	if date := helpers.SafeString(extra["deliver_date"]); date != "" {
		payload.DeliverDate = helpers.ToBackendDatePtr(date)
	}

	if v, ok := helpers.CoerceFloat(extra["maintenance_cost"]); ok && v > 0 {
		payload.MaintenanceCost = fmt.Sprintf("%g", v)
	}

	jsonPayload, _ := json.Marshal(payload)
	log.Printf("[SUBMIT DRAFT] ID=%s Payload: %s", id, string(jsonPayload))

	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/bill/"+id, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		log.Printf("[SUBMIT DRAFT] Request error: %v", err)
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[SUBMIT DRAFT] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, resources.T(lang, "invoice.submit_draft_error"))
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/bill/"+id, resources.T(lang, "invoice.submit_draft_success"))
}

// HandleDeleteInvoice deletes an invoice
func HandleDeleteInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/bill/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", resources.T(lang, "invoice.delete_success"))
}

// HandleCreateCompanyInvoice creates an invoice for a company client
func HandleCreateCompanyInvoice(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	// Build the invoice payload with client_id
	payload := map[string]interface{}{
		"store_id":          helpers.ParseIntValue(r.FormValue("store_id")),
		"client_id":         r.FormValue("client_id"),
		"user_name":         r.FormValue("user_name"),
		"user_phone_number": r.FormValue("user_phone_number"),
		"note":              r.FormValue("note"),
		"state":             1,
	}
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", resources.T(lang, "company_invoice.create_success"))
}

// buildSubmitProductItems converts persisted bill product rows into the
// API payload shape (string price, string quantity).
func buildSubmitProductItems(products []models.BillItem) []models.BillProductItem {
	items := make([]models.BillProductItem, 0, len(products))
	for _, p := range products {
		items = append(items, models.BillProductItem{
			ID:       p.ProductID,
			PartName: p.PartName,
			Price:    fmt.Sprintf("%g", p.Price),
			Quantity: strconv.Itoa(p.Quantity),
		})
	}
	return items
}

// buildSubmitManualItems converts persisted manual-product rows into the
// API payload shape (string price, string quantity).
func buildSubmitManualItems(products []models.BillItem) []models.BillManualItem {
	items := make([]models.BillManualItem, 0, len(products))
	for _, p := range products {
		items = append(items, models.BillManualItem{
			PartName:   p.PartName,
			PartNumber: p.PartNumber,
			Price:      fmt.Sprintf("%g", p.Price),
			Quantity:   strconv.Itoa(p.Quantity),
		})
	}
	return items
}

// extraInt reads an int field from the loose-typed `extra` map returned by
// FetchBillDetail; missing/invalid values become 0 to keep callers small.
func extraInt(extra map[string]interface{}, key string) int {
	if v, ok := helpers.CoerceFloat(extra[key]); ok {
		return int(v)
	}
	return 0
}

// extraIntPtr is like extraInt but yields *int for fields that distinguish
// "absent" from "zero" on the wire (e.g. client_id).
func extraIntPtr(extra map[string]interface{}, key string) *int {
	if v, ok := helpers.CoerceFloat(extra[key]); ok && v > 0 {
		i := int(v)
		return &i
	}
	return nil
}

// buildSubmitDraftPayload assembles the BillPayload for the
// draft-to-processing transition. Date and maintenance_cost overrides are
// applied by the caller because they require their own conditional logic.
func buildSubmitDraftPayload(inv models.Invoice, extra map[string]interface{}, prodItems []models.BillProductItem, manualItems []models.BillManualItem) models.BillPayload {
	p := models.BillPayload{
		StoreID:         extraInt(extra, "store_id"),
		Products:        prodItems,
		ManualProducts:  manualItems,
		TotalAmount:     inv.Total,
		Discount:        fmt.Sprintf("%g", inv.Discount),
		MaintenanceCost: "0",
		State:           1, // submit as processing
		VIN:             helpers.SafeString(extra["vin"]),
		UserName:        helpers.SafeString(extra["user_name"]),
		UserPhoneNumber: helpers.SafeString(extra["user_phone_number"]),
		Note:            helpers.SafeString(extra["note"]),
		PaymentMethod:   extraInt(extra, "payment_method"),
		ClientID:        extraIntPtr(extra, "client_id"),
		BranchID:        extraInt(extra, "branch_id"),
	}
	if inv.EffectiveDate.Valid {
		p.EffectiveDate = helpers.ToBackendDatePtr(inv.EffectiveDate.Time)
	}
	return p
}
