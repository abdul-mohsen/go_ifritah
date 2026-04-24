package helpers

import (
	"afrita/config"
	"afrita/models"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HttpClient is the shared HTTP client for API calls. Exported for test injection.
// Uses an optimized transport with connection pooling and keep-alive so that
// repeated calls to the same backend reuse TCP+TLS connections.
var HttpClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     120 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	},
}

func GetTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	config.SessionTokensMutex.RLock()
	token, exists := config.SessionTokens[cookie.Value]
	config.SessionTokensMutex.RUnlock()
	if !exists {
		return ""
	}
	return token
}

func GetTokenOrRedirect(w http.ResponseWriter, r *http.Request) (string, bool) {
	token := GetTokenFromRequest(r)
	if token == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return "", false
	}
	return token, true
}

func GetSessionIDFromRequest(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func HandleUnauthorized(w http.ResponseWriter, r *http.Request) {
	// Just write 401 — DO NOT delete session cookie or redirect here.
	// The TokenRefreshMiddleware wraps the response, intercepts the 401,
	// attempts a token refresh using the refresh token, and either:
	//   - on success: clears pending headers and issues a redirect so the
	//     browser retries the request with the new token
	//   - on failure: clears the cookie and redirects to login
	// If we delete the cookie or redirect here, the middleware can't
	// recover the session and the user gets logged out unnecessarily.
	w.WriteHeader(http.StatusUnauthorized)
}

func IsUnauthorizedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return StringContains(errStr, "backend status 401") ||
		StringContains(errStr, "unauthorized") ||
		StringContains(errStr, "no session token")
}

func StringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func DoAuthedRequest(req *http.Request, token string) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+token)
	return HttpClient.Do(req)
}

