package handlers

import (
	"afrita/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSaveSettingsSendsPBPDFRequiredToBackend is a regression test for a bug
// where "pb_pdf_required" (whether a PDF attachment is required on purchase
// bills) was treated as a frontend-only setting - excluded from
// settingsCategoryMap and therefore never sent to the backend at all. It
// only ever lived in the process-local settingsByToken map, which is wiped
// on every restart/rebuild, making the setting appear to silently reset.
func TestSaveSettingsSendsPBPDFRequiredToBackend(t *testing.T) {
	var capturedPayloads []map[string]interface{}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/settings") {
			_, _ = w.Write([]byte(`{"data":{}}`))
			return
		}
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/api/v2/settings") {
			var payload map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			capturedPayloads = append(capturedPayloads, payload)
			_, _ = w.Write([]byte(`{"detail":"success","updated":1}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-pdf-setting-test", "pb-pdf-setting-token")
	defer cleanup()

	form := "pb_pdf_required=optional"
	req := httptest.NewRequest(http.MethodPost, "/dashboard/settings", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-pdf-setting-test"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got %d. body: %s", w.Code, w.Body.String())
	}

	found := false
	for _, payload := range capturedPayloads {
		if payload["category"] != "invoice" {
			continue
		}
		settings, ok := payload["settings"].(map[string]interface{})
		if !ok {
			continue
		}
		if v, ok := settings["pb_pdf_required"]; ok && v == "optional" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected pb_pdf_required=optional to be sent to backend PUT /api/v2/settings under the invoice category, got payloads: %+v", capturedPayloads)
	}
}

func TestSaveSettingsSendsExplicitlyClearedTextValue(t *testing.T) {
	var invoiceSettings map[string]interface{}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/settings") {
			_, _ = w.Write([]byte(`{"data":{}}`))
			return
		}
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/api/v2/settings") {
			var payload struct {
				Category string                 `json:"category"`
				Settings map[string]interface{} `json:"settings"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Category == "invoice" {
				invoiceSettings = payload.Settings
			}
			_, _ = w.Write([]byte(`{"detail":"success","updated":1}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("settings-clear-test", "settings-clear-token")
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/settings",
		strings.NewReader("invoice_footer=&pb_pdf_required=optional"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "settings-clear-test"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got %d. body: %s", w.Code, w.Body.String())
	}
	if value, ok := invoiceSettings["invoice_footer"]; !ok || value != "" {
		t.Fatalf("expected explicit empty invoice_footer to be sent, got %v", invoiceSettings)
	}
}

func TestAddPurchaseBillLoadsPDFSettingFromBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/settings":
			_, _ = w.Write([]byte(`{"data":{"invoice":{"pb_pdf_required":"optional"}}}`))
		case "/api/v2/notification/config":
			_, _ = w.Write([]byte(`{"data":{"low_stock_alert":true,"low_stock_threshold":5,"pending_invoice_days":7,"new_order_alert":true,"payment_due_alert":true}}`))
		default:
			_, _ = w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	token := "settings-load-token"
	settingsByToken.Delete(tokenKey(token))
	cleanup := setupPBTestSession("settings-load-test", token)
	defer func() {
		cleanup()
		settingsByToken.Delete(tokenKey(token))
	}()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/purchase-bills/add", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "settings-load-test"})
	w := httptest.NewRecorder()

	HandleAddPurchaseBill(w, req)

	start := strings.Index(w.Body.String(), `name="bill_pdf"`)
	if start < 0 {
		t.Fatal("expected purchase-bill PDF input")
	}
	field := w.Body.String()[start : start+220]
	if strings.Contains(field, " required") {
		t.Fatalf("optional PDF setting from backend must not mark input required: %s", field)
	}
}
