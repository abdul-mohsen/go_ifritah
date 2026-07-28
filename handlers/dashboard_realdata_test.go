package handlers

import (
	"afrita/config"
	"afrita/helpers"
	"afrita/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleDashboardRealData(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v2/bill/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		invoices := []models.Invoice{
			{
				ID:             1,
				SequenceNumber: 1001,
				State:          1,
				Total:          150.5,
				EffectiveDate: struct {
					Time  string `json:"Time"`
					Valid bool   `json:"Valid"`
				}{Time: "2026-02-13T10:00:00Z", Valid: true},
			},
			{
				ID:             2,
				SequenceNumber: 1002,
				State:          3,
				Total:          320.0,
				EffectiveDate: struct {
					Time  string `json:"Time"`
					Valid bool   `json:"Valid"`
				}{Time: "2026-02-12T10:00:00Z", Valid: true},
			},
		}
		_ = json.NewEncoder(w).Encode(invoices)
	})

	mux.HandleFunc("/api/v2/product/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		products := []models.Product{{ID: 10, Quantity: "7"}, {ID: 11, Quantity: "3"}}
		_ = json.NewEncoder(w).Encode(products)
	})

	mux.HandleFunc("/api/v2/client/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		clients := []models.Client{{ID: "1", Name: "Acme"}, {ID: "2", Name: "Beta"}}
		_ = json.NewEncoder(w).Encode(clients)
	})

	mux.HandleFunc("/api/v2/supplier/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		suppliers := []models.Supplier{{ID: 1, Name: "Supplier A"}}
		_ = json.NewEncoder(w).Encode(suppliers)
	})

	mux.HandleFunc("/api/v2/order/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		orders := []map[string]interface{}{
			{"client": "Acme", "status": "pending", "total": 1200, "date": "2026-02-12"},
			{"client": "Beta", "status": "completed", "total": 900, "date": "2026-02-10"},
		}
		_ = json.NewEncoder(w).Encode(orders)
	})

	mux.HandleFunc("/api/v2/purchase_bill/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
	})

	mux.HandleFunc("/api/v2/store/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]models.Store{{ID: 1, Name: "Main Store"}})
	})

	mux.HandleFunc("/api/v2/branch/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{{"id": 1, "name": "Branch 1"}})
	})

	mux.HandleFunc("/api/v2/dashboard/available-years", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"years": []int{2026, 2025}})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	oldBackend := config.BackendDomain
	oldClient := helpers.HttpClient
	config.BackendDomain = server.URL
	helpers.HttpClient = server.Client()
	t.Cleanup(func() {
		config.BackendDomain = oldBackend
		helpers.HttpClient = oldClient
	})

	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "test-session")
		config.SessionTokensMutex.Unlock()
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	rr := httptest.NewRecorder()

	HandleDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "العملاء") {
		t.Fatalf("dashboard missing clients card")
	}
}

func TestHandleDashboardAvailableYears(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"years": []int{2026, 2025}})
	}))
	t.Cleanup(server.Close)

	oldBackend := config.BackendDomain
	oldClient := helpers.HttpClient
	config.BackendDomain = server.URL
	helpers.HttpClient = server.Client()
	t.Cleanup(func() {
		config.BackendDomain = oldBackend
		helpers.HttpClient = oldClient
	})

	config.SessionTokensMutex.Lock()
	config.SessionTokens["available-years-session"] = "token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "available-years-session")
		config.SessionTokensMutex.Unlock()
	})

	req := httptest.NewRequest("GET", "/api/v2/dashboard/available-years", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "available-years-session"})
	rr := httptest.NewRecorder()

	HandleDashboardAvailableYears(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var response helpers.DashboardAvailableYearsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode available years response: %v", err)
	}
	if len(response.Years) != 2 || response.Years[0] != 2026 || response.Years[1] != 2025 {
		t.Fatalf("unexpected available years: %#v", response.Years)
	}
}