// DoAuthedRequestWithRetry performs an authenticated request and retries with refresh token on 401
func DoAuthedRequestWithRetry(req *http.Request, sessionID string) (*http.Response, error) {
	// Get current access token
	config.SessionTokensMutex.RLock()
	accessToken, hasAccess := config.SessionTokens[sessionID]
	refreshToken, hasRefresh := config.SessionRefreshTokens[sessionID]
	config.SessionTokensMutex.RUnlock()

	if !hasAccess {
		return nil, fmt.Errorf("no session token")
	}

	// Clone the request to retry if needed
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Try with current access token
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// If not 401, return the response
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	resp.Body.Close()

	// Try to refresh the token
	if !hasRefresh || refreshToken == "" {
		return nil, fmt.Errorf("unauthorized - no refresh token")
	}

	// Request new access token using refresh token
	refreshReq, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/refresh", nil)
	refreshReq.Header.Set("Authorization", "Bearer "+refreshToken)
	refreshResp, err := HttpClient.Do(refreshReq)
	if err != nil {
		return nil, fmt.Errorf("refresh failed: %v", err)
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unauthorized - refresh failed")
	}

	var authResp models.AuthResponse
	if err := json.NewDecoder(refreshResp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("refresh decode error: %v", err)
	}

	// Update session with new tokens
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = authResp.AccessToken
	if authResp.RefreshToken != "" {
		config.SessionRefreshTokens[sessionID] = authResp.RefreshToken
	}
	config.SessionTokensMutex.Unlock()

	// Retry original request with new access token
	retryReq, _ := http.NewRequest(req.Method, req.URL.String(), nil)
	if bodyBytes != nil {
		retryReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	for key, values := range req.Header {
		for _, value := range values {
			retryReq.Header.Add(key, value)
		}
	}
	retryReq.Header.Set("Authorization", "Bearer "+authResp.AccessToken)

	return HttpClient.Do(retryReq)
}

func FetchInvoices(token string) ([]models.Invoice, error) {
	return FetchInvoicesAll(token, 1, "")
}

func FetchInvoicesAll(token string, page int, query string) ([]models.Invoice, error) {
	if page < 1 {
		page = 1
	}

	// Always fetch all bills from backend (page_number 0) and paginate client-side
	payload := map[string]interface{}{"page_number": 0, "page_size": 10000}
	log.Printf("🔵 [API REQUEST] POST %s/api/v2/bill/all", config.BackendDomain)
	if query != "" {
		payload["query"] = query
		log.Printf("🔵 [API SEARCH] query=%s", query)
	}
	body, _ := json.Marshal(payload)
	log.Printf("🔵 [API BODY] %s", string(body))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill/all", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("🔴 [API RESPONSE] Status: %d", resp.StatusCode)
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	log.Printf("🟢 [API RESPONSE] Status: 200 OK")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return decodeInvoiceList(bodyBytes)
}

// FetchAllInvoicesUnpaginated fetches ALL invoices without pagination (returns all items)
// Uses page_size=10000 to get maximum data for dashboard analytics
func FetchAllInvoicesUnpaginated(token string) ([]models.Invoice, error) {
	if cached, ok := APICache.Get("invoices_all"); ok {
		if v, ok := cached.([]models.Invoice); ok {
			return v, nil
		}
	}
	payload := map[string]interface{}{
		"page_number": 0,
		"page_size":   10000,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	log.Printf("🔵 [API REQUEST] POST %s/api/v2/bill/all (page_size=10000 for dashboard)", config.BackendDomain)
	log.Printf("🔵 [API BODY] %s", string(body))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill/all", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("🔴 [API RESPONSE] Status: %d", resp.StatusCode)
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	log.Printf("🟢 [API RESPONSE] Status: 200 OK")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	invoices, decErr := decodeInvoiceList(bodyBytes)
	if decErr == nil {
		APICache.Set("invoices_all", invoices, CacheTTLInvoices)
	}
	return invoices, decErr
}

// FetchPurchaseBillsAll fetches purchase bills with smart pagination
// For page 1, returns all bills (including drafts)
// For other pages, sends page_number to backend
func FetchPurchaseBillsAll(token string, page int, query string) ([]models.Invoice, error) {
	if page < 1 {
		page = 1
	}

	// Cache only the default dashboard call (page 1, no query)
	if page == 1 && query == "" {
		if cached, found := APICache.Get("purchase_bills"); found {
			log.Printf("⚡ [CACHE HIT] purchase_bills")
			if v, ok := cached.([]models.Invoice); ok {
				return v, nil
			}
		}
	}

	// Always fetch all purchase bills from backend (page_number 0) and paginate client-side
	payload := map[string]interface{}{"page_number": 0, "page_size": 10000}
	log.Printf("🔵 [API REQUEST] POST %s/api/v2/purchase_bill/all", config.BackendDomain)
	if query != "" {
		payload["query"] = query
		log.Printf("🔵 [API SEARCH] query=%s", query)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	log.Printf("🔵 [API BODY] %s", string(body))

	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/purchase_bill/all", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("🔴 [API RESPONSE] Status: %d", resp.StatusCode)
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	log.Printf("🟢 [API RESPONSE] Status: 200 OK")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Purchase bill list returns numeric fields as strings ("total": "287.5").
	// Decode into raw maps first, then manually coerce into []Invoice.
	result, err := decodePurchaseBillList(bodyBytes)
	if err == nil && page == 1 && query == "" {
		APICache.Set("purchase_bills", result, CacheTTLPurchBill)
		log.Printf("💾 [CACHE SET] purchase_bills (TTL %v)", CacheTTLPurchBill)
	}
	return result, err
}

// FetchInvoicesPaginated fetches invoices with server-side pagination
// page: 1-based page number (default 1)
// perPage: items per page (default 10)
func FetchInvoicesPaginated(token string, page int, perPage int) ([]models.Invoice, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	// Build pagination request payload
	payload := map[string]interface{}{}
	body, _ := json.Marshal(payload)

	// Debug logging
	log.Printf("🔵 [API REQUEST] POST %s/api/v2/bill/all", config.BackendDomain)
	log.Printf("🔵 [API BODY] %s", string(body))

	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill/all", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("🔴 [API RESPONSE] Status: %d", resp.StatusCode)
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	log.Printf("🟢 [API RESPONSE] Status: 200 OK")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(bodyBytes))
	if trimmed == "" || trimmed == "null" {
		return FetchInvoicesFallback(token, page, perPage)
	}

	invoices, err := decodeInvoiceList(bodyBytes)
	if err != nil {
		return nil, err
	}
	return invoices, nil
}

func FetchInvoicesFallback(token string, page int, perPage int) ([]models.Invoice, error) {
	req, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/bill/all", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	invoices, err := decodeInvoiceList(bodyBytes)
	if err != nil {
		return nil, err
	}

	if page > 0 && perPage > 0 {
		paged, _ := PaginateSlice(invoices, page, perPage)
		return paged, nil
	}
	return invoices, nil
}

// FetchBillRaw fetches a single bill by ID and returns ALL raw data from the backend.
func FetchBillRaw(token string, id string) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/bill/"+id, nil)
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// FetchPurchaseBillRaw fetches a single purchase bill by ID and returns ALL raw data.
func FetchPurchaseBillRaw(token string, id string) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/purchase_bill/"+id, nil)
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// FetchPurchaseBillDetail fetches a purchase bill by ID and returns structured data.
func FetchPurchaseBillDetail(token string, id string) (models.Invoice, []models.BillItem, []models.BillItem, map[string]interface{}, error) {
	raw, err := FetchPurchaseBillRaw(token, id)
	if err != nil {
		return models.Invoice{}, nil, nil, nil, err
	}

	inv, products, manualProducts, extra, err := ParseBillRaw(raw, id)
	return inv, products, manualProducts, extra, err
}

// SupplierReportResult holds all data computed for the supplier account statement.
type SupplierReportResult struct {
	Bills          []models.SupplierReportBill
	TopItems       []models.SupplierTopItem
	Summary        models.SupplierBillSummary
	Ledger         []models.LedgerEntry
	Aging          []models.AgingBucket
	PaymentMethods []models.PaymentMethodBreakdown
	Monthly        []models.MonthlySpend
}

// FetchSupplierReport calls the backend supplier report endpoint (كشف حساب).
// GET /api/v2/supplier/:id/report?from=&to=
// Falls back to the old N+1 approach if the new endpoint isn't available (404).
func FetchSupplierReport(token string, supplierID int, dateFrom, dateTo string) (SupplierReportResult, error) {
	var result SupplierReportResult

	// Build URL with query params
	u := fmt.Sprintf("%s/api/v2/supplier/%d/report", config.BackendDomain, supplierID)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return result, fmt.Errorf("build request: %w", err)
	}
	q := req.URL.Query()
	if dateFrom != "" {
		q.Set("from", dateFrom)
	}
	if dateTo != "" {
		q.Set("to", dateTo)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return result, fmt.Errorf("supplier report request: %w", err)
	}
	defer resp.Body.Close()

	// If the endpoint doesn't exist yet or fails, fall back to legacy N+1
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode >= http.StatusInternalServerError {
		log.Printf("⚠️ [SUPPLIER REPORT] Backend endpoint returned %d, using legacy N+1 fetch", resp.StatusCode)
		return fetchSupplierReportLegacy(token, supplierID, dateFrom, dateTo)
	}

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return result, fmt.Errorf("decode response: %w", err)
	}

	now := time.Now()

	// ── Parse summary ────────────────────────────────────────────────
	if sm, ok := raw["summary"].(map[string]interface{}); ok {
		if v, ok := CoerceFloat(sm["bill_count"]); ok {
			result.Summary.BillCount = int(v)
		}
		if v, ok := CoerceFloat(sm["total_spent"]); ok {
			result.Summary.TotalSpent = v
		}
		if v, ok := CoerceFloat(sm["total_before_vat"]); ok {
			result.Summary.TotalBeforeVAT = v
		}
		if v, ok := CoerceFloat(sm["total_vat"]); ok {
			result.Summary.TotalVAT = v
		}
		if v, ok := CoerceFloat(sm["unpaid_total"]); ok {
			result.Summary.UnpaidTotal = v
		}
		if v, ok := CoerceFloat(sm["paid_total"]); ok {
			result.Summary.PaidTotal = v
		}
		if v, ok := CoerceFloat(sm["received_count"]); ok {
			result.Summary.ReceivedCount = int(v)
		}
		if v, ok := CoerceFloat(sm["avg_bill"]); ok {
			result.Summary.AvgBill = v
		}
		if v, ok := CoerceFloat(sm["total_discount"]); ok {
			result.Summary.TotalDiscount = v
		}
		if v, ok := CoerceFloat(sm["total_payments"]); ok {
			result.Summary.TotalPayments = v
		}
		if v, ok := CoerceFloat(sm["payment_count"]); ok {
			result.Summary.PaymentCount = int(v)
		}
		if v, ok := CoerceFloat(sm["closing_balance"]); ok {
			result.Summary.ClosingBalance = v
		}
	}

	// ── Parse bills ──────────────────────────────────────────────────
	if billsRaw, ok := raw["bills"].([]interface{}); ok {
		for _, bRaw := range billsRaw {
			b, ok := bRaw.(map[string]interface{})
			if !ok {
				continue
			}
			rb := models.SupplierReportBill{}
			if v, ok := CoerceFloat(b["id"]); ok {
				rb.ID = int(v)
			}
			if v, ok := CoerceFloat(b["sequence_number"]); ok {
				rb.SequenceNumber = int(v)
			}
			if v, ok := CoerceFloat(b["supplier_sequence_number"]); ok {
				rb.SSN = fmt.Sprintf("%d", int(v))
			} else if v, ok := b["supplier_sequence_number"].(string); ok {
				rb.SSN = v
			}
			if v, ok := CoerceFloat(b["total"]); ok {
				rb.Total = v
			}
			if v, ok := CoerceFloat(b["total_before_vat"]); ok {
				rb.TotalBeforeVAT = v
			}
			if v, ok := CoerceFloat(b["total_vat"]); ok {
				rb.TotalVAT = v
			}
			if v, ok := CoerceFloat(b["discount"]); ok {
				rb.Discount = v
			}
			if v, ok := CoerceFloat(b["state"]); ok {
				rb.State = int(v)
			}
			rb.EffectiveDate = safeStringDate(b["effective_date"])
			rb.PaymentDueDate = safeStringDate(b["payment_due_date"])
			rb.ReceivedAt = safeStringDate(b["received_at"])
			if v, ok := CoerceFloat(b["received_by"]); ok && v > 0 {
				rb.ReceivedBy = fmt.Sprintf("%d", int(v))
			}

			// Compute overdue on the frontend side
			if rb.State == 0 && rb.PaymentDueDate != "" && len(rb.PaymentDueDate) >= 10 {
				if dueDate, err := time.Parse("2006-01-02", rb.PaymentDueDate[:10]); err == nil {
					if now.After(dueDate) {
						rb.IsOverdue = true
						rb.DaysOverdue = int(now.Sub(dueDate).Hours() / 24)
					}
				}
			}

			result.Bills = append(result.Bills, rb)
		}
	}

	// Compute overdue summary from parsed bills
	for _, b := range result.Bills {
		if b.IsOverdue {
			result.Summary.OverdueAmount += b.Total
			result.Summary.OverdueCount++
		}
	}

	// ── Parse payments → build PaymentMethods breakdown ──────────────
	var supplierPayments []models.CashVoucher
	payMethodAgg := map[string]*models.PaymentMethodBreakdown{}
	if paymentsRaw, ok := raw["payments"].([]interface{}); ok {
		for _, pRaw := range paymentsRaw {
			p, ok := pRaw.(map[string]interface{})
			if !ok {
				continue
			}
			cv := models.CashVoucher{}
			if v, ok := CoerceFloat(p["id"]); ok {
				cv.ID = int(v)
			}
			if v, ok := CoerceFloat(p["voucher_number"]); ok {
				cv.VoucherNumber = int(v)
			}
			if v, ok := p["voucher_type"].(string); ok {
				cv.VoucherType = v
			}
			cv.EffectiveDate = safeStringDate(p["effective_date"])
			if v, ok := CoerceFloat(p["amount"]); ok {
				cv.Amount = v
			}
			if v, ok := p["payment_method"].(string); ok {
				cv.PaymentMethod = v
			}
			if v, ok := p["description"].(string); ok {
				cv.Description = v
			}
			supplierPayments = append(supplierPayments, cv)

			// Aggregate by payment method label
			label := cv.PaymentMethod
			if label == "" {
				label = "غير محدد"
			} else if label == "cash" {
				label = "نقدي"
			} else if label == "bank_transfer" {
				label = "تحويل بنكي"
			}
			if _, ok := payMethodAgg[label]; !ok {
				payMethodAgg[label] = &models.PaymentMethodBreakdown{Method: label}
			}
			payMethodAgg[label].Amount += cv.Amount
			payMethodAgg[label].Count++
		}
	}
	// Also add payment_breakdown from backend for cases with no individual payments listed
	if breakdown, ok := raw["payment_breakdown"].(map[string]interface{}); ok {
		if len(payMethodAgg) == 0 {
			if v, ok := CoerceFloat(breakdown["cash_total"]); ok && v > 0 {
				payMethodAgg["نقدي"] = &models.PaymentMethodBreakdown{Method: "نقدي", Amount: v}
			}
			if v, ok := CoerceFloat(breakdown["bank_transfer_total"]); ok && v > 0 {
				payMethodAgg["تحويل بنكي"] = &models.PaymentMethodBreakdown{Method: "تحويل بنكي", Amount: v}
			}
		}
	}
	for _, pm := range payMethodAgg {
		result.PaymentMethods = append(result.PaymentMethods, *pm)
	}

	// ── Parse top items ──────────────────────────────────────────────
	if topRaw, ok := raw["top_items"].([]interface{}); ok {
		for _, tRaw := range topRaw {
			t, ok := tRaw.(map[string]interface{})
			if !ok {
				continue
			}
			item := models.SupplierTopItem{}
			if v, ok := t["item_name"].(string); ok {
				item.Name = v
			}
			if v, ok := CoerceFloat(t["total_qty"]); ok {
				item.TotalQty = int(v)
			}
			if v, ok := CoerceFloat(t["total_value"]); ok {
				item.TotalVal = v
			}
			if v, ok := CoerceFloat(t["avg_price"]); ok {
				item.AvgPrice = v
			}
			if v, ok := CoerceFloat(t["bill_count"]); ok {
				item.BillCount = int(v)
			}
			result.TopItems = append(result.TopItems, item)
		}
	}

	// ── Parse aging ──────────────────────────────────────────────────
	agingLabels := map[string]string{
		"current": "جاري (غير مستحق)",
		"1-30":    "1-30 يوم",
		"31-60":   "31-60 يوم",
		"61-90":   "61-90 يوم",
		"90+":     "أكثر من 90 يوم",
	}
	// Start with all 5 buckets to match template expectations
	agingBuckets := []models.AgingBucket{
		{Label: "جاري (غير مستحق)"},
		{Label: "1-30 يوم"},
		{Label: "31-60 يوم"},
		{Label: "61-90 يوم"},
		{Label: "أكثر من 90 يوم"},
	}
	agingIdx := map[string]int{"current": 0, "1-30": 1, "31-60": 2, "61-90": 3, "90+": 4}
	if agingRaw, ok := raw["aging"].([]interface{}); ok {
		for _, aRaw := range agingRaw {
			a, ok := aRaw.(map[string]interface{})
			if !ok {
				continue
			}
			bucket, _ := a["bucket"].(string)
			if idx, ok := agingIdx[bucket]; ok {
				if v, ok := CoerceFloat(a["bill_count"]); ok {
					agingBuckets[idx].Count = int(v)
				}
				if v, ok := CoerceFloat(a["bucket_total"]); ok {
					agingBuckets[idx].Amount = v
				}
				if label, ok := agingLabels[bucket]; ok {
					agingBuckets[idx].Label = label
				}
			}
		}
	}
	result.Aging = agingBuckets

	// ── Parse monthly spending ───────────────────────────────────────
	monthMap := map[string]*models.MonthlySpend{}
	if monthlyRaw, ok := raw["monthly_spending"].([]interface{}); ok {
		for _, mRaw := range monthlyRaw {
			m, ok := mRaw.(map[string]interface{})
			if !ok {
				continue
			}
			ms := &models.MonthlySpend{}
			if v, ok := m["month"].(string); ok {
				ms.Month = v
			}
			if v, ok := CoerceFloat(m["total_spent"]); ok {
				ms.Amount = v
			}
			monthMap[ms.Month] = ms
		}
	}
	// Add payment amounts per month from parsed vouchers
	for _, cv := range supplierPayments {
		if cv.EffectiveDate != "" && len(cv.EffectiveDate) >= 7 {
			monthKey := cv.EffectiveDate[:7]
			if _, ok := monthMap[monthKey]; !ok {
				monthMap[monthKey] = &models.MonthlySpend{Month: monthKey}
			}
			monthMap[monthKey].Payments += cv.Amount
		}
	}
	for _, m := range monthMap {
		result.Monthly = append(result.Monthly, *m)
	}
	// Sort monthly by month ascending
	for i := 0; i < len(result.Monthly); i++ {
		for j := i + 1; j < len(result.Monthly); j++ {
			if result.Monthly[j].Month < result.Monthly[i].Month {
				result.Monthly[i], result.Monthly[j] = result.Monthly[j], result.Monthly[i]
			}
		}
	}

	// ── Build ledger (still client-side — merges bills + payments chronologically) ──
	result.Ledger = buildSupplierLedger(result.Bills, supplierPayments)

	return result, nil
}

