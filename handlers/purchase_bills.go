package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

	"github.com/gorilla/mux"
)

const purchaseBillAPIPath = "/api/v2/purchase_bill/"

// purchaseBillCreateLock prevents duplicate purchase bill creation per user session.
var purchaseBillCreateLock sync.Map

type purchaseBillDuplicateCheckRequest struct {
	SupplierID             int    `json:"supplier_id"`
	SupplierSequenceNumber uint64 `json:"supplier_sequence_number"`
}

func purchaseBillReceiptEnabled(r *http.Request) bool {
	tenantID := config.TenantID
	if value, ok := r.Context().Value(config.TenantIDContextKey).(string); ok && value != "" {
		tenantID = value
	}
	return helpers.IsEnabled(tenantID, config.FeaturePurchaseBillReceipt)
}

func purchaseBillHeaderData(r *http.Request, invoice models.Invoice, extra map[string]interface{}, id string) map[string]interface{} {
	status, statusClass := helpers.InvoiceStatus(invoice)
	status = helpers.TranslateInvoiceStatus(status)

	effectiveDate := ""
	if invoice.EffectiveDate.Valid && invoice.EffectiveDate.Time != "" {
		effectiveDate = helpers.ToDisplayDate(invoice.EffectiveDate.Time)
	}

	supplierSeqNum := ""
	if v, ok := extra["supplier_sequence_number"].(string); ok && v != "" {
		supplierSeqNum = v
	} else if v, ok := helpers.CoerceFloat(extra["supplier_sequence_number"]); ok && v > 0 {
		supplierSeqNum = fmt.Sprintf("%.0f", v)
	}

	return map[string]interface{}{
		"bill":                     invoice,
		"bill_id":                  id,
		"status_label":             status,
		"status_class":             statusClass,
		"effective_date":           effectiveDate,
		"supplier_sequence_number": supplierSeqNum,
		"received_date":            extractDateField(extra["received_at"]),
		"receipt_enabled":          purchaseBillReceiptEnabled(r),
	}
}

// HandlePurchaseBills renders the purchase bills list page.
func HandlePurchaseBills(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	query := r.URL.Query().Get("q")
	stateFilter := r.URL.Query().Get("state")
	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	if page < 0 {
		page = 0
	}

	// Search + state filter are BE-driven. Sort is FE-only (see
	// static/js/script.js sortable table init).
	bills, err := helpers.FetchPurchaseBillsAll(token, 1, query, stateFilter)
	backendErr := ""
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		log.Printf("[purchase-bills] backend list fetch failed: %v", err)
		bills = nil
		backendErr = "تعذر تحميل فواتير المشتريات من الخادم حالياً"
	}

	displayBills := make([]map[string]interface{}, 0)
	for i, inv := range bills {
		status, statusClass := helpers.InvoiceStatus(inv)
		status = helpers.TranslateInvoiceStatus(status)
		invoiceType := helpers.InvoiceTypeLabel(inv)
		// Format date: re-localize to Riyadh before display. The backend may
		// return this as a UTC-offset timestamp; naively slicing the first 10
		// characters (as this used to do) silently shifts the calendar date
		// by a day for any wall-clock time before 03:00 Riyadh time.
		dateStr := helpers.ToDisplayDate(inv.EffectiveDate.Time)
		supplierSeq := ""
		if inv.SupplierSequenceNumber > 0 {
			supplierSeq = fmt.Sprintf("%d", inv.SupplierSequenceNumber)
		}
		displayBills = append(displayBills, map[string]interface{}{
			"id":                       inv.ID,
			"order":                    i + 1,
			"supplier_sequence_number": supplierSeq,
			"total":                    fmt.Sprintf("%.2f", inv.Total),
			"date":                     dateStr,
			"type":                     invoiceType,
			"state":                    inv.State,
			"status":                   status,
			"status_class":             statusClass,
		})
	}

	pagedBills, pagination := helpers.PaginateSlice(displayBills, page, perPage)
	prevPage := -1
	nextPage := -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "purchase-bills", map[string]interface{}{
		"title":      "فواتير المشتريات",
		"bills":      pagedBills,
		"pagination": pagination,
		"prev_page":  prevPage,
		"next_page":  nextPage,
		"query":      query,
		"state":      stateFilter,
		"error":      backendErr,
	})
}

