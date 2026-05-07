package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

// settingsDefaults holds the seed values used to initialise a per-token cache
// entry the first time we see a session. The actual cache is per-token (see
// settingsByToken) — there is NO global mutable map shared across users.
var settingsDefaults = map[string]string{
	"vat_rate":                "15",
	"currency":                "SAR",
	"language":                "ar",
	"date_format":             "DD/MM/YYYY",
	"theme":                   "light",
	"low_stock_threshold":     "10",
	"zatca_enabled":           "false",
	"invoice_prefix":          "INV-",
	"payment_terms":           "",
	"show_vat_breakdown":      "true",
	"auto_calculate_vat":      "true",
	"prices_include_vat":      "false",
	"pb_pdf_required":         "required",
	"default_payment_method":  "10",
	"invoice_footer":          "",
	"paper_size":              "A4",
	"print_copies":            "",
	"show_logo_print":         "true",
	"show_company_info_print": "true",
	"show_qr_print":           "true",
	"show_bank_details":       "false",
	"bank_details":            "",
	"number_format":           "ar",
	"notif_invoices":          "true",
	"notif_stock":             "true",
	"notif_payments":          "true",
	"notif_orders":            "true",
	"notif_session":           "true",
	"session_duration":        "",
	"max_login_attempts":      "",
	"require_strong_password": "true",
	"auto_logout_inactive":    "true",
	"default_unit":            "piece",
	"stock_enforcement":       "disable",
	"track_inventory":         "true",
	"allow_negative_stock":    "false",
	"show_cost_price":         "false",
	"company_name":            "",
	"company_email":           "",
	"company_vat":             "",
	"company_cr":              "",
	"company_description":     "",
	"company_address":         "",
	"company_phone":           "",
}

// tenantSettings is the per-token cache cell. mu protects values.
type tenantSettings struct {
	mu     sync.RWMutex
	values map[string]string
}

// settingsByToken keys on sha256(token) so we never keep raw tokens in memory
// for cache identity. Each entry is independent — settings written for user A
// are never visible to user B (different tenant / branch / role).
var settingsByToken sync.Map // map[string]*tenantSettings

func tokenKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func storeFor(token string) *tenantSettings {
	k := tokenKey(token)
	if v, ok := settingsByToken.Load(k); ok {
		return v.(*tenantSettings)
	}
	values := make(map[string]string, len(settingsDefaults))
	for kk, vv := range settingsDefaults {
		values[kk] = vv
	}
	ts := &tenantSettings{values: values}
	actual, _ := settingsByToken.LoadOrStore(k, ts)
	return actual.(*tenantSettings)
}

// settingsCategoryMap maps each settings key to the backend category
// it belongs to (for PUT /api/v2/settings).
var settingsCategoryMap = map[string]string{
	// appearance
	"language": "appearance", "date_format": "appearance",
	"theme": "appearance", "number_format": "appearance",
	// company
	"company_name": "company", "company_email": "company",
	"company_vat": "company", "company_cr": "company",
	"company_description": "company", "company_address": "company",
	"company_phone": "company",
	// inventory
	"low_stock_threshold": "inventory", "default_unit": "inventory",
	"stock_enforcement": "inventory", "track_inventory": "inventory",
	"allow_negative_stock": "inventory", "show_cost_price": "inventory",
	// invoice
	"vat_rate": "invoice", "currency": "invoice",
	"invoice_prefix": "invoice", "payment_terms": "invoice",
	"show_vat_breakdown": "invoice", "auto_calculate_vat": "invoice",
	"prices_include_vat": "invoice", "invoice_footer": "invoice",
	// notifications
	"notif_invoices": "notifications", "notif_stock": "notifications",
	"notif_payments": "notifications", "notif_orders": "notifications",
	"notif_session": "notifications",
	// print
	"paper_size": "print", "print_copies": "print",
	"show_logo_print": "print", "show_company_info_print": "print",
	"show_qr_print": "print", "show_bank_details": "print",
	"bank_details": "print",
	// security
	"session_duration": "security", "max_login_attempts": "security",
	"require_strong_password": "security", "auto_logout_inactive": "security",
}