// safeStringDate extracts a date string from a JSON value that may be
// a plain string or a nested {"Time":"...", "Valid":true} object.
func safeStringDate(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if m, ok := v.(map[string]interface{}); ok {
		if t, ok := m["Time"].(string); ok {
			return t
		}
	}
	return ""
}

// fetchSupplierReportLegacy is the old N+1 approach used when the backend
// doesn't have the dedicated supplier report endpoint yet.
func fetchSupplierReportLegacy(token string, supplierID int, dateFrom, dateTo string) (SupplierReportResult, error) {
	var result SupplierReportResult

	allBills, err := FetchPurchaseBillsAll(token, 1, "")
	if err != nil {
		return result, fmt.Errorf("fetch bills: %w", err)
	}

	// Fetch all cash vouchers (payments to supplier)
	allVouchers, err := FetchCashVouchers(token, 1, 10000, "", "")
	if err != nil {
		log.Printf("⚠️ [SUPPLIER REPORT] Could not fetch vouchers: %v", err)
		allVouchers = nil // non-fatal, continue without payment data
	}

	now := time.Now()
	itemAgg := map[string]*models.SupplierTopItem{}
	payMethodAgg := map[int]*models.PaymentMethodBreakdown{}
	monthAgg := map[string]*models.MonthlySpend{}

	// Collect all supplier bills
	for _, bill := range allBills {
		idStr := strconv.Itoa(bill.ID)
		raw, err := FetchPurchaseBillRaw(token, idStr)
		if err != nil {
			continue
		}

		sid := 0
		if v, ok := CoerceFloat(raw["supplier_id"]); ok {
			sid = int(v)
		}
		if sid != supplierID {
			continue
		}

		// Parse effective_date
		effDate := ""
		if ed, ok := raw["effective_date"].(map[string]interface{}); ok {
			if t, ok := ed["Time"].(string); ok {
				effDate = t
			}
		} else if ed, ok := raw["effective_date"].(string); ok {
			effDate = ed
		}

		// Filter by date range
		if effDate != "" && (dateFrom != "" || dateTo != "") {
			d := effDate[:10]
			if dateFrom != "" && d < dateFrom {
				continue
			}
			if dateTo != "" && d > dateTo {
				continue
			}
		}

		inv, products, manualProducts, extra, _ := ParseBillRaw(raw, idStr)

		rb := models.SupplierReportBill{
			ID:             inv.ID,
			SequenceNumber: inv.SequenceNumber,
			Total:          inv.Total,
			TotalBeforeVAT: inv.TotalBeforeVAT,
			TotalVAT:       inv.TotalVAT,
			Discount:       inv.Discount,
			State:          inv.State,
			EffectiveDate:  effDate,
			ItemCount:      len(products) + len(manualProducts),
		}
		if v, ok := extra["supplier_sequence_number"]; ok {
			rb.SSN = fmt.Sprintf("%v", v)
		}
		if v, ok := extra["payment_due_date"]; ok {
			rb.PaymentDueDate = fmt.Sprintf("%v", v)
		}
		if v, ok := extra["deliver_date"]; ok {
			rb.DeliverDate = fmt.Sprintf("%v", v)
		}
		if v, ok := extra["payment_method"]; ok {
			if f, ok2 := CoerceFloat(v); ok2 {
				rb.PaymentMethod = int(f)
			}
		}

		// Check overdue — any bill with a past-due date is overdue
		if rb.PaymentDueDate != "" && len(rb.PaymentDueDate) >= 10 {
			if dueDate, err := time.Parse("2006-01-02", rb.PaymentDueDate[:10]); err == nil {
				if now.After(dueDate) {
					rb.IsOverdue = true
					rb.DaysOverdue = int(now.Sub(dueDate).Hours() / 24)
				}
			}
		}

		result.Bills = append(result.Bills, rb)

		// Aggregate summary
		result.Summary.BillCount++
		result.Summary.TotalSpent += inv.Total
		result.Summary.TotalBeforeVAT += inv.TotalBeforeVAT
		result.Summary.TotalVAT += inv.TotalVAT
		result.Summary.TotalDiscount += inv.Discount
		// PaidTotal and UnpaidTotal are recomputed after voucher processing
		// based on actual payments, not bill state
		if rb.IsOverdue {
			result.Summary.OverdueAmount += inv.Total
			result.Summary.OverdueCount++
		}

		// Payment method aggregation
		pm := rb.PaymentMethod
		if _, ok := payMethodAgg[pm]; !ok {
			payMethodAgg[pm] = &models.PaymentMethodBreakdown{Method: paymentMethodLabel(pm)}
		}
		payMethodAgg[pm].Amount += inv.Total
		payMethodAgg[pm].Count++

		// Monthly aggregation
		if effDate != "" && len(effDate) >= 7 {
			monthKey := effDate[:7] // YYYY-MM
			if _, ok := monthAgg[monthKey]; !ok {
				monthAgg[monthKey] = &models.MonthlySpend{Month: monthKey}
			}
			monthAgg[monthKey].Amount += inv.Total
		}

		// Aggregate items
		allItems := append(products, manualProducts...)
		for _, item := range allItems {
			name := item.PartName
			if name == "" {
				name = item.PartNumber
			}
			if name == "" {
				name = "غير معروف"
			}
			if existing, ok := itemAgg[name]; ok {
				existing.TotalQty += item.Quantity
				existing.TotalVal += item.TotalBeforeVAT
				existing.BillCount++
			} else {
				itemAgg[name] = &models.SupplierTopItem{
					Name:      name,
					TotalQty:  item.Quantity,
					TotalVal:  item.TotalBeforeVAT,
					AvgPrice:  item.Price,
					BillCount: 1,
				}
			}
		}
	}

	// Filter vouchers for this supplier (payments)
	var supplierPayments []models.CashVoucher
	for _, v := range allVouchers {
		if v.RecipientType != "supplier" || v.RecipientID != supplierID {
			continue
		}
		// Parse voucher date for range filter
		vDate := ""
		if len(v.EffectiveDate) >= 10 {
			vDate = v.EffectiveDate[:10]
		}
		if vDate != "" {
			if dateFrom != "" && vDate < dateFrom {
				continue
			}
			if dateTo != "" && vDate > dateTo {
				continue
			}
		}
		supplierPayments = append(supplierPayments, v)
		result.Summary.TotalPayments += v.Amount
		result.Summary.PaymentCount++

		// Monthly payment aggregation
		if vDate != "" && len(vDate) >= 7 {
			monthKey := vDate[:7]
			if _, ok := monthAgg[monthKey]; !ok {
				monthAgg[monthKey] = &models.MonthlySpend{Month: monthKey}
			}
			monthAgg[monthKey].Payments += v.Amount
		}
	}

	// Closing balance = total bills - total payments
	result.Summary.ClosingBalance = result.Summary.TotalSpent - result.Summary.TotalPayments

	// PaidTotal/UnpaidTotal based on actual payments, not bill state
	result.Summary.PaidTotal = result.Summary.TotalPayments
	result.Summary.UnpaidTotal = result.Summary.ClosingBalance
	if result.Summary.UnpaidTotal < 0 {
		result.Summary.UnpaidTotal = 0
	}

	// Compute averages
	if result.Summary.BillCount > 0 {
		result.Summary.AvgBill = result.Summary.TotalSpent / float64(result.Summary.BillCount)
	}

	// Build ledger (chronological account statement)
	result.Ledger = buildSupplierLedger(result.Bills, supplierPayments)

	// Build aging buckets
	result.Aging = buildAgingBuckets(result.Bills, now)

	// Sort top items by TotalVal descending, take top 20
	topItems := make([]models.SupplierTopItem, 0, len(itemAgg))
	for _, item := range itemAgg {
		if item.TotalQty > 0 {
			item.AvgPrice = item.TotalVal / float64(item.TotalQty)
		}
		topItems = append(topItems, *item)
	}
	for i := 0; i < len(topItems); i++ {
		for j := i + 1; j < len(topItems); j++ {
			if topItems[j].TotalVal > topItems[i].TotalVal {
				topItems[i], topItems[j] = topItems[j], topItems[i]
			}
		}
	}
	if len(topItems) > 20 {
		topItems = topItems[:20]
	}
	result.TopItems = topItems

	// Payment methods
	for _, pm := range payMethodAgg {
		result.PaymentMethods = append(result.PaymentMethods, *pm)
	}

	// Monthly spending (sorted by month)
	for _, m := range monthAgg {
		result.Monthly = append(result.Monthly, *m)
	}
	for i := 0; i < len(result.Monthly); i++ {
		for j := i + 1; j < len(result.Monthly); j++ {
			if result.Monthly[j].Month < result.Monthly[i].Month {
				result.Monthly[i], result.Monthly[j] = result.Monthly[j], result.Monthly[i]
			}
		}
	}

	return result, nil
}

