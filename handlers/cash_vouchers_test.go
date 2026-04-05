package handlers

import (
	"afrita/config"
	"afrita/helpers"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCreateCashVoucherAmountIsString verifies that the cash voucher create
// payload sends amount as a JSON string (not number) to the backend.
func TestCreateCashVoucherAmountIsString(t *testing.T) {
	helpers.APICache.Delete("cash_vouchers")

	var capturedBody []byte
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/api/v2/cash_voucher") && r.Method == "POST" {
			capturedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":1,"voucher_number":1}`))
			return
		}
		// stores/suppliers for other fetches
		w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	config.SessionTokensMutex.Lock()
	config.SessionTokens["cv-test-session"] = "cv-test-token"
	config.SessionTokensMutex.Unlock()
	defer func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "cv-test-session")
		config.SessionTokensMutex.Unlock()
	}()

	formData := "voucher_type=disbursement" +
		"&effective_date=2026-03-01" +
		"&amount=1500.50" +
		"&payment_method=cash" +
		"&recipient_type=supplier" +
		"&recipient_name=test" +
		"&store_id=1" +
		"&csrf_token=test"

	req := httptest.NewRequest("POST", "/dashboard/cash-vouchers/create", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "cv-test-session"})
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test"})
	w := httptest.NewRecorder()

	HandleCreateCashVoucher(w, req)

	if len(capturedBody) == 0 {
		t.Fatal("backend did not receive any request body")
	}

	// Parse the JSON body as raw map to check types
	var raw map[string]interface{}
	if err := json.Unmarshal(capturedBody, &raw); err != nil {
		t.Fatalf("failed to parse captured body: %v", err)
	}

	// amount must be a string in the JSON payload, not a number
	amountVal, ok := raw["amount"]
	if !ok {
		t.Fatal("amount field missing from JSON payload")
	}

	if _, isString := amountVal.(string); !isString {
		t.Errorf("amount should be a JSON string, got %T (value: %v)", amountVal, amountVal)
	}
}