// HandleAddPurchaseBill renders the create purchase bill form.
func HandleAddPurchaseBill(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	stores, _ := helpers.FetchStores(token)
	suppliers, _ := helpers.FetchSuppliers(token)
	today := time.Now().Format("2006-01-02")

	helpers.Render(w, r, "add-purchase-bill", map[string]interface{}{
		"title":           "إضافة فاتورة مشتريات",
		"stores":          stores,
		"suppliers":       suppliers,
		"pb_pdf_required": GetSettingValue(token, "pb_pdf_required"),
		"today":           today,
	})
}

func HandlePurchaseBillDuplicateCheck(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	var payload purchaseBillDuplicateCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.SupplierID <= 0 || payload.SupplierSequenceNumber == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "invalid duplicate check payload",
		})
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "failed to build duplicate check payload",
		})
		return
	}

	req, err := http.NewRequest(http.MethodPost, config.BackendDomain+"/api/v2/purchase_bill/duplicate-check", bytes.NewBuffer(body))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "failed to build duplicate check request",
		})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		log.Printf("[PURCHASE BILL DUPLICATE CHECK] backend request failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "duplicate check unavailable",
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

// HandleCreatePurchaseBill creates a new purchase bill.
func HandleCreatePurchaseBill(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Prevent duplicate submissions: only one in-flight create per user session
	if _, loaded := purchaseBillCreateLock.LoadOrStore(token, true); loaded {
		log.Printf("[CREATE PURCHASE BILL] Duplicate request blocked for token=%s…", token[:8])
		// Return silently — the first request is still processing
		w.WriteHeader(http.StatusNoContent)
		return
	}
	defer purchaseBillCreateLock.Delete(token)

	payload := helpers.BuildPurchaseBillPayload(r)
	if payload.Subtotal <= 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "You can't submit an invoice with 0")
		return
	}
	jsonPayload, _ := json.Marshal(payload)

	log.Printf("[CREATE PURCHASE BILL] Payload: %s", helpers.SanitizeForLog(string(jsonPayload)))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/purchase_bill", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[CREATE PURCHASE BILL] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, "فشل في إنشاء فاتورة الشراء")
		return
	}

	helpers.APICache.Delete("purchase_bills")
	helpers.WriteSuccessRedirect(w, "/dashboard/purchase-bills", "تم إنشاء فاتورة الشراء بنجاح")
}

// HandleGetPurchaseBill shows details for a purchase bill.
func HandleGetPurchaseBill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Fetch and parse the purchase bill into structured data
	invoice, products, manualProducts, extra, err := helpers.FetchPurchaseBillDetail(token, id)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	// Keep products and manual_products separate for the detail template

	// Resolve store and supplier names
	stores, _ := helpers.FetchStores(token)
	suppliers, _ := helpers.FetchSuppliers(token)

	storeName := ""
	storeID := 0
	if v, ok := extra["store_id"]; ok {
		storeID = int(helpers.SafeFloat(v))
	}
	for _, s := range stores {
		if s.ID == storeID {
			storeName = s.Name
			break
		}
	}

	supplierName := ""
	var matchedSupplier *models.Supplier
	merchantID := 0
	if v, ok := extra["merchant_id"]; ok {
		merchantID = int(helpers.SafeFloat(v))
	}
	// supplier_id is the actual supplier reference; merchant_id is the company/tenant
	supplierID := 0
	if v, ok := extra["supplier_id"]; ok {
		supplierID = int(helpers.SafeFloat(v))
	}
	for i, s := range suppliers {
		if s.ID == supplierID {
			supplierName = s.Name
			matchedSupplier = &suppliers[i]
			break
		}
	}

	// Deliver date
	deliverDate := extractDateField(extra["deliver_date"])

	// Note
	note := ""
	if v, ok := extra["note"].(string); ok {
		note = v
	}

	// Payment method
	paymentMethod := ""
	if v, ok := extra["payment_method"].(string); ok {
		paymentMethod = v
	}

	// Compute total if backend doesn't return it (purchase bills: total = subtotal + vat - discount)
	total := invoice.Total
	if total == 0 && invoice.Subtotal > 0 {
		discountAmount := invoice.Subtotal * invoice.Discount / 100
		total = invoice.Subtotal - discountAmount + invoice.TotalVAT
		// If TotalVAT is 0 but VAT amount is in the vat field (purchase bills use vat as amount)
		if invoice.TotalVAT == 0 && invoice.VAT > 0 && invoice.VAT < invoice.Subtotal {
			total = invoice.Subtotal - discountAmount + invoice.VAT
		}
	}

	// Format payment due date
	paymentDueDate := ""
	if v, ok := extra["payment_due_date"].(string); ok {
		paymentDueDate = helpers.ToDisplayDate(v)
	}

	// Bill type label
	typeLabel := "فاتورة مشتريات"
	if invoice.Type {
		typeLabel = "فاتورة مشتريات (شركة)"
	}

	// Extract pdf_link if it has a valid file extension
	pdfLinkKey := ""
	if v, ok := extra["pdf_link"].(string); ok && v != "" {
		if strings.Contains(filepath.Base(v), ".") {
			ext := filepath.Ext(v)
			if ext != "" && len(ext) > 1 {
				pdfLinkKey = v
			}
		}
	}

	data := map[string]interface{}{
		"title":             "تفاصيل فاتورة المشتريات",
		"bill":              invoice,
		"bill_id":           id,
		"catalog_products":  products,
		"manual_products":   manualProducts,
		"products_subtotal": helpers.SumBillItemsTotal(products),
		"manual_subtotal":   helpers.SumBillItemsTotal(manualProducts),
		// Detail view merges catalog+manual into one items table (a single
		// per-row badge distinguishes the two, matching add/edit's item
		// list) - needs one combined subtotal rather than the two separate
		// per-table ones above (kept for back-compat, no longer rendered).
		"items_subtotal":   helpers.SumBillItemsTotal(append(append([]models.BillItem{}, products...), manualProducts...)),
		"store_name":       storeName,
		"supplier_name":    supplierName,
		"supplier":         matchedSupplier,
		"store_id":         storeID,
		"merchant_id":      merchantID,
		"payment_due_date": paymentDueDate,
		"type_label":       typeLabel,
		"total_display":    fmt.Sprintf("%.2f", total),
		"vat_amount":       invoice.VAT,
		"deliver_date":     deliverDate,
		"note":             note,
		"payment_method":   paymentMethod,
		"pdf_link_key":     pdfLinkKey,
	}
	for key, value := range purchaseBillHeaderData(r, invoice, extra, id) {
		data[key] = value
	}

	helpers.Render(w, r, "purchase-bill-detail", data)
}