// paymentMethodLabel converts a payment method int to Arabic label.
func paymentMethodLabel(pm int) string {
	switch pm {
	case 10:
		return "نقدي"
	case 30:
		return "بطاقة ائتمان"
	case 42:
		return "تحويل بنكي"
	case 48:
		return "آجل"
	default:
		if pm == 0 {
			return "غير محدد"
		}
		return fmt.Sprintf("طريقة %d", pm)
	}
}

// buildSupplierLedger builds a chronological ledger from bills + payments.
func buildSupplierLedger(bills []models.SupplierReportBill, payments []models.CashVoucher) []models.LedgerEntry {
	var entries []models.LedgerEntry

	// Add bill entries
	for _, b := range bills {
		date := ""
		if len(b.EffectiveDate) >= 10 {
			date = b.EffectiveDate[:10]
		}
		ref := fmt.Sprintf("PB-%d", b.SequenceNumber)
		if b.SSN != "" {
			ref = b.SSN
		}
		entries = append(entries, models.LedgerEntry{
			Date:        date,
			Type:        "bill",
			Reference:   ref,
			Description: fmt.Sprintf("فاتورة مشتريات #%d", b.SequenceNumber),
			Debit:       b.Total,
			LinkURL:     fmt.Sprintf("/dashboard/purchase-bills/%d", b.ID),
		})
	}

	// Add payment entries
	for _, p := range payments {
		date := ""
		if len(p.EffectiveDate) >= 10 {
			date = p.EffectiveDate[:10]
		}
		ref := fmt.Sprintf("CV-%d", p.VoucherNumber)
		desc := "سند صرف"
		if p.Description != "" {
			desc = p.Description
		}
		entries = append(entries, models.LedgerEntry{
			Date:        date,
			Type:        "payment",
			Reference:   ref,
			Description: desc,
			Credit:      p.Amount,
			LinkURL:     fmt.Sprintf("/dashboard/cash-vouchers/%d", p.ID),
		})
	}

	// Sort by date ascending
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Date < entries[i].Date {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Calculate running balance
	balance := 0.0
	for i := range entries {
		balance += entries[i].Debit - entries[i].Credit
		entries[i].Balance = balance
	}

	return entries
}

// buildAgingBuckets categorizes unpaid bills into aging periods.
func buildAgingBuckets(bills []models.SupplierReportBill, now time.Time) []models.AgingBucket {
	buckets := []models.AgingBucket{
		{Label: "جاري (غير مستحق)"},
		{Label: "1-30 يوم"},
		{Label: "31-60 يوم"},
		{Label: "61-90 يوم"},
		{Label: "أكثر من 90 يوم"},
	}

	for _, b := range bills {
		// Include all bills with a due date in aging analysis
		if b.PaymentDueDate == "" || len(b.PaymentDueDate) < 10 {
			buckets[0].Amount += b.Total
			buckets[0].Count++
			continue
		}
		dueDate, err := time.Parse("2006-01-02", b.PaymentDueDate[:10])
		if err != nil {
			buckets[0].Amount += b.Total
			buckets[0].Count++
			continue
		}
		days := int(now.Sub(dueDate).Hours() / 24)
		switch {
		case days <= 0:
			buckets[0].Amount += b.Total
			buckets[0].Count++
		case days <= 30:
			buckets[1].Amount += b.Total
			buckets[1].Count++
		case days <= 60:
			buckets[2].Amount += b.Total
			buckets[2].Count++
		case days <= 90:
			buckets[3].Amount += b.Total
			buckets[3].Count++
		default:
			buckets[4].Amount += b.Total
			buckets[4].Count++
		}
	}

	return buckets
}

// FetchBillDetail fetches a single bill by ID and returns an Invoice with product details.
// The backend detail endpoint returns some numeric fields as strings (e.g. total, total_vat),
// so we decode into map[string]interface{} and convert manually using CoerceFloat.
func FetchBillDetail(token string, id string) (models.Invoice, []models.BillItem, []models.BillItem, map[string]interface{}, error) {
	raw, err := FetchBillRaw(token, id)
	if err != nil {
		return models.Invoice{}, nil, nil, nil, err
	}

	return ParseBillRaw(raw, id)
}

// parseBillItems converts a raw interface{} into []BillItem.
// Handles two backend formats:
//   - JSON array ([]interface{}) — regular bills
//   - Base64-encoded JSON string — purchase bills
func parseBillItems(raw interface{}) []models.BillItem {
	var arr []interface{}
	switch v := raw.(type) {
	case []interface{}:
		arr = v
	case string:
		// Purchase bills return products as base64-encoded JSON
		if v == "" {
			return nil
		}
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			log.Printf("[parseBillItems] base64 decode error: %v", err)
			return nil
		}
		if err := json.Unmarshal(decoded, &arr); err != nil {
			log.Printf("[parseBillItems] JSON unmarshal error: %v", err)
			return nil
		}
	default:
		return nil
	}
	if len(arr) == 0 {
		return nil
	}
	items := make([]models.BillItem, 0, len(arr))
	for _, elem := range arr {
		m, ok := elem.(map[string]interface{})
		if !ok {
			continue
		}
		item := models.BillItem{}
		if v, ok := CoerceFloat(m["product_id"]); ok {
			item.ProductID = int(v)
		}
		if v, ok := m["name"].(string); ok {
			item.PartName = v
		}
		if v, ok := m["part_name"].(string); ok {
			item.PartNumber = v
		}
		if v, ok := CoerceFloat(m["price"]); ok {
			item.Price = v
		}
		if v, ok := CoerceFloat(m["quantity"]); ok {
			item.Quantity = int(v)
		}
		if v, ok := CoerceFloat(m["discount"]); ok {
			item.Discount = v
		}
		if v, ok := CoerceFloat(m["total_before_vat"]); ok {
			item.TotalBeforeVAT = v
		}
		items = append(items, item)
	}
	return items
}

