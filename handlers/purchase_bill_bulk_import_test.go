package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

func TestHandleBulkPurchaseBillImportForwardsRowsAndPerItemErrors(t *testing.T) {
	var received []models.PurchaseBillPayload
	backendCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/purchase_bill" {
			t.Errorf("backend request = %s %s, want POST /api/v2/purchase_bill", r.Method, r.URL.Path)
		}
		var payload models.PurchaseBillPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode backend payload: %v", err)
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		received = append(received, payload)
		if backendCalls == 2 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":"supplier sequence already used"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer backend.Close()

	oldBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = oldBackend })
	cleanup := setupPBTestSession("bulk-purchase-forward-test", "bulk-purchase-token")
	t.Cleanup(cleanup)

	body := mustMarshalPurchaseBulkPayload(t, []map[string]interface{}{
		validBulkPurchaseBillPayload(),
		validBulkPurchaseBillPayload(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills/bulk-import", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "bulk-purchase-forward-test"})
	w := httptest.NewRecorder()

	HandleBulkPurchaseBillImport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var result bulkImportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode bulk response: %v", err)
	}
	if result.Total != 2 || result.Success != 1 || result.Failed != 1 || len(result.Results) != 2 {
		t.Fatalf("result = %+v, want total=2 success=1 failed=1", result)
	}
	if !result.Results[0].OK || result.Results[0].Index != 0 {
		t.Errorf("first result = %+v, want successful index 0", result.Results[0])
	}
	if result.Results[1].OK || result.Results[1].Index != 1 ||
		!strings.Contains(result.Results[1].Error, "422") ||
		!strings.Contains(result.Results[1].Error, "supplier sequence already used") {
		t.Errorf("second result = %+v, want backend 422 error for index 1", result.Results[1])
	}
	if backendCalls != 2 || len(received) != 2 {
		t.Fatalf("backend calls = %d, decoded payloads = %d; want 2", backendCalls, len(received))
	}

	got := received[0]
	if got.StoreID != 5 || got.SupplierID != 7 || got.MerchantID != 7 ||
		got.SupplierSequenceNumber != 1001 || got.PaymentMethod != 10 ||
		got.EffectiveDate != "2026-04-10T00:00:00+03:00" || got.State != 1 {
		t.Errorf("forwarded bill fields = %+v, want backend-required supplier/payment/date values", got)
	}
	if len(got.ManualProducts) != 1 {
		t.Fatalf("manual_products = %d, want 1", len(got.ManualProducts))
	}
	product := got.ManualProducts[0]
	if product.PartName != "Oil filter" || product.Price != "25" ||
		product.Quantity != "2" || product.CostPrice != "20.5" || product.ProductID != nil {
		t.Errorf("forwarded manual product = %+v, want name/price/quantity/cost_price and null product_id", product)
	}
}

func TestHandleBulkPurchaseBillImportCacheInvalidation(t *testing.T) {
	tests := []struct {
		name         string
		successFirst bool
		wantSuccess  int
		wantCached   bool
	}{
		{name: "partial success invalidates", successFirst: true, wantSuccess: 1},
		{name: "zero successes preserve cache", wantSuccess: 0, wantCached: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			backendCalls := 0
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backendCalls++
				if testCase.successFirst && backendCalls == 1 {
					w.WriteHeader(http.StatusCreated)
					return
				}
				w.WriteHeader(http.StatusUnprocessableEntity)
			}))
			defer backend.Close()

			oldBackend := config.BackendDomain
			config.BackendDomain = backend.URL
			t.Cleanup(func() { config.BackendDomain = oldBackend })

			sessionID := "bulk-purchase-cache-" + strings.ReplaceAll(t.Name(), "/", "-")
			cleanup := setupPBTestSession(sessionID, "bulk-purchase-cache-token")
			t.Cleanup(cleanup)

			const cacheSentinel = "cached purchase bills"
			helpers.APICache.Set("purchase_bills", cacheSentinel, time.Hour)
			t.Cleanup(func() { helpers.APICache.Delete("purchase_bills") })

			body := mustMarshalPurchaseBulkPayload(t, []map[string]interface{}{
				validBulkPurchaseBillPayload(),
				validBulkPurchaseBillPayload(),
			})
			req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills/bulk-import", strings.NewReader(body))
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			w := httptest.NewRecorder()

			HandleBulkPurchaseBillImport(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
			}
			var result bulkImportResponse
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode bulk response: %v", err)
			}
			if result.Total != 2 || result.Success != testCase.wantSuccess ||
				result.Failed != 2-testCase.wantSuccess || backendCalls != 2 {
				t.Fatalf("result = %+v, backend calls = %d; want success=%d and 2 backend calls",
					result, backendCalls, testCase.wantSuccess)
			}

			cached, ok := helpers.APICache.Get("purchase_bills")
			if ok != testCase.wantCached {
				t.Fatalf("purchase_bills cached = %v, want %v", ok, testCase.wantCached)
			}
			if testCase.wantCached && cached != cacheSentinel {
				t.Errorf("purchase_bills cache value = %v, want seeded sentinel", cached)
			}
		})
	}
}