// allSettingsKeys lists every key the template uses.
var allSettingsKeys = []string{
	"vat_rate", "currency", "language", "date_format", "theme",
	"low_stock_threshold", "zatca_enabled", "invoice_prefix", "payment_terms",
	"show_vat_breakdown", "auto_calculate_vat", "prices_include_vat",
	"pb_pdf_required", "default_payment_method", "invoice_footer",
	"paper_size", "print_copies", "show_logo_print", "show_company_info_print",
	"show_qr_print", "show_bank_details", "bank_details", "number_format",
	"notif_invoices", "notif_stock", "notif_payments", "notif_orders", "notif_session",
	"session_duration", "max_login_attempts", "require_strong_password", "auto_logout_inactive",
	"default_unit", "stock_enforcement", "track_inventory", "allow_negative_stock",
	"show_cost_price", "company_name", "company_email", "company_vat",
	"company_cr", "company_description", "company_address", "company_phone",
}

// loadSettingsFromBackend fetches settings from GET /api/v2/settings
// and merges them into the in-memory store.
func loadSettingsFromBackend(token string) {
	req, _ := http.NewRequest("GET", config.BackendDomain+"/api/v2/settings", nil)
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		log.Printf("[SETTINGS] Failed to fetch from backend: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[SETTINGS] Backend returned %d", resp.StatusCode)
		return
	}

	var result struct {
		Data map[string]map[string]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[SETTINGS] Failed to decode backend response: %v", err)
		return
	}

	ts := storeFor(token)
	ts.mu.Lock()
	for _, categorySettings := range result.Data {
		for key, value := range categorySettings {
			ts.values[key] = value
		}
	}
	ts.mu.Unlock()
	log.Printf("[SETTINGS] Loaded %d categories from backend", len(result.Data))

	// Also load notification config (separate endpoint, structured payload).
	overlayNotificationConfigIntoSettings(token)
}

// overlayNotificationConfigIntoSettings calls /api/v2/notification/config and
// projects its fields onto the flat settings map so the settings page renders
// the same source of truth as the notification system.
func overlayNotificationConfigIntoSettings(token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg, err := helpers.GetNotificationConfig(ctx, token)
	if err != nil {
		log.Printf("[SETTINGS] notification config fetch failed: %v", err)
		return
	}
	ts := storeFor(token)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.values["low_stock_threshold"] = strconv.Itoa(cfg.LowStockThreshold)
	ts.values["notif_stock"] = strconv.FormatBool(cfg.LowStockAlert)
	ts.values["notif_orders"] = strconv.FormatBool(cfg.NewOrderAlert)
	ts.values["notif_payments"] = strconv.FormatBool(cfg.PaymentDueAlert)
}

