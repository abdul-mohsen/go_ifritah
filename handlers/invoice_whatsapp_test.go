package handlers

import (
	"afrita/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestHandleSendInvoiceWhatsAppSuccess(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/bill/123/whatsapp" {
			t.Fatalf("backend path = %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatal("expected Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"detail":"sent","message_id":"wamid.1"}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	setupPDFSession(t)

	req := httptest.NewRequest(http.MethodPost, "/api/invoices/123/whatsapp", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-pdf-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "123"})

	w := httptest.NewRecorder()
	HandleSendInvoiceWhatsApp(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "showToast") {
		t.Fatalf("expected success toast trigger, got %s", w.Header().Get("HX-Trigger"))
	}
}

func TestHandleSendInvoiceWhatsAppBackendError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"WhatsApp invoice sending is disabled"}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	setupPDFSession(t)

	req := httptest.NewRequest(http.MethodPost, "/api/invoices/123/whatsapp", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-pdf-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "123"})

	w := httptest.NewRecorder()
	HandleSendInvoiceWhatsApp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "WhatsApp invoice sending is disabled") {
		t.Fatalf("body = %s", w.Body.String())
	}
}
