package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

	"github.com/gorilla/mux"
)

// purchaseBillCreateLock prevents duplicate purchase bill creation per user session.
var purchaseBillCreateLock sync.Map

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

	// Fetch ALL from backend (page=1 returns all), client-side pagination after state filter
	bills, err := helpers.FetchPurchaseBillsAll(token, 1, query)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	displayBills := make([]map[string]interface{}, 0)
	for i, inv := range bills {
		status, statusClass := helpers.InvoiceStatus(inv)
		status = helpers.TranslateInvoiceStatus(status)
		invoiceType := helpers.InvoiceTypeLabel(inv)
		// Format date: strip time portion from ISO timestamp
		dateStr := inv.EffectiveDate.Time
		if len(dateStr) >= 10 {
			dateStr = dateStr[:10]
		}
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

	// Apply state filter
	if stateFilter != "" {
		stateValue, _ := strconv.Atoi(stateFilter)
		filtered := make([]map[string]interface{}, 0)
		for _, b := range displayBills {
			if s, ok := b["state"].(int); ok && s == stateValue {
				filtered = append(filtered, b)
			}
		}
		displayBills = filtered
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

	helpers.Render(w, r, "add-purchase-bill", map[string]interface{}{
		"title":     "إضافة فاتورة مشتريات",
		"stores":    stores,
		"suppliers": suppliers,
	})
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
	jsonPayload, _ := json.Marshal(payload)

	log.Printf("[CREATE PURCHASE BILL] Payload: %s", string(jsonPayload))

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

	// Auto-create products in the store from the purchase bill items
	autoCreateProductsFromPurchaseBill(token, int(payload.StoreID), r)

	helpers.APICache.Delete("purchase_bills")
	helpers.WriteSuccessRedirect(w, "/dashboard/purchase-bills", "تم إنشاء فاتورة الشراء بنجاح")
}

// autoCreateProductsFromPurchaseBill creates products in the store for each item in the purchase bill.
// It reads cost_price[] from the form and uses the purchase bill product data.
func autoCreateProductsFromPurchaseBill(token string, storeID int, r *http.Request) {
	productIDs := r.Form["products_product_id"]
	quantities := r.Form["products_quantity"]
	prices := r.Form["products_price"]
	costPrices := r.Form["products_cost_price"]
	shelfNumbers := r.Form["products_shelf_number"]

	if len(quantities) == 0 || storeID == 0 {
		return
	}

	products := make([]map[string]interface{}, 0, len(quantities))
	for i := range quantities {
		// Skip manual items (product_id=0 means unlinked/manual)
		pid := 0
		if i < len(productIDs) {
			pid, _ = strconv.Atoi(productIDs[i])
		}
		if pid == 0 {
			continue
		}

		qty, _ := strconv.Atoi(quantities[i])
		price := 0
		if i < len(prices) {
			price, _ = strconv.Atoi(prices[i])
		}
		costPrice := 0
		if i < len(costPrices) {
			costPrice, _ = strconv.Atoi(costPrices[i])
		}
		shelfNum := ""
		if i < len(shelfNumbers) {
			shelfNum = shelfNumbers[i]
		}

		if qty == 0 && price == 0 {
			continue
		}

		products = append(products, map[string]interface{}{
			"product_id":   pid,
			"quantity":     qty,
			"price":        price,
			"cost_price":   costPrice,
			"shelf_number": shelfNum,
		})
	}

	if len(products) == 0 {
		return
	}

	payloadMap := map[string]interface{}{
		"store_id": storeID,
		"products": products,
	}
	jsonPayload, _ := json.Marshal(payloadMap)
	log.Printf("[AUTO-CREATE PRODUCTS] From purchase bill → %s", string(jsonPayload))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/product", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		log.Printf("[AUTO-CREATE PRODUCTS] Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[AUTO-CREATE PRODUCTS] Backend error: %d body=[%s]", resp.StatusCode, string(respBody))
	} else {
		log.Printf("[AUTO-CREATE PRODUCTS] Success: %d products created in store %d", len(products), storeID)
	}
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

	// Status label and class
	status, statusClass := helpers.InvoiceStatus(invoice)
	status = helpers.TranslateInvoiceStatus(status)

	// Format effective date
	effectiveDate := ""
	if invoice.EffectiveDate.Valid && invoice.EffectiveDate.Time != "" {
		if len(invoice.EffectiveDate.Time) >= 10 {
			effectiveDate = invoice.EffectiveDate.Time[:10]
		}
	}

	// Supplier sequence number
	supplierSeqNum := ""
	if v, ok := extra["supplier_sequence_number"].(string); ok && v != "" {
		supplierSeqNum = v
	} else if v, ok := helpers.CoerceFloat(extra["supplier_sequence_number"]); ok && v > 0 {
		supplierSeqNum = fmt.Sprintf("%.0f", v)
	}

	// Deliver date
	deliverDate := ""
	if v, ok := extra["deliver_date"].(string); ok && len(v) >= 10 {
		deliverDate = v[:10]
	}

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
	if v, ok := extra["payment_due_date"].(string); ok && len(v) >= 10 {
		paymentDueDate = v[:10]
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

	helpers.Render(w, r, "purchase-bill-detail", map[string]interface{}{
		"title":                    "تفاصيل فاتورة المشتريات",
		"bill":                     invoice,
		"bill_id":                  id,
		"catalog_products":          products,
		"manual_products":           manualProducts,
		"products_subtotal":         helpers.SumBillItemsTotal(products),
		"manual_subtotal":           helpers.SumBillItemsTotal(manualProducts),
		"store_name":               storeName,
		"supplier_name":            supplierName,
		"supplier":                 matchedSupplier,
		"store_id":                 storeID,
		"merchant_id":              merchantID,
		"status_label":             status,
		"status_class":             statusClass,
		"effective_date":           effectiveDate,
		"payment_due_date":         paymentDueDate,
		"type_label":               typeLabel,
		"total_display":            fmt.Sprintf("%.2f", total),
		"vat_amount":               invoice.VAT,
		"supplier_sequence_number": supplierSeqNum,
		"deliver_date":             deliverDate,
		"note":                     note,
		"payment_method":           paymentMethod,
		"pdf_link_key":             pdfLinkKey,
	})
}

// HandleEditPurchaseBill renders the edit purchase bill form.
func HandleEditPurchaseBill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/purchase_bill/"+id, nil)
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

	// Resolve float64 IDs from JSON to int for template comparison
	billStoreID := int(helpers.SafeFloat(bill["store_id"]))
	billSupplierID := int(helpers.SafeFloat(bill["merchant_id"]))

	// Format effective_date for the date input
	editDate := ""
	if ed, ok := bill["effective_date"].(map[string]interface{}); ok {
		if t, ok := ed["Time"].(string); ok && len(t) >= 10 {
			editDate = t[:10]
		}
	}

	helpers.Render(w, r, "edit-purchase-bill", map[string]interface{}{
		"title":          "تعديل فاتورة المشتريات",
		"bill":           bill,
		"bill_id":        id,
		"stores":         stores,
		"suppliers":      suppliers,
		"all_products":   products,
		"store_id":       billStoreID,
		"supplier_id":    billSupplierID,
		"edit_date":      editDate,
		"bill_products":  billProducts,
		"bill_manual":    billManual,
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

	req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/purchase_bill/"+id, bytes.NewBuffer(body))
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

	req, _ := http.NewRequest("DELETE", config.BackendDomain+"/api/v2/purchase_bill/"+id, nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	defer resp.Body.Close()

	helpers.APICache.Delete("purchase_bills")
	helpers.WriteSuccessRedirect(w, "/dashboard/purchase-bills", "تم حذف فاتورة الشراء بنجاح")
}
