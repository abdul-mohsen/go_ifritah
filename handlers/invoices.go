package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

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

	// Read pagination and filter parameters
	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	if perPage < 1 {
		perPage = 10
	}
	stateFilter := r.URL.Query().Get("state")
	query := r.URL.Query().Get("q")

	// Fetch invoices with pagination support and backend search
	invoices, err := helpers.FetchInvoicesAll(token, page, query)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	log.Printf("Fetched %d invoices from backend (page %d)", len(invoices), page)

	// Apply state filter before display formatting
	filtered := invoices
	if stateFilter != "" {
		stateValue := helpers.ParseIntValue(stateFilter)
		stateFiltered := make([]models.Invoice, 0, len(invoices))
		for _, inv := range invoices {
			if inv.State == stateValue {
				stateFiltered = append(stateFiltered, inv)
			}
		}
		filtered = stateFiltered
	}

	// Transform to display format
	displayInvoices := make([]map[string]interface{}, 0, len(filtered))
	for _, inv := range filtered {
		totalDisplay := fmt.Sprintf("%.2f", inv.Total)
		status, statusClass := helpers.InvoiceStatus(inv)
		status = helpers.TranslateInvoiceStatus(status)
		invoiceType := helpers.InvoiceTypeLabel(inv)
		if inv.State == 0 {
			invoiceType = fmt.Sprintf("%s (مسودة)", invoiceType)
		}

		displayInvoices = append(displayInvoices, map[string]interface{}{
			"id":              inv.ID,
			"sequence_number": inv.SequenceNumber,
			"total":           totalDisplay,
			"subtotal":        fmt.Sprintf("%.2f", inv.TotalBeforeVAT),
			"vat":             fmt.Sprintf("%.2f", inv.TotalVAT),
			"discount":        fmt.Sprintf("%.2f", inv.Discount),
			"date":            helpers.FormatInvoiceDate(inv.EffectiveDate.Time),
			"type":            invoiceType,
			"status":          status,
			"status_class":    statusClass,
			"state":           inv.State,
			"credit_state":    inv.CreditState,
			"is_credit":       inv.CreditState > 0,
		})
	}

	// Add order numbers
	offset := (page - 1) * perPage
	for i := range displayInvoices {
		displayInvoices[i]["order"] = offset + i + 1
	}

	prevPage := 0
	if page > 1 {
		prevPage = page - 1
	}
	// Only show next page if we got a full page of results from backend
	nextPage := 0
	if len(invoices) >= perPage {
		nextPage = page + 1
	}

	pagination := helpers.Pagination{
		Page:    page,
		PerPage: perPage,
		Total:   len(displayInvoices),
	}

	data := map[string]interface{}{
		"title":      "الفواتير",
		"invoices":   displayInvoices,
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

	stores, _ := helpers.FetchStores(token)
	branches, _ := helpers.FetchBranches(token)
	products, _ := helpers.FetchProducts(token)
	today := time.Now().Format("2006-01-02")

	isCompany := r.URL.Query().Get("type") == "company"

	data := map[string]interface{}{
		"title":      "إضافة فاتورة",
		"stores":     stores,
		"branches":   branches,
		"products":   products,
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
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "\u0644\u0627 \u062a\u0648\u062c\u062f \u0645\u062a\u0627\u062c\u0631 \u0644\u0625\u0646\u0634\u0627\u0621 \u0641\u0627\u062a\u0648\u0631\u0629")
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
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "فشل في إنشاء مسودة الفاتورة")
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", "تم إنشاء مسودة الفاتورة بنجاح")
}

// HandleAddCreditNote displays the add credit note form
func HandleAddCreditNote(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]
	data := map[string]interface{}{
		"title": "إضافة إشعار دائن",
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
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, "فشل في إنشاء الفاتورة")
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", "تم إنشاء الفاتورة بنجاح")
}

