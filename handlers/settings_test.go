//go:build ignore
// +build ignore

// Disabled: depends on settings_store.go.disabled mock stores.

package handlers

import (
	"afrita/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ============================================================================
// Settings Store Tests
// ============================================================================

func TestSettingsStoreGetAll(t *testing.T) {
	store := NewMockSettingsStore()
	all := store.GetAll()

	if len(all) < 30 {
		t.Fatalf("expected at least 30 seed settings, got %d", len(all))
	}

	// Check a few known defaults from each section
	checks := map[string]string{
		"company_name":            "عفريته لقطع الغيار",
		"company_email":           "info@afrita.dev",
		"currency":                "SAR",
		"vat_rate":                "15",
		"invoice_prefix":          "INV-",
		"company_vat":             "300000000000003",
		"paper_size":              "A4",
		"theme":                   "light",
		"language":                "ar",
		"notif_invoices":          "true",
		"session_duration":        "60",
		"low_stock_threshold":     "5",
		"default_unit":            "piece",
		"show_vat_breakdown":      "true",
		"auto_logout_inactive":    "true",
		"allow_negative_stock":    "false",
	}
	for key, want := range checks {
		if got := all[key]; got != want {
			t.Errorf("settings[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestSettingsStoreGet(t *testing.T) {
	store := NewMockSettingsStore()

	if v := store.Get("company_name"); v != "عفريته لقطع الغيار" {
		t.Errorf("Get(company_name) = %q, want Arabic name", v)
	}
	if v := store.Get("nonexistent_key"); v != "" {
		t.Errorf("Get(nonexistent_key) = %q, want empty string", v)
	}
}

func TestSettingsStoreUpdate(t *testing.T) {
	store := NewMockSettingsStore()

	updated := store.Update(map[string]string{
		"company_name":  "اختبار",
		"vat_rate":      "10",
	})

	if len(updated) != 2 {
		t.Fatalf("expected 2 updated keys, got %d", len(updated))
	}
	if v := store.Get("company_name"); v != "اختبار" {
		t.Errorf("after update, company_name = %q, want 'اختبار'", v)
	}
	if v := store.Get("vat_rate"); v != "10" {
		t.Errorf("after update, vat_rate = %q, want '10'", v)
	}
	// Other settings should be unchanged
	if v := store.Get("currency"); v != "SAR" {
		t.Errorf("currency should be unchanged, got %q", v)
	}
}

func TestSettingsStoreKeys(t *testing.T) {
	store := NewMockSettingsStore()
	keys := store.Keys()

	if len(keys) < 30 {
		t.Fatalf("expected at least 30 keys, got %d", len(keys))
	}

	// Keys should be sorted
	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[i-1] {
			t.Errorf("keys not sorted: %q comes after %q", keys[i], keys[i-1])
		}
	}
}

// ============================================================================
// Settings Handler Tests
// ============================================================================

func TestHandleSettingsLoadsData(t *testing.T) {
	// Reset store to known state
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	req := httptest.NewRequest("GET", "/dashboard/settings", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSettings(w, req)
	body := w.Body.String()

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Check that server-side settings are pre-populated in form inputs
	mustContain := []string{
		"عفريته لقطع الغيار",    // company_name default
		"info@afrita.dev",       // company_email default
		"300000000000003",       // company_vat default
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("settings page missing pre-populated value: %s", s)
		}
	}
}

func TestHandleSaveSettingsCompanySection(t *testing.T) {
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	form := url.Values{
		"section":       {"company"},
		"company_name":  {"شركة الاختبار"},
		"company_email": {"test@test.com"},
		"company_vat":   {"999999999999999"},
	}

	req := httptest.NewRequest("POST", "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	// Settings form uses plain POST, so handler returns 303 redirect
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/dashboard/settings" {
		t.Errorf("Location = %q, want /dashboard/settings", loc)
	}

	// Verify store was updated
	if v := SettingsStore.Get("company_name"); v != "شركة الاختبار" {
		t.Errorf("company_name = %q, want 'شركة الاختبار'", v)
	}
	if v := SettingsStore.Get("company_email"); v != "test@test.com" {
		t.Errorf("company_email = %q, want 'test@test.com'", v)
	}
	// Unchanged settings should remain
	if v := SettingsStore.Get("currency"); v != "SAR" {
		t.Errorf("currency should be unchanged, got %q", v)
	}
}

func TestHandleSaveSettingsInvoiceSection(t *testing.T) {
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	form := url.Values{
		"section":            {"invoice"},
		"vat_rate":           {"10"},
		"currency":           {"USD"},
		"invoice_prefix":     {"FACT-"},
		"payment_terms":      {"45"},
		"show_vat_breakdown": {"true"},
		// auto_calculate_vat not sent = unchecked = false
		"prices_include_vat": {"true"},
		"invoice_footer":     {"شكراً لتعاملكم"},
	}

	req := httptest.NewRequest("POST", "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if v := SettingsStore.Get("vat_rate"); v != "10" {
		t.Errorf("vat_rate = %q, want '10'", v)
	}
	if v := SettingsStore.Get("currency"); v != "USD" {
		t.Errorf("currency = %q, want 'USD'", v)
	}
	if v := SettingsStore.Get("show_vat_breakdown"); v != "true" {
		t.Errorf("show_vat_breakdown = %q, want 'true'", v)
	}
	if v := SettingsStore.Get("auto_calculate_vat"); v != "false" {
		t.Errorf("auto_calculate_vat = %q, want 'false' (unchecked)", v)
	}
	if v := SettingsStore.Get("prices_include_vat"); v != "true" {
		t.Errorf("prices_include_vat = %q, want 'true'", v)
	}
	if v := SettingsStore.Get("invoice_footer"); v != "شكراً لتعاملكم" {
		t.Errorf("invoice_footer = %q, want 'شكراً لتعاملكم'", v)
	}
}

func TestHandleSaveSettingsInventorySection(t *testing.T) {
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	form := url.Values{
		"section":             {"inventory"},
		"low_stock_threshold": {"10"},
		"default_unit":        {"kg"},
		"track_inventory":     {"true"},
		// allow_negative_stock not sent = unchecked = false
		"show_cost_price":     {"true"},
	}

	req := httptest.NewRequest("POST", "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if v := SettingsStore.Get("low_stock_threshold"); v != "10" {
		t.Errorf("low_stock_threshold = %q, want '10'", v)
	}
	if v := SettingsStore.Get("default_unit"); v != "kg" {
		t.Errorf("default_unit = %q, want 'kg'", v)
	}
	if v := SettingsStore.Get("track_inventory"); v != "true" {
		t.Errorf("track_inventory = %q, want 'true'", v)
	}
	if v := SettingsStore.Get("allow_negative_stock"); v != "false" {
		t.Errorf("allow_negative_stock = %q, want 'false' (unchecked)", v)
	}
	if v := SettingsStore.Get("show_cost_price"); v != "true" {
		t.Errorf("show_cost_price = %q, want 'true'", v)
	}
}

func TestHandleSaveSettingsPrintSection(t *testing.T) {
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	form := url.Values{
		"section":      {"print"},
		"paper_size":   {"A5"},
		"print_copies": {"3"},
		"show_logo_print": {"true"},
		// show_company_info_print not sent = unchecked = false
		"show_qr_print":    {"true"},
		"show_bank_details": {"true"},
		"bank_details":     {"بنك الراجحي SA1234"},
	}

	req := httptest.NewRequest("POST", "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if v := SettingsStore.Get("paper_size"); v != "A5" {
		t.Errorf("paper_size = %q, want 'A5'", v)
	}
	if v := SettingsStore.Get("show_company_info_print"); v != "false" {
		t.Errorf("show_company_info_print = %q, want 'false' (unchecked)", v)
	}
	if v := SettingsStore.Get("show_bank_details"); v != "true" {
		t.Errorf("show_bank_details = %q, want 'true'", v)
	}
	if v := SettingsStore.Get("bank_details"); v != "بنك الراجحي SA1234" {
		t.Errorf("bank_details = %q, want Arabic bank info", v)
	}
}

func TestHandleSaveSettingsSecuritySection(t *testing.T) {
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	form := url.Values{
		"section":                 {"security"},
		"session_duration":        {"120"},
		"max_login_attempts":      {"3"},
		"require_strong_password": {"true"},
		// auto_logout_inactive not sent = unchecked = false
	}

	req := httptest.NewRequest("POST", "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if v := SettingsStore.Get("session_duration"); v != "120" {
		t.Errorf("session_duration = %q, want '120'", v)
	}
	if v := SettingsStore.Get("auto_logout_inactive"); v != "false" {
		t.Errorf("auto_logout_inactive = %q, want 'false' (unchecked)", v)
	}
}

func TestHandleSaveSettingsNotificationsSection(t *testing.T) {
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	form := url.Values{
		"section":         {"notifications"},
		"notif_invoices":  {"true"},
		"notif_payments":  {"true"},
		// notif_stock, notif_orders, notif_session not sent = unchecked = false
	}

	req := httptest.NewRequest("POST", "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if v := SettingsStore.Get("notif_invoices"); v != "true" {
		t.Errorf("notif_invoices = %q, want 'true'", v)
	}
	if v := SettingsStore.Get("notif_stock"); v != "false" {
		t.Errorf("notif_stock = %q, want 'false' (unchecked)", v)
	}
	if v := SettingsStore.Get("notif_payments"); v != "true" {
		t.Errorf("notif_payments = %q, want 'true'", v)
	}
}

func TestHandleSaveSettingsAppearanceSection(t *testing.T) {
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	form := url.Values{
		"section":       {"appearance"},
		"theme":         {"dark"},
		"language":      {"en"},
		"date_format":   {"yyyy-mm-dd"},
		"number_format": {"ar"},
	}

	req := httptest.NewRequest("POST", "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if v := SettingsStore.Get("theme"); v != "dark" {
		t.Errorf("theme = %q, want 'dark'", v)
	}
	if v := SettingsStore.Get("language"); v != "en" {
		t.Errorf("language = %q, want 'en'", v)
	}
	if v := SettingsStore.Get("date_format"); v != "yyyy-mm-dd" {
		t.Errorf("date_format = %q, want 'yyyy-mm-dd'", v)
	}
}

func TestHandleSaveSettingsZatcaSection(t *testing.T) {
	SettingsStore = NewMockSettingsStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	// Mock backend that accepts PUT /api/v2/branch/:id/zatca and captures the payload
	var capturedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/zatca") && r.Method == "PUT" {
			json.NewDecoder(r.Body).Decode(&capturedPayload)
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]string{"detail": "success"})
			return
		}
		// Settings endpoints fall through to mock
		w.WriteHeader(404)
	}))
	defer server.Close()
	prev := config.BackendDomain
	config.BackendDomain = server.URL
	t.Cleanup(func() { config.BackendDomain = prev })

	form := url.Values{
		"section":                  {"zatca"},
		"zatca_branch_id":         {"1"},
		"zatca_otp":               {"123456"},
		"zatca_csr_org_identifier": {"300000000000003"},
		"zatca_csr_org_unit":      {"IT"},
		"zatca_csr_org_name":      {"شركة الاختبار ذ.م.م"},
		"zatca_csr_country":       {"SA"},
		"zatca_csr_location":      {"الرياض"},
		"zatca_csr_business_category": {"Retail"},
		"zatca_seller_vat":        {"399999999900003"},
		"zatca_seller_crn":        {"7000000000"},
	}

	req := httptest.NewRequest("POST", "/dashboard/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	// Verify the backend received the ZATCA payload
	if capturedPayload == nil {
		t.Fatal("ZATCA config was not sent to backend")
	}
	if v, ok := capturedPayload["zatca_otp"].(string); !ok || v != "123456" {
		t.Errorf("ZatcaOTP = %v, want '123456'", capturedPayload["zatca_otp"])
	}
	if v, ok := capturedPayload["csr_org_name"].(string); !ok || v != "شركة الاختبار ذ.م.م" {
		t.Errorf("CsrOrgName = %v, want 'شركة الاختبار ذ.م.م'", capturedPayload["csr_org_name"])
	}
}

func TestHandleSettingsRequiresAuth(t *testing.T) {
	req := httptest.NewRequest("GET", "/dashboard/settings", nil)
	// No access_token cookie
	w := httptest.NewRecorder()

	HandleSettings(w, req)

	// Should redirect to login
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (no auth), got %d", w.Code)
	}
}
