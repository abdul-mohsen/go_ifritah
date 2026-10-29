package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"

	"github.com/gorilla/mux"
)

func TestHandleGetInvoiceRendersPersistedZATCAQR(t *testing.T) {
	const (
		qrCode       = "persisted-zatca-qr"
		invoiceUUID  = "invoice-uuid-321"
		invoiceHash  = "invoice-hash-321"
		xmlPath      = "tenant/invoices/321.xml"
		sessionID    = "invoice-detail-qr-test"
		invoiceID    = "321"
		issuedStatus = "صادرة"
	)

	var currentQR = qrCode
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v2/bill/"+invoiceID:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":                321,
				"sequence_number":   9321,
				"state":             3,
				"total":             115.0,
				"subtotal":          100.0,
				"total_before_vat":  100.0,
				"total_vat":         15.0,
				"vat":               15.0,
				"type":              false,
				"effective_date":    "2026-10-29T00:00:00+03:00",
				"qr_code":           currentQR,
				"invoice_uuid":      map[string]interface{}{"String": invoiceUUID, "Valid": true},
				"invoice_hash":      invoiceHash,
				"invoice_xml_path":  xmlPath,
				"products":          []interface{}{},
				"manual_products":   []interface{}{},
				"payment_due_date":  "",
				"deliver_date":      "",
				"maintenance_cost":  0.0,
				"user_name":         "",
				"user_phone_number": "",
				"note":              "",
			})
		case r.URL.Path == "/api/v2/settings":
			_, _ = w.Write([]byte(`{"data":{}}`))
		case r.URL.Path == "/api/v2/notification/config":
			_, _ = w.Write([]byte(`{"data":{}}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer backend.Close()

	previousBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = previousBackend })

	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "test-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})

	render := func() string {
		req := httptest.NewRequest(http.MethodGet, "/bill/"+invoiceID, nil)
		req = mux.SetURLVars(req, map[string]string{"id": invoiceID})
		req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})

		w := httptest.NewRecorder()
		HandleGetInvoice(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %q", w.Code, http.StatusOK, w.Body.String())
		}
		return w.Body.String()
	}

	body := render()
	for _, want := range []string{qrCode, invoiceUUID, invoiceHash, xmlPath, issuedStatus, "qr-canvas"} {
		if !strings.Contains(body, want) {
			t.Fatalf("invoice detail body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "qr_pending") {
		t.Fatalf("persisted QR body unexpectedly rendered pending label:\n%s", body)
	}

	currentQR = ""
	body = render()
	for _, unwanted := range []string{"qr-canvas", "QRious", qrCode} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("empty QR body rendered %q:\n%s", unwanted, body)
		}
	}
	if !strings.Contains(body, "رمز الاستجابة السريعة غير متاح بعد") {
		t.Fatalf("empty QR body missing localized pending label:\n%s", body)
	}
}