// HandleCreateCreditNote creates a new credit note
func HandleCreateCreditNote(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "\u0628\u064a\u0627\u0646\u0627\u062a \u0627\u0644\u0646\u0645\u0648\u0630\u062c \u063a\u064a\u0631 \u0635\u0627\u0644\u062d\u0629")
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
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", "تم إنشاء إشعار الدائن بنجاح")
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
		invoiceType = fmt.Sprintf("%s (مسودة)", invoiceType)
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
		"title":            "تفاصيل الفاتورة",
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
	}
	helpers.Render(w, r, "invoice-detail", data)
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
		invoiceType = fmt.Sprintf("%s (مسودة)", invoiceType)
	}

	data := map[string]interface{}{
		"title":         "معاينة الفاتورة",
		"invoice":       invoice,
		"total_display": fmt.Sprintf("%.2f", invoice.Total),
		"status":        status,
		"status_class":  statusClass,
		"type":          invoiceType,
	}
	helpers.RenderStandalone(w, "invoice-preview", data)
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
		"title":                "تعديل الفاتورة",
		"id":                  id,
		"invoice":             bill,
		"stores":              stores,
		"branches":            branches,
		"all_products":        allProducts,
		"bill_payment_method": billPaymentMethod,
		"payment_due_date":    paymentDueDate,
		"deliver_date":        deliverDate,
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
		helpers.WriteErrorResponse(w, resp.StatusCode, nil, "فشل في تحديث الفاتورة")
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", "تم تحديث الفاتورة بنجاح")
}

// HandleSubmitDraftInvoice converts a draft bill into a real bill by POSTing to /api/v2/bill/{id}.
// The backend expects the same payload as creating a new bill.
func HandleSubmitDraftInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

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
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "هذه الفاتورة ليست مسودة")
		return
	}

	// Build product items for payload
	prodItems := make([]models.BillProductItem, 0, len(products))
	for _, p := range products {
		prodItems = append(prodItems, models.BillProductItem{
			ID:       p.ProductID,
			PartName: p.PartName,
			Price:    fmt.Sprintf("%g", p.Price),
			Quantity: strconv.Itoa(p.Quantity),
		})
	}
	manualItems := make([]models.BillManualItem, 0, len(manualProducts))
	for _, p := range manualProducts {
		manualItems = append(manualItems, models.BillManualItem{
			PartName:   p.PartName,
			PartNumber: p.PartNumber,
			Price:      fmt.Sprintf("%g", p.Price),
			Quantity:   strconv.Itoa(p.Quantity),
		})
	}

	// Resolve store_id from extra
	storeID := 0
	if v, ok := helpers.CoerceFloat(extra["store_id"]); ok {
		storeID = int(v)
	}

	payload := models.BillPayload{
		StoreID:         storeID,
		Products:        prodItems,
		ManualProducts:  manualItems,
		TotalAmount:     inv.Total,
		Discount:        fmt.Sprintf("%g", inv.Discount),
		MaintenanceCost: "0",
		State:           1, // submit as processing
		UserName:        helpers.SafeString(extra["user_name"]),
		UserPhoneNumber: helpers.SafeString(extra["user_phone_number"]),
		Note:            helpers.SafeString(extra["note"]),
	}

	if v, ok := helpers.CoerceFloat(extra["maintenance_cost"]); ok && v > 0 {
		payload.MaintenanceCost = fmt.Sprintf("%g", v)
	}

	jsonPayload, _ := json.Marshal(payload)
	log.Printf("[SUBMIT DRAFT] ID=%s Payload: %s", id, string(jsonPayload))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill/"+id, bytes.NewBuffer(jsonPayload))
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
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, "فشل في تقديم المسودة")
		return
	}

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/bill/"+id, "تم تقديم المسودة كفاتورة بنجاح")
}

// HandleDeleteInvoice deletes an invoice
func HandleDeleteInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/bill/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("invoices_all")
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", "تم حذف الفاتورة بنجاح")
}

// HandleCreateCompanyInvoice creates an invoice for a company client
func HandleCreateCompanyInvoice(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

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
	helpers.WriteSuccessRedirect(w, "/dashboard/invoices", "تم إنشاء فاتورة الشركة بنجاح")
}
