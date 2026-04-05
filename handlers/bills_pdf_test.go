package handlers

import (
	"afrita/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func setupPDFSession(t *testing.T) {
	t.Helper()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-pdf-session"] = "test-pdf-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "test-pdf-session")
		config.SessionTokensMutex.Unlock()
	})
}

func TestHandleBillPDF_Success(t *testing.T) {
	fakePDF := []byte("%PDF-1.4 fake pdf content for testing")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/bill/pdf/123" {
			t.Errorf("expected backend path /api/v2/bill/pdf/123, got %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("expected Authorization header")
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		w.Write(fakePDF)
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	setupPDFSession(t)

	req := httptest.NewRequest("GET", "/bill/pdf/123", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-pdf-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "123"})

	w := httptest.NewRecorder()
	HandleBillPDF(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "invoice-123.pdf") {
		t.Errorf("expected Content-Disposition with invoice-123.pdf, got %s", cd)
	}
	if w.Body.Len() != len(fakePDF) {
		t.Errorf("expected body length %d, got %d", len(fakePDF), w.Body.Len())
	}
}

func TestHandleBillPDF_NoSession(t *testing.T) {
	req := httptest.NewRequest("GET", "/bill/pdf/123", nil)
	// No session_id cookie
	req = mux.SetURLVars(req, map[string]string{"id": "123"})

	w := httptest.NewRecorder()
	HandleBillPDF(w, req)

	// Should redirect to login (303 See Other)
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", w.Code)
	}
}

func TestHandleBillPDF_BackendError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	setupPDFSession(t)

	req := httptest.NewRequest("GET", "/bill/pdf/123", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-pdf-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "123"})

	w := httptest.NewRecorder()
	HandleBillPDF(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON error response, got Content-Type %s", ct)
	}
	if !strings.Contains(w.Body.String(), "pdf_unavailable") {
		t.Errorf("expected pdf_unavailable error in body, got %s", w.Body.String())
	}
}

func TestHandleCreditBillPDF_Success(t *testing.T) {
	fakePDF := []byte("%PDF-1.4 fake credit bill pdf")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/bill/credit/pdf/456" {
			t.Errorf("expected backend path /api/v2/bill/credit/pdf/456, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		w.Write(fakePDF)
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	setupPDFSession(t)

	req := httptest.NewRequest("GET", "/credit_bill/pdf/456", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-pdf-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "456"})

	w := httptest.NewRecorder()
	HandleCreditBillPDF(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "credit-bill-456.pdf") {
		t.Errorf("expected Content-Disposition with credit-bill-456.pdf, got %s", cd)
	}
	if w.Body.Len() != len(fakePDF) {
		t.Errorf("expected body length %d, got %d", len(fakePDF), w.Body.Len())
	}
}
