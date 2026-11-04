package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
			"title":      "الفواتير",
			"invoices":   []map[string]interface{}{},
			"pagination": helpers.Pagination{Page: 0, PerPage: 10, Total: 0, TotalPages: 0},
			"prev_page":  -1,
			"next_page":  -1,
			"query":      query,
			"state":      stateFilter,
			"error":      "تعذر تحميل الفواتير من الخادم حالياً",
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
			invoiceType = fmt.Sprintf("%s (مسودة)", invoiceType)
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
		"title":      "الفواتير",
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

	stores, _ := helpers.FetchStores(token)
	branches, _ := helpers.FetchBranches(token)
	today := time.Now().Format("2006-01-02")

	isCompany := r.URL.Query().Get("type") == "company"

	data := map[string]interface{}{
		"title":      "إضافة فاتورة",
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

	storeDetail, err := helpers.FetchStoreByID(token, stores[0].ID)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
	if storeDetail.BranchID == nil || *storeDetail.BranchID == 0 {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "لا يوجد فرع مرتبط بالمتجر الافتراضي لإنشاء مسودة الفاتورة")
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
		PaymentMethod:   10, // cash
		BranchID:        *storeDetail.BranchID,
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

const maxCreditSourceResponseBytes = 1 << 20

type creditDecimal string

func (d *creditDecimal) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("invalid credit decimal")
	}

	var value string
	switch {
	case data[0] == '"':
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode credit decimal: %w", err)
		}
	case data[0] == '-' || data[0] >= '0' && data[0] <= '9':
		var number json.Number
		if err := json.Unmarshal(data, &number); err != nil {
			return fmt.Errorf("decode credit decimal: %w", err)
		}
		value = number.String()
	default:
		return fmt.Errorf("credit decimal must be a JSON string or number")
	}
	if _, err := parseCreditDecimalValue(value); err != nil {
		return err
	}

	*d = creditDecimal(value)
	return nil
}

func (d creditDecimal) MarshalJSON() ([]byte, error) {
	if _, err := parseCreditDecimalValue(string(d)); err != nil {
		return nil, err
	}
	return json.Marshal(string(d))
}

func parseCreditDecimalValue(value string) (float64, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return 0, fmt.Errorf("invalid credit decimal")
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("invalid credit decimal")
	}
	return parsed, nil
}

type creditSourceLine struct {
	BillProductID int64         `json:"bill_product_id"`
	Quantity      creditDecimal `json:"quantity"`
	Price         creditDecimal `json:"price"`
	VAT           creditDecimal `json:"vat"`
}

type creditSourceSnapshot struct {
	BillID   int64              `json:"bill_id"`
	BranchID int64              `json:"branch_id"`
	Amount   creditDecimal      `json:"amount"`
	Lines    []creditSourceLine `json:"lines"`
}

type creditNotePayload struct {
	BillID        int64              `json:"bill_id"`
	BranchID      int64              `json:"branch_id"`
	PosID         string             `json:"pos_id"`
	BillRequestID string             `json:"bill_request_id"`
	Reason        string             `json:"reason"`
	Amount        creditDecimal      `json:"amount"`
	Lines         []creditSourceLine `json:"lines"`
}

type creditSourceFetchError struct {
	statusCode int
	body       []byte
}

type creditAttemptCacheEntry struct {
	billID int64
	body   string
}

type creditAttemptKeyLock struct {
	mu         sync.Mutex
	references int
}

var creditAttemptKeyLocks = struct {
	sync.Mutex
	locks map[string]*creditAttemptKeyLock
}{locks: make(map[string]*creditAttemptKeyLock)}

const creditAttemptPayloadCacheTTL = 30 * time.Minute

func creditAttemptCacheKey(sessionID, posID, billRequestID string) string {
	hash := sha256.New()
	for _, value := range []string{sessionID, posID, billRequestID} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	return "credit_note_attempt_" + hex.EncodeToString(hash.Sum(nil))
}

func acquireCreditAttemptKeyLock(key string) func() {
	creditAttemptKeyLocks.Lock()
	lock := creditAttemptKeyLocks.locks[key]
	if lock == nil {
		lock = &creditAttemptKeyLock{}
		creditAttemptKeyLocks.locks[key] = lock
	}
	lock.references++
	creditAttemptKeyLocks.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		creditAttemptKeyLocks.Lock()
		lock.references--
		if lock.references == 0 {
			delete(creditAttemptKeyLocks.locks, key)
		}
		creditAttemptKeyLocks.Unlock()
	}
}

