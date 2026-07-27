package handlers

import (
	"afrita/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestCreatePurchaseBillRejectsZeroTotal(t *testing.T) {
	backendCalled := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 999}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-zero-total", "pb-zero-token")
	defer cleanup()

	form := "store_id=4&supplier_id=251&payment_date=2026-04-10&payment_method=10" +
		"&manual_part_name=Zero" +
		"&manual_quantity=1&manual_price=0&discount=0&total_amount=0"

	req := httptest.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-zero-total"})
	w := httptest.NewRecorder()

	HandleCreatePurchaseBill(w, req)

	if backendCalled {
		t.Fatal("backend should not be called for zero-total purchase bill")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "You can't submit an invoice with 0") {
		t.Fatalf("expected zero-total error message, got %q", w.Body.String())
	}
}

func TestCreatePurchaseBillRejectsDiscountToZero(t *testing.T) {
	backendCalled := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-discount-zero", "pb-discount-zero-token")
	defer cleanup()

	form := "store_id=4&supplier_id=251&payment_date=2026-04-10&payment_method=10" +
		"&manual_part_name=Discounted&manual_quantity=1&manual_price=50" +
		"&discount=100&total_amount=0"
	req := httptest.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-discount-zero"})
	w := httptest.NewRecorder()

	HandleCreatePurchaseBill(w, req)

	if backendCalled {
		t.Fatal("backend should not be called when discount reduces total to zero")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestUpdatePurchaseBillRejectsZeroTotal(t *testing.T) {
	backendCalled := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-zero-update", "pb-zero-update-token")
	defer cleanup()

	req := httptest.NewRequest("PUT", "/api/purchase-bills/123", strings.NewReader(
		"store_id=4&supplier_id=251&payment_date=2026-04-10&payment_method=10"+
			"&manual_part_name=Zero&manual_quantity=1&manual_price=0&discount=0&total_amount=0",
	))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-zero-update"})
	req = mux.SetURLVars(req, map[string]string{"id": "123"})
	w := httptest.NewRecorder()

	HandleUpdatePurchaseBill(w, req)

	if backendCalled {
		t.Fatal("backend should not be called for zero-total purchase bill updates")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), purchaseBillZeroTotalText) {
		t.Fatalf("expected zero-total error message, got %q", w.Body.String())
	}
}