// saveSettingsToBackend sends changed settings to PUT /api/v2/settings
// grouped by category. Returns a non-nil error if any category failed so the
// caller can surface the failure to the user instead of pretending to succeed.
func saveSettingsToBackend(token string, settings map[string]string) error {
	// Group settings by category
	categories := map[string]map[string]string{}
	for key, value := range settings {
		cat, ok := settingsCategoryMap[key]
		if !ok {
			continue // frontend-only setting (zatca_enabled, pb_pdf_required, etc.)
		}
		if categories[cat] == nil {
			categories[cat] = map[string]string{}
		}
		categories[cat][key] = value
	}

	var firstErr error
	// Send one PUT per category
	for cat, catSettings := range categories {
		payload := map[string]interface{}{
			"category": cat,
			"settings": catSettings,
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("PUT", config.BackendDomain+"/api/v2/settings", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := helpers.DoAuthedRequest(req, token)
		if err != nil {
			log.Printf("[SETTINGS] Failed to save category %s: %v", cat, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			log.Printf("[SETTINGS] Backend error saving %s: %d %s", cat, resp.StatusCode, string(respBody))
			if firstErr == nil {
				firstErr = fmt.Errorf("backend status %d for category %s", resp.StatusCode, cat)
			}
		} else {
			log.Printf("[SETTINGS] Saved category %s (%d keys)", cat, len(catSettings))
		}
	}

	// Mirror notification-relevant keys into the structured /notification/config
	// endpoint so the low-stock generator sees them.
	mirrorNotificationConfig(token, settings)
	return firstErr
}

// mirrorNotificationConfig forwards the subset of the saved settings that
// drive the low-stock generator and per-channel toggles.
func mirrorNotificationConfig(token string, settings map[string]string) {
	partial := map[string]any{}
	if v, ok := settings["low_stock_threshold"]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			partial["low_stock_threshold"] = n
		}
	}
	if v, ok := settings["notif_stock"]; ok {
		partial["low_stock_alert"] = v == "true"
	}
	if v, ok := settings["notif_orders"]; ok {
		partial["new_order_alert"] = v == "true"
	}
	if v, ok := settings["notif_payments"]; ok {
		partial["payment_due_alert"] = v == "true"
	}
	if len(partial) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := helpers.UpdateNotificationConfig(ctx, token, partial); err != nil {
		log.Printf("[SETTINGS] notification config sync failed: %v", err)
	}
}

func getSettings(token string) map[string]string {
	ts := storeFor(token)
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	cp := make(map[string]string, len(ts.values))
	for k, v := range ts.values {
		cp[k] = v
	}
	return cp
}

// HandleSettingsPage displays the settings page.
func HandleSettingsPage(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	// Load settings from backend on first access (or refresh)
	loadSettingsFromBackend(token)

	branches, _ := helpers.FetchBranches(token)
	stores, _ := helpers.FetchStores(token)
	settings := getSettings(token)

	// Load ZATCA config per branch
	sessionID := helpers.GetSessionIDFromRequest(r)
	zatcaByBranch := map[int]map[string]string{}
	zatcaStatusByBranch := map[int]int{}
	for _, b := range branches {
		status, err := FetchZatcaConfigForBranch(sessionID, b.ID)
		if err == nil {
			zatcaByBranch[b.ID] = status.Config
			zatcaStatusByBranch[b.ID] = status.ZatcaStatus
		}
	}

	helpers.Render(w, r, "settings", map[string]interface{}{
		"title":               "الإعدادات",
		"Settings":            settings,
		"Branches":            branches,
		"Stores":              stores,
		"ZatcaByBranch":       zatcaByBranch,
		"ZatcaStatusByBranch": zatcaStatusByBranch,
	})
}

// HandleSaveSettings processes the settings form POST.
func HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadRequest, nil, "بيانات غير صالحة")
		return
	}

	// Checkbox fields — need special handling (unchecked = absent from form)
	checkboxKeys := []string{
		"show_vat_breakdown", "auto_calculate_vat", "prices_include_vat",
		"show_logo_print", "show_company_info_print", "show_qr_print", "show_bank_details",
		"notif_invoices", "notif_stock", "notif_payments", "notif_orders", "notif_session",
		"require_strong_password", "auto_logout_inactive",
		"track_inventory", "allow_negative_stock", "show_cost_price",
	}
	checkboxSet := make(map[string]bool, len(checkboxKeys))
	for _, k := range checkboxKeys {
		checkboxSet[k] = true
	}

	// Build the new settings map
	newSettings := make(map[string]string, len(allSettingsKeys))

	ts := storeFor(token)
	ts.mu.Lock()
	for _, k := range checkboxKeys {
		ts.values[k] = "false" // default unchecked
	}
	for _, key := range allSettingsKeys {
		val := r.FormValue(key)
		if val != "" {
			ts.values[key] = val
			newSettings[key] = val
		} else if checkboxSet[key] {
			newSettings[key] = "false"
		}
	}
	ts.mu.Unlock()

	// Persist to backend SYNCHRONOUSLY — a fire-and-forget goroutine would let
	// us flash "saved" even when every PUT returns 500. Surface the failure
	// to the user instead of lying to them.
	if err := saveSettingsToBackend(token, newSettings); err != nil {
		log.Printf("[SETTINGS] save failed: %v", err)
		writeFlashCookie(w, `{"message":"فشل حفظ الإعدادات","type":"error"}`)
		http.Redirect(w, r, "/dashboard/settings", http.StatusSeeOther)
		return
	}

	log.Printf("[SETTINGS] Settings saved successfully")

	// Set flash cookie for success toast, then do a standard HTTP redirect
	writeFlashCookie(w, `{"message":"تم حفظ الإعدادات بنجاح","type":"success"}`)
	http.Redirect(w, r, "/dashboard/settings", http.StatusSeeOther)
}

// writeFlashCookie sets the short-lived afrita_flash cookie used to render a
// toast on the next page render. Secure is enabled when not running on
// localhost so the cookie is only sent over HTTPS in deployed envs.
func writeFlashCookie(w http.ResponseWriter, payload string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "afrita_flash",
		Value:    url.QueryEscape(payload),
		Path:     "/",
		MaxAge:   10,
		HttpOnly: false,
		Secure:   !isLocalhost(),
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSettingValue returns a single setting value (for use by other handlers).
// GetSettingValue returns a single setting value scoped to the caller's token
// (one logical tenant / branch / user). Use this from other handlers when
// they already have a session token.
func GetSettingValue(token, key string) string {
	ts := storeFor(token)
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.values[key]
}

// Branch type alias to avoid import cycle — use models.Branch directly.
var _ = models.Branch{}