// SumBillItemsTotal calculates the total of all bill items (price * quantity) and returns a formatted string.
func SumBillItemsTotal(items []models.BillItem) string {
	total := 0.0
	for _, item := range items {
		if item.TotalBeforeVAT > 0 {
			total += item.TotalBeforeVAT
		} else {
			total += item.Price * float64(item.Quantity)
		}
	}
	return fmt.Sprintf("%.2f", total)
}

// ParseBillItemsPublic is the exported version of parseBillItems for use by handlers.
func ParseBillItemsPublic(raw interface{}) []models.BillItem {
	return parseBillItems(raw)
}

// FetchCreditBillDetail fetches a credit bill by ID using the same robust parsing.
func FetchCreditBillDetail(token string, id string) (models.Invoice, []models.BillItem, []models.BillItem, map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/credit_bill/"+id, nil)
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return models.Invoice{}, nil, nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.Invoice{}, nil, nil, nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return models.Invoice{}, nil, nil, nil, err
	}

	return ParseBillRaw(raw, id)
}

// ParseBillRaw converts a raw bill map into an Invoice plus products and extras.
func ParseBillRaw(raw map[string]interface{}, id string) (models.Invoice, []models.BillItem, []models.BillItem, map[string]interface{}, error) {
	inv := models.Invoice{}
	if v, ok := CoerceFloat(raw["id"]); ok {
		inv.ID = int(v)
	}
	if inv.ID == 0 {
		inv.ID = ParseIntValue(id)
	}
	if v, ok := CoerceFloat(raw["sequence_number"]); ok {
		inv.SequenceNumber = int(v)
	}
	if v, ok := CoerceFloat(raw["state"]); ok {
		inv.State = int(v)
	}
	if v, ok := CoerceFloat(raw["credit_state"]); ok {
		inv.CreditState = int(v)
	}
	if v, ok := CoerceFloat(raw["total"]); ok {
		inv.Total = v
	}
	if v, ok := CoerceFloat(raw["subtotal"]); ok {
		inv.Subtotal = v
	}
	if v, ok := CoerceFloat(raw["total_vat"]); ok {
		inv.TotalVAT = v
	}
	if v, ok := CoerceFloat(raw["total_before_vat"]); ok {
		inv.TotalBeforeVAT = v
	}
	if v, ok := CoerceFloat(raw["discount"]); ok {
		inv.Discount = v
	}
	if v, ok := CoerceFloat(raw["vat"]); ok {
		inv.VAT = v
	}
	if v, ok := CoerceFloat(raw["bill_type"]); ok {
		inv.Type = v == 1
	} else if t, ok := raw["bill_type"].(bool); ok {
		inv.Type = t
	} else if t, ok := raw["type"].(bool); ok {
		inv.Type = t
	}
	if ed, ok := raw["effective_date"].(map[string]interface{}); ok {
		if t, ok := ed["Time"].(string); ok {
			inv.EffectiveDate.Time = t
			inv.EffectiveDate.Valid = true
		}
	} else if ed, ok := raw["effective_date"].(string); ok && ed != "" {
		inv.EffectiveDate.Time = ed
		inv.EffectiveDate.Valid = true
	}

	products := parseBillItems(raw["products"])
	manualProducts := parseBillItems(raw["manual_products"])

	extra := map[string]interface{}{}
	for _, key := range []string{
		"store_id", "store_name", "merchant_id", "company_name",
		"address", "vat_registration", "CommercialRegistrationNumber",
		"user_name", "user_phone_number", "note",
		"maintenance_cost", "url", "credit_note", "qr_code",
		"supplier_id", "supplier_sequence_number",
		"payment_method", "branch_id", "branch_name", "deliver_date",
		"pdf_link", "attachments",
	} {
		if v, exists := raw[key]; exists {
			extra[key] = v
		}
	}

	// Also parse payment_due_date (backend may return nested object or plain string)
	if pdd, ok := raw["payment_due_date"].(map[string]interface{}); ok {
		if t, ok := pdd["Time"].(string); ok {
			extra["payment_due_date"] = t
		}
	} else if pdd, ok := raw["payment_due_date"].(string); ok && pdd != "" {
		extra["payment_due_date"] = pdd
	}

	// Parse deliver_date (may be nested or plain string)
	if dd, ok := raw["deliver_date"].(map[string]interface{}); ok {
		if t, ok := dd["Time"].(string); ok {
			extra["deliver_date"] = t
		}
	} else if dd, ok := raw["deliver_date"].(string); ok && dd != "" {
		extra["deliver_date"] = dd
	}

	return inv, products, manualProducts, extra, nil
}