func parseCreditBillID(value string) (int64, bool) {
	billID, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return billID, err == nil && billID > 0
}

func validCreditPOSID(value string) bool {
	return value != "" && len(value) <= 64 && value == strings.TrimSpace(value)
}

func validCreditRequestID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, char := range value {
		switch i {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if !(char >= '0' && char <= '9') &&
				!(char >= 'a' && char <= 'f') &&
				!(char >= 'A' && char <= 'F') {
				return false
			}
		}
	}
	switch value[14] {
	case '1', '2', '3', '4', '5', '6', '7', '8':
	default:
		return false
	}
	switch value[19] {
	case '8', '9', 'a', 'A', 'b', 'B':
		return true
	default:
		return false
	}
}

func validCreditSource(source creditSourceSnapshot, billID int64) bool {
	amount, err := parseCreditDecimalValue(string(source.Amount))
	if source.BillID != billID || source.BranchID <= 0 ||
		err != nil || amount <= 0 || len(source.Lines) == 0 {
		return false
	}
	for _, line := range source.Lines {
		quantity, quantityErr := parseCreditDecimalValue(string(line.Quantity))
		price, priceErr := parseCreditDecimalValue(string(line.Price))
		vat, vatErr := parseCreditDecimalValue(string(line.VAT))
		if line.BillProductID <= 0 ||
			quantityErr != nil || quantity <= 0 ||
			priceErr != nil || price < 0 ||
			vatErr != nil || vat < 0 {
			return false
		}
	}
	return true
}

func fetchCreditSource(token string, billID int64) (creditSourceSnapshot, *creditSourceFetchError) {
	endpoint := strings.TrimRight(config.BackendDomain, "/") +
		"/api/v2/bill/" + strconv.FormatInt(billID, 10) + "/credit-source"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return creditSourceSnapshot{}, &creditSourceFetchError{statusCode: http.StatusBadGateway}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		return creditSourceSnapshot{}, &creditSourceFetchError{statusCode: http.StatusBadGateway}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCreditSourceResponseBytes+1))
	if err != nil || len(body) > maxCreditSourceResponseBytes {
		return creditSourceSnapshot{}, &creditSourceFetchError{statusCode: http.StatusBadGateway}
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		statusCode := resp.StatusCode
		if statusCode < http.StatusBadRequest || statusCode > 599 {
			statusCode = http.StatusBadGateway
		}
		return creditSourceSnapshot{}, &creditSourceFetchError{statusCode: statusCode, body: body}
	}

	var source creditSourceSnapshot
	if err := json.Unmarshal(body, &source); err != nil || !validCreditSource(source, billID) {
		return creditSourceSnapshot{}, &creditSourceFetchError{statusCode: http.StatusBadGateway}
	}
	return source, nil
}

func creditPayloadForAttempt(
	sessionID string,
	token string,
	billID int64,
	posID string,
	billRequestID string,
	reason string,
) (string, *creditSourceFetchError, bool, error) {
	cacheKey := creditAttemptCacheKey(sessionID, posID, billRequestID)
	unlock := acquireCreditAttemptKeyLock(cacheKey)
	defer unlock()

	if cached, ok := helpers.APICache.Get(cacheKey); ok {
		entry, validEntry := cached.(creditAttemptCacheEntry)
		if !validEntry || entry.body == "" {
			helpers.APICache.Delete(cacheKey)
		} else if entry.billID != billID {
			return "", nil, true, nil
		} else {
			return entry.body, nil, false, nil
		}
	}

	source, failure := fetchCreditSource(token, billID)
	if failure != nil {
		return "", failure, false, nil
	}

	payload, err := json.Marshal(creditNotePayload{
		BillID:        source.BillID,
		BranchID:      source.BranchID,
		PosID:         posID,
		BillRequestID: billRequestID,
		Reason:        reason,
		Amount:        source.Amount,
		Lines:         source.Lines,
	})
	if err != nil {
		return "", nil, false, err
	}

	body := string(payload)
	helpers.APICache.Set(cacheKey, creditAttemptCacheEntry{billID: source.BillID, body: body}, creditAttemptPayloadCacheTTL)
	return body, nil, false, nil
}