func TestHandleBulkPurchaseBillImportReportsValidationPerItem(t *testing.T) {
	backendCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalls++
		w.WriteHeader(http.StatusCreated)
	}))
	defer backend.Close()

	oldBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = oldBackend })
	cleanup := setupPBTestSession("bulk-purchase-validation-test", "bulk-purchase-token")
	t.Cleanup(cleanup)

	missingCost := validBulkPurchaseBillPayload()
	delete(missingCost["manual_products"].([]map[string]interface{})[0], "cost_price")
	missingSequence := validBulkPurchaseBillPayload()
	missingSequence["supplier_sequence_number"] = 0
	missingPayment := validBulkPurchaseBillPayload()
	delete(missingPayment, "payment_method")
	invalidDate := validBulkPurchaseBillPayload()
	invalidDate["effective_date"] = "not-a-date"

	body := mustMarshalPurchaseBulkPayload(t, []map[string]interface{}{
		validBulkPurchaseBillPayload(),
		missingCost,
		missingSequence,
		missingPayment,
		invalidDate,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills/bulk-import", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "bulk-purchase-validation-test"})
	w := httptest.NewRecorder()

	HandleBulkPurchaseBillImport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var result bulkImportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode bulk response: %v", err)
	}
	if result.Total != 5 || result.Success != 1 || result.Failed != 4 || len(result.Results) != 5 {
		t.Fatalf("result = %+v, want total=5 success=1 failed=4", result)
	}
	if backendCalls != 1 {
		t.Errorf("backend calls = %d, want only the valid row to be forwarded", backendCalls)
	}
	for i := 1; i < len(result.Results); i++ {
		if result.Results[i].OK || result.Results[i].Error == "" {
			t.Errorf("result[%d] = %+v, want a validation error", i, result.Results[i])
		}
	}
	if !strings.Contains(result.Results[1].Error, "cost_price") ||
		!strings.Contains(result.Results[2].Error, "supplier_sequence_number") ||
		!strings.Contains(result.Results[3].Error, "payment_method") ||
		!strings.Contains(result.Results[4].Error, "effective_date") {
		t.Errorf("validation errors = %+v, want field-specific reasons", result.Results)
	}
}

func TestHandleBulkPurchaseBillImportMalformedRequestIsNotSuccessShaped(t *testing.T) {
	cleanup := setupPBTestSession("bulk-purchase-malformed-test", "bulk-purchase-token")
	t.Cleanup(cleanup)

	req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills/bulk-import", strings.NewReader(`{"store_id":5}`))
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "bulk-purchase-malformed-test"})
	w := httptest.NewRecorder()

	HandleBulkPurchaseBillImport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	var result bulkImportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if result.Error == "" || result.Success != 0 || result.Total != 0 || len(result.Results) != 0 {
		t.Errorf("response = %+v, want explicit error and zero successes", result)
	}
}

func TestHandleBulkPurchaseBillImportRequiresAuthentication(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/purchase-bills/bulk-import", strings.NewReader("[]"))
	w := httptest.NewRecorder()

	HandleBulkPurchaseBillImport(w, req)

	if w.Code < http.StatusMultipleChoices || w.Code >= http.StatusBadRequest {
		t.Fatalf("status = %d, want an authentication redirect", w.Code)
	}
}

func validBulkPurchaseBillPayload() map[string]interface{} {
	return map[string]interface{}{
		"store_id":                 5,
		"merchant_id":              0,
		"supplier_id":              7,
		"supplier_sequence_number": 1001,
		"state":                    3,
		"effective_date":           "2026-04-10T00:00:00+03:00",
		"products":                 []interface{}{},
		"manual_products": []map[string]interface{}{
			{
				"product_id": nil,
				"name":       "Oil filter",
				"price":      "25",
				"quantity":   "2",
				"cost_price": "20.5",
			},
		},
		"discount":       "0",
		"subtotal":       50,
		"payment_method": 10,
		"paid_amount":    "0",
		"attachments":    []interface{}{},
	}
}

func mustMarshalPurchaseBulkPayload(t *testing.T, value interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return string(encoded)
}