func FetchProducts(token string) ([]models.Product, error) {
	if cached, ok := APICache.Get("products"); ok {
		if v, ok := cached.([]models.Product); ok {
			return v, nil
		}
	}
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/product/all", bytes.NewBufferString(`{"page_number":0,"page_size":10000}`))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	products, err := decodeListResponse[models.Product](bodyBytes)
	if err == nil {
		for i, p := range products {
			if p.ID == 0 && p.PartID > 0 {
				products[i].ID = p.PartID
			}
			// Use Name from backend as fallback for PartName
			if products[i].PartName == "" && products[i].Name != "" {
				products[i].PartName = products[i].Name
			}
		}
		APICache.Set("products", products, CacheTTLProducts)
		return products, nil
	}

	items, itemErr := decodeListResponse[map[string]interface{}](bodyBytes)
	if itemErr != nil {
		return nil, err
	}

	converted := make([]models.Product, 0, len(items))
	for _, item := range items {
		id := 0
		qty := ""
		price := ""
		if value, ok := CoerceFloat(item["article_id"]); ok {
			id = int(value)
		}
		if id == 0 {
			if value, ok := CoerceFloat(item["id"]); ok {
				id = int(value)
			}
		}
		if value, ok := item["quantity"].(string); ok {
			qty = value
		} else if value, ok := CoerceFloat(item["quantity"]); ok {
			qty = fmt.Sprintf("%g", value)
		}
		if value, ok := item["price"].(string); ok {
			price = value
		} else if value, ok := CoerceFloat(item["price"]); ok {
			price = fmt.Sprintf("%g", value)
		}
		partName := ""
		if value, ok := item["part_name"].(string); ok {
			partName = value
		}
		// Fallback: use "name" field from backend
		if partName == "" {
			if value, ok := item["name"].(string); ok {
				partName = value
			}
		}
		costPrice := ""
		if value, ok := item["cost_price"].(string); ok {
			costPrice = value
		}

		shelfNumber := ""
		if value, ok := item["shelf_number"].(string); ok {
			shelfNumber = value
		}
		storeID := 0
		if value, ok := CoerceFloat(item["store_id"]); ok {
			storeID = int(value)
		}
		converted = append(converted, models.Product{ID: id, PartName: partName, Quantity: qty, Price: price, CostPrice: costPrice, ShelfNumber: shelfNumber, StoreID: storeID})
	}

	APICache.Set("products", converted, CacheTTLProducts)
	return converted, nil
}

// FetchPartNames fetches all parts from the backend and returns a map of part ID → OEM number (part name).
func FetchPartNames(token string) (map[int]string, error) {
	if cached, ok := APICache.Get("part_names"); ok {
		if v, ok := cached.(map[int]string); ok {
			return v, nil
		}
	}

	payload, _ := json.Marshal(map[string]string{"query": ""})
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/part/", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("parts API status %d", resp.StatusCode)
	}

	var parts []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&parts); err != nil {
		return nil, err
	}

	nameMap := make(map[int]string, len(parts))
	for _, p := range parts {
		id := 0
		if v, ok := CoerceFloat(p["id"]); ok {
			id = int(v)
		}
		oem := ""
		if v, ok := p["oem_number"].(string); ok {
			oem = v
		}
		if id > 0 && oem != "" {
			nameMap[id] = oem
		}
	}

	APICache.Set("part_names", nameMap, CacheTTLProducts)
	return nameMap, nil
}

