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
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Check if it's an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Regular redirect
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

	// For page 1, use empty payload to show drafts (sequence_number=0)
	// For other pages, send page_number to backend for pagination
	var payload map[string]interface{}
	if page == 1 {
		payload = map[string]interface{}{}
		log.Printf("🔵 [API REQUEST] POST %s/api/v2/bill/all (PAGE 1 - INCLUDE DRAFTS)", config.BackendDomain)
	} else {
		payload = map[string]interface{}{"page_number": page}
		log.Printf("🔵 [API REQUEST] POST %s/api/v2/bill/all (PAGE %d)", config.BackendDomain, page)
	}
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
		"page_size": 10000,
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

	// For page 1, use empty payload to show drafts (sequence_number=0)
	// For other pages, send page_number to backend for pagination
	var payload map[string]interface{}
	if page == 1 {
		payload = map[string]interface{}{}
		log.Printf("🔵 [API REQUEST] POST %s/api/v2/purchase_bill/all (PAGE 1 - INCLUDE DRAFTS)", config.BackendDomain)
	} else {
		payload = map[string]interface{}{"page_number": page}
		log.Printf("🔵 [API REQUEST] POST %s/api/v2/purchase_bill/all (PAGE %d)", config.BackendDomain, page)
	}
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
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/product/all", bytes.NewBufferString(`{}`))
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
		qty := 0
		price := 0.0
		if value, ok := CoerceFloat(item["article_id"]); ok {
			id = int(value)
		}
		if id == 0 {
			if value, ok := CoerceFloat(item["id"]); ok {
				id = int(value)
			}
		}
		if value, ok := CoerceFloat(item["quantity"]); ok {
			qty = int(value)
		}
		if value, ok := CoerceFloat(item["price"]); ok {
			price = value
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
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/supplier/all", bytes.NewBufferString(`{}`))
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
	body := []byte(`{"page":0,"page_size":10000}`)
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
	req, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/order/all", bytes.NewBufferString(`{}`))
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
	payload := []byte("{}")
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