// HandleMarkPurchaseBillReceived confirms receipt of the purchase bill's goods.
func HandleMarkPurchaseBillReceived(w http.ResponseWriter, r *http.Request) {
	handlePurchaseBillReceipt(w, r, http.MethodPut)
}

// HandleUnmarkPurchaseBillReceived clears the purchase bill receipt confirmation.
func HandleUnmarkPurchaseBillReceived(w http.ResponseWriter, r *http.Request) {
	handlePurchaseBillReceipt(w, r, http.MethodDelete)
}

func handlePurchaseBillReceipt(w http.ResponseWriter, r *http.Request, method string) {
	id := mux.Vars(r)["id"]
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, err := http.NewRequestWithContext(
		r.Context(),
		method,
		config.BackendDomain+purchaseBillAPIPath+id+"/received",
		nil,
	)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر تحديث حالة الاستلام")
		return
	}

	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "تعذر الاتصال بخادم الفواتير")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		helpers.HandleUnauthorized(w, r)
		return
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "تعذر تحديث حالة الاستلام")
		return
	}

	helpers.APICache.Delete("purchase_bills")
	invoice, _, _, extra, err := helpers.FetchPurchaseBillDetail(token, id)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "تم التحديث لكن تعذر تحميل حالة الفاتورة")
		return
	}

	data := purchaseBillHeaderData(r, invoice, extra, id)
	data["receipt_oob"] = true
	helpers.RenderPartial(w, "purchase-bill-header", data)
}