// EnrichProductPartNames resolves article_id → oem_number for products that have no PartName set.
func EnrichProductPartNames(products []models.Product, token string) {
	// Check if any product needs a name
	needsLookup := false
	for _, p := range products {
		if p.PartName == "" && p.PartID > 0 {
			needsLookup = true
			break
		}
	}
	if !needsLookup {
		return
	}

	nameMap, err := FetchPartNames(token)
	if err != nil {
		log.Printf("[ENRICH PARTS] Failed to fetch part names: %v", err)
		return
	}

	for i := range products {
		if products[i].PartName == "" && products[i].PartID > 0 {
			if name, ok := nameMap[products[i].PartID]; ok {
				products[i].PartName = name
			}
		}
	}
}

func FetchSuppliers(token string) ([]models.Supplier, error) {
	if cached, ok := APICache.Get("suppliers"); ok {
		if v, ok := cached.([]models.Supplier); ok {
			return v, nil
		}
	}
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/supplier/all", bytes.NewBufferString(`{"page_number":0,"page_size":10000}`))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	suppliers, err := decodeSupplierList(bodyBytes)
	if err != nil {
		return nil, err
	}
	APICache.Set("suppliers", suppliers, CacheTTLSuppliers)
	return suppliers, nil
}

func decodeSupplierList(body []byte) ([]models.Supplier, error) {
	var rawList []map[string]interface{}
	if err := json.Unmarshal(body, &rawList); err != nil {
		var wrapper map[string]json.RawMessage
		if wErr := json.Unmarshal(body, &wrapper); wErr != nil {
			return nil, err
		}
		for _, key := range []string{"data", "items", "results", "suppliers"} {
			if raw, ok := wrapper[key]; ok {
				if json.Unmarshal(raw, &rawList) == nil {
					break
				}
			}
		}
		if rawList == nil {
			return nil, fmt.Errorf("unsupported response shape for suppliers")
		}
	}

	suppliers := make([]models.Supplier, 0, len(rawList))
	for _, m := range rawList {
		s := models.Supplier{}
		if v, ok := CoerceFloat(m["id"]); ok {
			s.ID = int(v)
		}
		if v, ok := m["name"].(string); ok {
			s.Name = v
		}
		if v, ok := m["email"].(string); ok {
			s.Email = v
		}
		if v, ok := m["address"].(string); ok {
			s.Address = v
		}
		if v, ok := m["short_address"].(string); ok {
			s.ShortAddress = v
		}
		if v, ok := m["phone_number"].(string); ok {
			s.PhoneNumber = v
		}
		if v, ok := m["number"].(string); ok {
			s.Number = v
		}
		if v, ok := m["vat_number"].(string); ok {
			s.VATNumber = v
		}
		if v, ok := m["commercial_registration"].(string); ok {
			s.CR = v
		}
		if v, ok := m["bank_account"].(string); ok {
			s.BankAccount = v
		}
		if v, ok := CoerceFloat(m["preferred_payment_method"]); ok {
			s.PreferredPaymentMethod = int(v)
		}
		if v, ok := m["is_post_paid"].(bool); ok {
			s.IsPostPaid = v
		}
		if v, ok := CoerceFloat(m["payment_terms_days"]); ok {
			s.PaymentTermsDays = int(v)
		}
		if v, ok := CoerceFloat(m["credit_limit"]); ok {
			s.CreditLimit = int(v)
		}
		if v, ok := m["created_at"].(string); ok {
			s.CreatedAt = v
		}
		if v, ok := m["updated_at"].(string); ok {
			s.UpdatedAt = v
		}
		suppliers = append(suppliers, s)
	}
	return suppliers, nil
}

func FetchClients(token string) ([]models.Client, error) {
	if cached, ok := APICache.Get("clients"); ok {
		if v, ok := cached.([]models.Client); ok {
			return v, nil
		}
	}
	body := []byte(`{"page_number":0,"page_size":10000}`)
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/client/all", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	clients, err := decodeListResponse[models.Client](bodyBytes)
	if err != nil {
		return nil, err
	}
	APICache.Set("clients", clients, CacheTTLClients)
	return clients, nil
}

// FetchClientByID fetches a single client by ID from the backend.
func FetchClientByID(token string, id string) (models.Client, error) {
	req, err := http.NewRequest("GET", config.BackendDomain+"/api/v2/client/"+id, nil)
	if err != nil {
		return models.Client{}, fmt.Errorf("new request: %w", err)
	}
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return models.Client{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.Client{}, fmt.Errorf("backend status %d", resp.StatusCode)
	}
	var client models.Client
	if err := json.NewDecoder(resp.Body).Decode(&client); err != nil {
		return models.Client{}, fmt.Errorf("decode client: %w", err)
	}
	return client, nil
}

func FetchOrders(token string) ([]map[string]interface{}, error) {
	if cached, ok := APICache.Get("orders"); ok {
		if v, ok := cached.([]map[string]interface{}); ok {
			return v, nil
		}
	}
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/order/all", bytes.NewBufferString(`{"page_number":0,"page_size":10000}`))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	orders, err := decodeListResponse[map[string]interface{}](bodyBytes)
	if err != nil {
		return nil, err
	}
	APICache.Set("orders", orders, CacheTTLOrders)
	return orders, nil
}