func writeCreditSourceError(w http.ResponseWriter, failure *creditSourceFetchError) {
	if failure == nil {
		return
	}
	helpers.WriteErrorResponseFromBytes(
		w,
		failure.statusCode,
		failure.body,
		"تعذر التحقق من بيانات الفاتورة للإشعار الدائن",
	)
}

func isValidCreditReason(reason string) bool {
	return utf8.ValidString(reason) &&
		strings.TrimSpace(reason) != "" &&
		utf8.RuneCountInString(reason) <= 1000
}

// HandleAddCreditNote displays the add credit note form
func HandleAddCreditNote(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	billID, validID := parseCreditBillID(vars["id"])
	if !validID {
		helpers.WriteErrorResponseFromBytes(w, http.StatusBadRequest, nil, "رقم الفاتورة غير صالح")
		return
	}
	source, failure := fetchCreditSource(token, billID)
	if failure != nil {
		writeCreditSourceError(w, failure)
		return
	}

	data := map[string]interface{}{
		"title":  "إضافة إشعار دائن",
		"id":     strconv.FormatInt(billID, 10),
		"Source": source,
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

	r.Body = http.MaxBytesReader(w, r.Body, maxCreditSourceResponseBytes)
	if err := r.ParseForm(); err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "بيانات النموذج غير صالحة")
		return
	}

	billID, validID := parseCreditBillID(r.FormValue("bill_id"))
	reason := strings.TrimSpace(r.FormValue("reason"))
	posID := r.FormValue("pos_id")
	billRequestID := r.FormValue("bill_request_id")
	if !validID || !isValidCreditReason(reason) ||
		!validCreditPOSID(posID) || !validCreditRequestID(billRequestID) {
		helpers.WriteErrorResponseFromBytes(w, http.StatusBadRequest, nil, "بيانات الإشعار الدائن غير صالحة")
		return
	}

	sessionID := helpers.GetSessionIDFromRequest(r)
	payload, failure, identityConflict, err := creditPayloadForAttempt(
		sessionID,
		token,
		billID,
		posID,
		billRequestID,
		reason,
	)
	if failure != nil {
		writeCreditSourceError(w, failure)
		return
	}
	if identityConflict {
		helpers.WriteErrorResponse(w, http.StatusConflict, nil, "معرّف محاولة الإشعار الدائن مستخدم لفاتورة أخرى")
		return
	}
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(config.BackendDomain, "/")+"/api/v2/bill/credit", bytes.NewBufferString(payload))
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "")
		return
	}
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
	sessionID := helpers.GetSessionIDFromRequest(r)
	loadSettingsFromBackend(sessionID, token)

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
		"whatsapp_enabled": GetSettingValueForSession(sessionID, token, "whatsapp_enabled") == "true",
	}
	helpers.Render(w, r, "invoice-detail", data)
}

// HandleSendInvoiceWhatsApp proxies invoice PDF WhatsApp sending to the backend.
func HandleSendInvoiceWhatsApp(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	id := mux.Vars(r)["id"]
	if strings.TrimSpace(id) == "" {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "رقم الفاتورة غير صالح")
		return
	}

	req, err := http.NewRequest(http.MethodPost, config.BackendDomain+"/api/v2/bill/"+id+"/whatsapp", nil)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusInternalServerError, nil, "تعذر إرسال الفاتورة عبر واتساب")
		return
	}

	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		log.Printf("[WHATSAPP] send failed: %v", err)
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "تعذر الاتصال بخدمة واتساب")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		helpers.HandleUnauthorized(w, r)
		return
	}
	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponseFromBytes(w, resp.StatusCode, body, "تعذر إرسال الفاتورة عبر واتساب")
		return
	}

	msg := helpers.ExtractMessageFromBytes(body)
	if msg == "" || strings.EqualFold(msg, "sent") {
		msg = "تم إرسال الفاتورة عبر واتساب"
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
		"title":               "تعديل الفاتورة",
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

// HandleSubmitDraftInvoice converts a draft bill into a real bill by PUTing to /api/v2/bill/{id}.
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
	prodItems := buildSubmitProductItems(products)
	manualItems := buildSubmitManualItems(manualProducts)

	payload := buildSubmitDraftPayload(inv, extra, prodItems, manualItems)
	payload.PosID = r.FormValue("pos_id")
	payload.BillRequestID = r.FormValue("bill_request_id")
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
			Discount: strconv.FormatFloat(p.Discount, 'f', -1, 64),
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
			Discount:   strconv.FormatFloat(p.Discount, 'f', -1, 64),
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
