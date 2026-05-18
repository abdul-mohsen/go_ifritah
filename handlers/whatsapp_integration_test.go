package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"afrita/config"
	"afrita/helpers"

	"github.com/gorilla/mux"
)

func withWhatsAppBackend(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	prev := config.BackendDomain
	config.BackendDomain = server.URL
	t.Cleanup(func() { config.BackendDomain = prev })
}

func seedWhatsAppSession(t *testing.T, token string) string {
	t.Helper()
	sessionID := "whatsapp-test-session"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = token
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})
	return sessionID
}

func TestWhatsAppSettingsDefaults(t *testing.T) {
	settings := storeFor("whatsapp-defaults-token")
	settings.mu.RLock()
	defer settings.mu.RUnlock()

	checks := map[string]string{
		"whatsapp_enabled":             "false",
		"whatsapp_business_account_id": "",
		"whatsapp_phone_number_id":     "",
		"whatsapp_access_token":        "",
		"whatsapp_api_version":         "v18.0",
		"whatsapp_invoice_message":     "Invoice PDF is attached.",
	}
	for key, want := range checks {
		if got := settings.values[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestNormalizeSettingsValue_WhatsAppDefaults(t *testing.T) {
	if got := normalizeSettingsValue("whatsapp_api_version", ""); got != "v18.0" {
		t.Fatalf("whatsapp_api_version default = %q", got)
	}
	if got := normalizeSettingsValue("whatsapp_invoice_message", " "); got != "Invoice PDF is attached." {
		t.Fatalf("whatsapp_invoice_message default = %q", got)
	}
	if got := normalizeSettingsValue("company_email", ""); got != "" {
		t.Fatalf("non-WhatsApp blank should remain blank, got %q", got)
	}
}

func TestHandleSaveSettings_WhatsAppIntegrationsPayloadOmitsBlankToken(t *testing.T) {
	sessionID := seedWhatsAppSession(t, "whatsapp-save-token")
	settings := storeFor("whatsapp-save-token")
	settings.mu.Lock()
	settings.values["whatsapp_access_token"] = whatsappMaskedToken
	settings.mu.Unlock()

	var integrations map[string]string
	withWhatsAppBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v2/settings" {
			w.WriteHeader(http.StatusOK)
			return
		}
		var payload struct {
			Category string            `json:"category"`
			Settings map[string]string `json:"settings"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Category == "integrations" {
			integrations = payload.Settings
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	}))

	form := url.Values{
		"whatsapp_enabled":             {"true"},
		"whatsapp_business_account_id": {"waba-1"},
		"whatsapp_phone_number_id":     {"phone-1"},
		"whatsapp_api_version":         {"v18.0"},
		"whatsapp_invoice_message":     {"Invoice ready"},
		"whatsapp_access_token":        {whatsappMaskedToken},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if integrations == nil {
		t.Fatal("integrations settings were not sent")
	}
	if _, ok := integrations["whatsapp_access_token"]; ok {
		t.Fatalf("masked access token should not be forwarded: %#v", integrations)
	}
	if integrations["whatsapp_enabled"] != "true" || integrations["whatsapp_business_account_id"] != "waba-1" {
		t.Fatalf("unexpected integrations payload: %#v", integrations)
	}
}

func TestHandleGetInvoice_ShowsWhatsAppButtonOnlyWhenEnabled(t *testing.T) {
	sessionID := seedWhatsAppSession(t, "whatsapp-invoice-token")
	withWhatsAppBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/settings":
			_, _ = io.WriteString(w, `{"data":{"integrations":{"whatsapp_enabled":"true"}}}`)
		case "/api/v2/bill/123":
			_, _ = io.WriteString(w, `{"id":123,"sequence_number":77,"state":1,"total":25,"bill_type":true,"effective_date":"2026-05-19T00:00:00Z"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/bill/123", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	req = mux.SetURLVars(req, map[string]string{"id": "123"})
	w := httptest.NewRecorder()

	HandleGetInvoice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `/api/invoices/123/whatsapp`) || !strings.Contains(body, `data-whatsapp-send-button`) {
		t.Fatalf("expected WhatsApp send button, body=%s", body)
	}
}

func TestHandleSettingsPage_WhatsAppTokenIsNeverRendered(t *testing.T) {
	helpers.APICache.Delete("branches")
	helpers.APICache.Delete("stores")
	sessionID := seedWhatsAppSession(t, "whatsapp-settings-page-token")
	withWhatsAppBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/settings":
			_, _ = io.WriteString(w, `{"data":{"integrations":{"whatsapp_access_token":"********","whatsapp_api_version":"v18.0","whatsapp_invoice_message":"Invoice PDF is attached."}}}`)
		case "/api/v2/branch/all", "/api/v2/stores/all", "/api/v2/notification/config":
			_, _ = io.WriteString(w, `{"data":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/settings", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()

	HandleSettingsPage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, whatsappMaskedToken) {
		t.Fatalf("settings page rendered the saved access token")
	}
	if !strings.Contains(body, `name="whatsapp_access_token" value=""`) {
		t.Fatalf("settings page should render an empty token input")
	}
	if !strings.Contains(body, `id="tab-integrations"`) || !strings.Contains(body, `name="whatsapp_enabled"`) {
		t.Fatalf("settings page missing WhatsApp integration controls")
	}
}

func TestHandleSendInvoiceWhatsApp_Success(t *testing.T) {
	sessionID := seedWhatsAppSession(t, "whatsapp-send-token")
	var hit string
	withWhatsAppBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = r.Method + " " + r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"detail":"sent","message_id":"wamid.1"}`)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/invoices/123/whatsapp", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	req = mux.SetURLVars(req, map[string]string{"id": "123"})
	w := httptest.NewRecorder()

	HandleSendInvoiceWhatsApp(w, req)

	if hit != "POST /api/v2/bill/123/whatsapp" {
		t.Fatalf("backend hit = %q", hit)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "success") {
		t.Fatalf("expected success toast trigger, got %s", w.Header().Get("HX-Trigger"))
	}
}

func TestHandleSendInvoiceWhatsApp_BackendErrorUsesDetail(t *testing.T) {
	sessionID := seedWhatsAppSession(t, "whatsapp-error-token")
	withWhatsAppBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"detail":"missing customer phone"}`)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/invoices/123/whatsapp", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	req = mux.SetURLVars(req, map[string]string{"id": "123"})
	w := httptest.NewRecorder()

	HandleSendInvoiceWhatsApp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "missing customer phone") {
		t.Fatalf("expected backend detail in body, got %s", w.Body.String())
	}
	if !strings.Contains(w.Header().Get("HX-Trigger"), "error") {
		t.Fatalf("expected error toast trigger, got %s", w.Header().Get("HX-Trigger"))
	}
}