// FetchOrderDetail retrieves a single order by ID from the backend.
// Endpoint: GET /api/v2/order/:id
func FetchOrderDetail(token string, id string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", config.BackendDomain+"/api/v2/order/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func FetchStores(token string) ([]models.Store, error) {
	if cached, ok := APICache.Get("stores"); ok {
		if v, ok := cached.([]models.Store); ok {
			return v, nil
		}
	}
	req, err := http.NewRequest("GET", config.BackendDomain+"/api/v2/stores/all", nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	stores, err := decodeListResponse[models.Store](bodyBytes)
	if err != nil {
		return nil, err
	}
	APICache.Set("stores", stores, CacheTTLStores)
	return stores, nil
}

// FetchBranches retrieves the list of branches from the backend.
// Endpoint: POST /api/v2/branch/all
func FetchBranches(token string) ([]models.Branch, error) {
	if cached, ok := APICache.Get("branches"); ok {
		if v, ok := cached.([]models.Branch); ok {
			return v, nil
		}
	}
	payload := []byte(`{"page_number":0,"page_size":10000}`)
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/branch/all", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	branches, err := decodeListResponse[models.Branch](bodyBytes)
	if err != nil {
		return nil, err
	}
	APICache.Set("branches", branches, CacheTTLBranches)
	return branches, nil
}

// decodeInvoiceList parses the invoice/bill list response where numeric
// fields (total, subtotal, total_before_vat, total_vat, discount, vat) may come as strings.
func decodeInvoiceList(body []byte) ([]models.Invoice, error) {
	var rawList []map[string]interface{}
	if err := json.Unmarshal(body, &rawList); err != nil {
		var wrapper map[string]json.RawMessage
		if wErr := json.Unmarshal(body, &wrapper); wErr != nil {
			return nil, err
		}
		for _, key := range []string{"data", "items", "results", "bills", "invoices"} {
			if raw, ok := wrapper[key]; ok {
				if json.Unmarshal(raw, &rawList) == nil {
					break
				}
			}
		}
		if rawList == nil {
			return nil, fmt.Errorf("unsupported response shape for invoices")
		}
	}

	invoices := make([]models.Invoice, 0, len(rawList))
	for _, m := range rawList {
		inv := models.Invoice{}
		if v, ok := CoerceFloat(m["id"]); ok {
			inv.ID = int(v)
		}
		if v, ok := CoerceFloat(m["sequence_number"]); ok {
			inv.SequenceNumber = int(v)
		}
		if v, ok := CoerceFloat(m["subtotal"]); ok {
			inv.Subtotal = v
		}
		if v, ok := CoerceFloat(m["total"]); ok {
			inv.Total = v
		}
		if v, ok := CoerceFloat(m["total_vat"]); ok {
			inv.TotalVAT = v
		}
		if v, ok := CoerceFloat(m["total_before_vat"]); ok {
			inv.TotalBeforeVAT = v
		}
		if v, ok := CoerceFloat(m["discount"]); ok {
			inv.Discount = v
		}
		if v, ok := CoerceFloat(m["vat"]); ok {
			inv.VAT = v
		}
		if v, ok := CoerceFloat(m["state"]); ok {
			inv.State = int(v)
		}
		if v, ok := CoerceFloat(m["credit_state"]); ok {
			inv.CreditState = int(v)
		}
		if v, ok := m["bill_type"].(bool); ok {
			inv.Type = v
		} else if v, ok := m["type"].(bool); ok {
			inv.Type = v
		}

		// Parse effective_date — may be string or {Time, Valid} object
		if ed, ok := m["effective_date"].(string); ok {
			inv.EffectiveDate.Time = ed
			inv.EffectiveDate.Valid = ed != ""
		} else if edMap, ok := m["effective_date"].(map[string]interface{}); ok {
			if t, ok := edMap["Time"].(string); ok {
				inv.EffectiveDate.Time = t
			}
			if v, ok := edMap["Valid"].(bool); ok {
				inv.EffectiveDate.Valid = v
			}
		}

		inv.PaymentDueDate = m["payment_due_date"]
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

// decodePurchaseBillList parses the purchase bill list response where numeric
// fields (total, total_before_vat, total_vat, discount) may come as strings.
func decodePurchaseBillList(body []byte) ([]models.Invoice, error) {
	var rawList []map[string]interface{}
	if err := json.Unmarshal(body, &rawList); err != nil {
		// Try wrapper format
		var wrapper map[string]json.RawMessage
		if wErr := json.Unmarshal(body, &wrapper); wErr != nil {
			return nil, err
		}
		for _, key := range []string{"data", "items", "results", "bills"} {
			if raw, ok := wrapper[key]; ok {
				if json.Unmarshal(raw, &rawList) == nil {
					break
				}
			}
		}
		if rawList == nil {
			return nil, fmt.Errorf("unsupported response shape for purchase bills")
		}
	}

	invoices := make([]models.Invoice, 0, len(rawList))
	for _, m := range rawList {
		inv := models.Invoice{}
		if v, ok := CoerceFloat(m["id"]); ok {
			inv.ID = int(v)
		}
		if v, ok := CoerceFloat(m["sequence_number"]); ok {
			inv.SequenceNumber = int(v)
		}
		if v, ok := CoerceFloat(m["supplier_sequence_number"]); ok {
			inv.SupplierSequenceNumber = int(v)
		}
		if v, ok := CoerceFloat(m["total"]); ok {
			inv.Total = v
		}
		if v, ok := CoerceFloat(m["total_vat"]); ok {
			inv.TotalVAT = v
		}
		if v, ok := CoerceFloat(m["total_before_vat"]); ok {
			inv.TotalBeforeVAT = v
		}
		if v, ok := CoerceFloat(m["discount"]); ok {
			inv.Discount = v
		}
		if v, ok := CoerceFloat(m["vat"]); ok {
			inv.VAT = v
		}
		if v, ok := CoerceFloat(m["state"]); ok {
			inv.State = int(v)
		}
		if v, ok := CoerceFloat(m["credit_state"]); ok {
			inv.CreditState = int(v)
		}
		if v, ok := m["bill_type"].(bool); ok {
			inv.Type = v
		} else if v, ok := m["type"].(bool); ok {
			inv.Type = v
		}

		// Parse effective_date — may be string or {Time, Valid} object
		if ed, ok := m["effective_date"].(string); ok {
			inv.EffectiveDate.Time = ed
			inv.EffectiveDate.Valid = ed != ""
		} else if edMap, ok := m["effective_date"].(map[string]interface{}); ok {
			if t, ok := edMap["Time"].(string); ok {
				inv.EffectiveDate.Time = t
			}
			if v, ok := edMap["Valid"].(bool); ok {
				inv.EffectiveDate.Valid = v
			}
		}

		inv.PaymentDueDate = m["payment_due_date"]
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

func decodeListResponse[T any](body []byte) ([]T, error) {
	var list []T
	if err := json.Unmarshal(body, &list); err == nil {
		return list, nil
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}

	keys := []string{
		"data",
		"items",
		"results",
		"bills",
		"invoices",
		"products",
		"suppliers",
		"clients",
		"orders",
	}
	for _, key := range keys {
		if raw, ok := wrapper[key]; ok {
			var list []T
			if err := json.Unmarshal(raw, &list); err == nil {
				return list, nil
			}
		}
	}

	return nil, fmt.Errorf("unsupported response shape")
}

// statusClassCreditNote is the CSS class for credit note (إشعار دائن) statuses.
const statusClassCreditNote = "badge-processing" //nolint:gosec // G101 false positive: CSS class, not credentials

func InvoiceStatus(inv models.Invoice) (string, string) {
	if inv.CreditState >= 1 {
		switch inv.CreditState {
		case 1:
			return "إشعار دائن قيد المعالجة", statusClassCreditNote
		case 2:
			return "إشعار دائن تمت المعالجة", statusClassCreditNote
		case 3:
			return "إشعار دائن صادرة", statusClassCreditNote
		default:
			return "ERROR", "badge-danger"
		}
	}

	switch inv.State {
	case 0:
		return "مسودة", "badge-draft"
	case 1:
		return "قيد المعالجة", "badge-pending"
	case 2:
		return "تمت المعالجة", "badge-issued"
	case 3:
		return "صادرة", "badge-issued"
	default:
		return "ERROR", "badge-danger"
	}
}

func TranslateInvoiceStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "draft":
		return "مسودة"
	case "bill is under process":
		return "قيد المعالجة"
	case "bill is processed":
		return "تمت المعالجة"
	case "bill is issued to zatca":
		return "صادرة"
	case "credit is under process":
		return "إشعار دائن قيد المعالجة"
	case "credit is processed":
		return "إشعار دائن تمت المعالجة"
	case "credit is issued to zatca":
		return "إشعار دائن صادرة"
	default:
		return status
	}
}

func InvoiceTypeLabel(inv models.Invoice) string {
	if inv.CreditState >= 1 {
		return "إشعار دائن"
	}
	if inv.Type {
		return "فاتورة ضريبية"
	}
	return "فاتورة مبسطة"
}

// GetUserRole returns the role of the current user from the session.
//
// TODO (backend): The real backend should return role info in the login/refresh
// response. Store the role in config.SessionUserRoles[sessionID] alongside the
// token. Then this function should look up the stored role.
//
// Backend changes needed:
//   - POST /api/v2/login response should include: "role": "admin"|"manager"|"employee"
//     File: helpers/api_helpers.go (login handler) — store role after successful login
//   - POST /api/v2/refresh response should include role as well
//     File: helpers/api_helpers.go (DoAuthedRequestWithRetry) — update stored role on refresh
//   - models/types.go AuthResponse — add Role field
//
// For now, returns "admin" for all authenticated sessions.
func GetUserRole(r *http.Request) string {
	token := GetTokenFromRequest(r)
	if token == "" {
		return ""
	}
	// TODO: look up config.SessionUserRoles[sessionID] once backend provides role
	return "admin"
}