// HandleEditPurchaseBill renders the edit purchase bill form.
func HandleEditPurchaseBill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("GET", config.BackendDomain+purchaseBillAPIPath+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "")
		return
	}

	var bill map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&bill); err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	stores, _ := helpers.FetchStores(token)
	suppliers, _ := helpers.FetchSuppliers(token)
	products, _ := helpers.FetchProducts(token)

	// Decode products from the raw bill (handles base64-encoded JSON)
	billProducts := helpers.ParseBillItemsPublic(bill["products"])
	billManual := helpers.ParseBillItemsPublic(bill["manual_products"])

	// Resolve float64 IDs from JSON to int for template comparison.
	// supplier_id is the actual supplier reference; merchant_id is the
	// company/tenant. The detail page already distinguishes them — the
	// edit form must use supplier_id, otherwise the supplier dropdown
	// renders blank and the user has to re-select on every edit.
	billStoreID := int(helpers.SafeFloat(bill["store_id"]))
	billSupplierID := int(helpers.SafeFloat(bill["supplier_id"]))
	if billSupplierID == 0 {
		billSupplierID = int(helpers.SafeFloat(bill["merchant_id"]))
	}

	// Format effective_date for the date input. The backend may return it
	// as a {Time, Valid} wrapper, a plain string, or omit it entirely.
	editDate := extractDateField(bill["effective_date"])
	if editDate == "" {
		editDate = extractDateField(bill["payment_date"])
	}

	// Extract additional fields the edit template references but the
	// previous handler did not pass. Without these the inputs render as
	// empty strings and silently overwrite real values on save.
	paymentDueDate := extractDateField(bill["payment_due_date"])
	deliverDate := extractDateField(bill["deliver_date"])
	supplierSeqNum := firstNonEmptyString(bill, "supplier_sequance_number", "supplier_sequence_number")
	billPaymentMethod := ""
	if v, ok := helpers.CoerceFloat(bill["payment_method"]); ok && v > 0 {
		billPaymentMethod = fmt.Sprintf("%d", int(v))
	}
	discount, _ := helpers.CoerceFloat(bill["discount"])
	subtotal := firstFloat(bill, "total_amount", "total")

	helpers.Render(w, r, "edit-purchase-bill", map[string]interface{}{
		"title":                    "تعديل فاتورة المشتريات",
		"bill":                     bill,
		"bill_id":                  id,
		"stores":                   stores,
		"suppliers":                suppliers,
		"all_products":             products,
		"store_id":                 billStoreID,
		"supplier_id":              billSupplierID,
		"edit_date":                editDate,
		"payment_due_date":         paymentDueDate,
		"deliver_date":             deliverDate,
		"supplier_sequence_number": supplierSeqNum,
		"bill_payment_method":      billPaymentMethod,
		"discount":                 discount,
		"subtotal":                 subtotal,
		"bill_products":            billProducts,
		"bill_manual":              billManual,
		"pb_pdf_required":          GetSettingValue(token, "pb_pdf_required"),
	})
}

// HandleUpdatePurchaseBill updates a purchase bill.
func HandleUpdatePurchaseBill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	payload := helpers.BuildPurchaseBillPayload(r)
	body, _ := json.Marshal(payload)

	log.Printf("[UPDATE PURCHASE BILL] ID=%s Payload: %s", id, string(body))

	req, _ := http.NewRequest("PUT", config.BackendDomain+purchaseBillAPIPath+id, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[UPDATE PURCHASE BILL] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, respBody, "فشل في تحديث فاتورة الشراء")
		return
	}

	helpers.APICache.Delete("purchase_bills")
	helpers.WriteSuccessRedirect(w, "/dashboard/purchase-bills", "تم تحديث فاتورة الشراء بنجاح")
}

// HandleDeletePurchaseBill deletes a purchase bill.
func HandleDeletePurchaseBill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("DELETE", config.BackendDomain+purchaseBillAPIPath+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("purchase_bills")
	helpers.WriteSuccessRedirect(w, "/dashboard/purchase-bills", "تم حذف فاتورة الشراء بنجاح")
}

// extractDateField pulls a raw date/time string out of a backend date field
// that may be either a {Time, Valid} object, a plain ISO string, or absent,
// then re-localizes it to Asia/Riyadh via helpers.ToDisplayDate. Do not
// naively slice the raw string - the backend may return a UTC-offset
// timestamp, and slicing the first 10 characters silently shifts the
// calendar date by a day for any wall-clock time before 03:00 Riyadh time.
func extractDateField(raw interface{}) string {
	if m, ok := raw.(map[string]interface{}); ok {
		if t, ok := m["Time"].(string); ok {
			return helpers.ToDisplayDate(t)
		}
		return ""
	}
	if s, ok := raw.(string); ok {
		return helpers.ToDisplayDate(s)
	}
	return ""
}

// firstNonEmptyString returns the first string-valued key from m whose
// value is non-empty. Used when the backend exposes the same field under
// multiple historical names (e.g. supplier_sequance_number vs *_sequence_*).
func firstNonEmptyString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// firstFloat returns the first numeric-valued key from m. CoerceFloat
// accepts both float64 and numeric strings.
func firstFloat(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := helpers.CoerceFloat(m[k]); ok {
			return v
		}
	}
	return 0
}
