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

// TestCreatePurchaseBillRejectsZeroTotalUnderLockContention proves the
// zero-total 400 fires even when the process-wide duplicate-submission
// lock (purchaseBillCreateLock) is already held for the same session
// token by another in-flight request. Playwright e2e runs 4 parallel
// workers sharing storageState.json (so they share one session cookie),
// and a peer worker's slow valid submit used to make our zero-total
// assertion see the lock's 204 sentinel instead of the actual 400 —
// the exact qa-34-purchase-bill-zero-total.spec.js failure this test
// pins.
func TestCreatePurchaseBillRejectsZeroTotalUnderLockContention(t *testing.T) {
	backendCalled := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 1000}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-zero-total-lock", "pb-zero-lock-token")
	defer cleanup()

	// Simulate a peer worker holding the lock for the same token.
	const token = "pb-zero-lock-token"
	purchaseBillCreateLock.LoadOrStore(token, true)
	defer purchaseBillCreateLock.Delete(token)

	form := "store_id=4&supplier_id=251&payment_date=2026-04-10&payment_method=10" +
		"&products_part_name=Zero&products_product_id=0" +
		"&products_quantity=1&products_cost_price=0&discount=0&total_amount=0"

	req := httptest.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-zero-total-lock"})
	w := httptest.NewRecorder()

	HandleCreatePurchaseBill(w, req)

	if backendCalled {
		t.Fatal("backend should not be called for zero-total purchase bill")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 (peer-lock must not mask zero-total validation)", w.Code)
	}
	if !strings.Contains(w.Body.String(), "You can't submit an invoice with 0") {
		t.Fatalf("expected zero-total error message, got %q", w.Body.String())
	}
}

// TestCreatePurchaseBillLockStillBlocksValidDuplicate is the counter-test
// to TestCreatePurchaseBillRejectsZeroTotalUnderLockContention: for a
// STRUCTURALLY VALID submit (non-zero total) the process-wide lock must
// still return 204 when contended, so the intended "silent duplicate
// suppression" behaviour is preserved after moving the Subtotal check
// above the lock.
func TestCreatePurchaseBillLockStillBlocksValidDuplicate(t *testing.T) {
	backendCalled := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 1001}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-valid-lock", "pb-valid-lock-token")
	defer cleanup()

	// Peer worker holds the lock; a valid-total duplicate must return 204.
	const token = "pb-valid-lock-token"
	purchaseBillCreateLock.LoadOrStore(token, true)
	defer purchaseBillCreateLock.Delete(token)

	form := "store_id=4&supplier_id=251&payment_date=2026-04-10&payment_method=10" +
		"&products_part_name=Widget&products_product_id=0" +
		"&products_quantity=1&products_cost_price=15&discount=0&total_amount=15"

	req := httptest.NewRequest("POST", "/api/purchase-bills", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-valid-lock"})
	w := httptest.NewRecorder()

	HandleCreatePurchaseBill(w, req)

	if backendCalled {
		t.Fatal("backend should not be called while duplicate lock is held")
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204 (lock still gates valid duplicate submits)", w.Code)
	}
}
