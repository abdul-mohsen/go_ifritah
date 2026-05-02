package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"afrita/config"

	"github.com/gorilla/mux"
)

func TestSubmitDraftInvoicePreservesZatcaSubmissionFields(t *testing.T) {
	var submitted map[string]interface{}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/bill/321" {
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":                321,
				"sequence_number":   9001,
				"state":             0,
				"total":             115.0,
				"discount":          5.0,
				"store_id":          12,
				"branch_id":         7,
				"client_id":         44,
				"payment_method":    2,
				"effective_date":    "2025-02-03T00:00:00+03:00",
				"payment_due_date":  "2025-02-10T00:00:00+03:00",
				"deliver_date":      "2025-02-04T00:00:00+03:00",
				"maintenance_cost":  3.5,
				"vin":               "VIN-ZATCA-321",
				"user_name":         "ZATCA Customer",
				"user_phone_number": "0500000000",
				"note":              "submit through ZATCA branch",
				"products":          []interface{}{},
				"manual_products":   []map[string]interface{}{{"name": "Manual service", "part_name": "SRV-1", "price": 100, "quantity": 1}},
			})
		case http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&submitted); err != nil {
				t.Fatalf("decode submitted payload: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer backend.Close()

	prevBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = prevBackend })

	sessionID := "test-submit-zatca-fields"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "test-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})

	req := httptest.NewRequest(http.MethodPost, "/api/invoices/321/submit", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "321"})
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()

	HandleSubmitDraftInvoice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected submit handler status 200, got %d body=%q", w.Code, w.Body.String())
	}
	if submitted == nil {
		t.Fatal("backend submit endpoint was not called")
	}

	assertNumber := func(key string, want float64) {
		t.Helper()
		got, ok := submitted[key].(float64)
		if !ok || got != want {
			t.Fatalf("payload[%s] = %#v, want %v", key, submitted[key], want)
		}
	}
	assertString := func(key string, want string) {
		t.Helper()
		got, ok := submitted[key].(string)
		if !ok || got != want {
			t.Fatalf("payload[%s] = %#v, want %q", key, submitted[key], want)
		}
	}

	assertNumber("state", 1)
	assertNumber("store_id", 12)
	assertNumber("branch_id", 7)
	assertNumber("client_id", 44)
	assertNumber("payment_method", 2)
	assertString("effective_date", "2025-02-03T00:00:00+03:00")
	assertString("payment_due_date", "2025-02-10T00:00:00+03:00")
	assertString("deliver_date", "2025-02-04T00:00:00+03:00")
	assertString("vin", "VIN-ZATCA-321")
	assertString("user_name", "ZATCA Customer")
	assertString("note", "submit through ZATCA branch")
}
