package handlers

import (
	"afrita/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

func TestCreatePurchaseBillZeroTotalRejectedWhileCreateInFlight(t *testing.T) {
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

	cleanup := setupPBTestSession("pb-zero-locked", "pb-zero-locked-token")
	defer cleanup()

	purchaseBillCreateLock.Store("pb-zero-locked-token", true)
	defer purchaseBillCreateLock.Delete("pb-zero-locked-token")

	form := "store_id=4&supplier_id=251&payment_date=2026-04-10&payment_method=10" +
		"&manual_part_name=Zero" +
		"&manual_quantity=1&manual_price=0&discount=0&total_amount=0"

	req := httptest.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-zero-locked"})
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

func TestCreatePurchaseBillDuplicateGuardStillReturnsNoContent(t *testing.T) {
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

	cleanup := setupPBTestSession("pb-valid-locked", "pb-valid-locked-token")
	defer cleanup()

	purchaseBillCreateLock.Store("pb-valid-locked-token", true)
	defer purchaseBillCreateLock.Delete("pb-valid-locked-token")

	form := "store_id=4&supplier_id=251&payment_date=2026-04-10&payment_method=10" +
		"&manual_part_name=Valid" +
		"&manual_quantity=1&manual_price=10&discount=0&total_amount=10"

	req := httptest.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-valid-locked"})
	w := httptest.NewRecorder()

	HandleCreatePurchaseBill(w, req)

	if backendCalled {
		t.Fatal("backend should not be called while another create holds the lock")
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204", w.Code)
	}
}
