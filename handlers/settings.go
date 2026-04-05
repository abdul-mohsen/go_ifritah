package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

// settingsStore holds in-memory settings. On startup it uses defaults,
// then gets overwritten by the backend on first load.
var (
	settingsMu      sync.RWMutex
	settingsLoaded  bool
	settingsStore   = map[string]string{
		"vat_rate":               "15",
		"currency":               "SAR",
		"language":               "ar",
		"date_format":            "DD/MM/YYYY",
		"theme":                  "light",
		"low_stock_threshold":    "10",
		"zatca_enabled":          "false",
		"invoice_prefix":         "INV-",
		"payment_terms":          "",
		"show_vat_breakdown":     "true",
		"auto_calculate_vat":     "true",
		"prices_include_vat":     "false",
		"pb_pdf_required":        "required",
		"default_payment_method": "10",
		"invoice_footer":         "",
		"paper_size":             "A4",
		"print_copies":           "",
		"show_logo_print":        "true",
		"show_company_info_print": "true",
		"show_qr_print":          "true",
		"show_bank_details":      "false",
		"bank_details":           "",
		"number_format":          "ar",
		"notif_invoices":         "true",
		"notif_stock":            "true",
		"notif_payments":         "true",
		"notif_orders":           "true",
		"notif_session":          "true",
		"session_duration":       "",
		"max_login_attempts":     "",
		"require_strong_password": "true",
		"auto_logout_inactive":   "true",
		"default_unit":           "piece",
		"stock_enforcement":      "disable",
		"track_inventory":        "true",
		"allow_negative_stock":   "false",
		"show_cost_price":        "false",
		"company_name":           "",
		"company_email":          "",
		"company_vat":            "",
		"company_cr":             "",
		"company_description":    "",
		"company_address":        "",
		"company_phone":          "",
	}
)

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

	settingsMu.Lock()
	defer settingsMu.Unlock()

	// Flatten category→key→value into settingsStore
	for _, categorySettings := range result.Data {
		for key, value := range categorySettings {
			settingsStore[key] = value
		}
	}
	settingsLoaded = true
	log.Printf("[SETTINGS] Loaded %d categories from backend", len(result.Data))
}

// saveSettingsToBackend sends changed settings to PUT /api/v2/settings
// grouped by category.
func saveSettingsToBackend(token string, settings map[string]string) {
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
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			log.Printf("[SETTINGS] Backend error saving %s: %d %s", cat, resp.StatusCode, string(respBody))
		} else {
			log.Printf("[SETTINGS] Saved category %s (%d keys)", cat, len(catSettings))
		}
	}
}

func getSettings() map[string]string {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	cp := make(map[string]string, len(settingsStore))
	for k, v := range settingsStore {
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
	settings := getSettings()

	helpers.Render(w, r, "settings", map[string]interface{}{
		"title":         "الإعدادات",
		"Settings":      settings,
		"Branches":      branches,
		"Stores":        stores,
		"ZatcaByBranch": map[int]map[string]string{},
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

	settingsMu.Lock()
	for _, k := range checkboxKeys {
		settingsStore[k] = "false" // default unchecked
	}
	for _, key := range allSettingsKeys {
		val := r.FormValue(key)
		if val != "" {
			settingsStore[key] = val
			newSettings[key] = val
		} else if checkboxSet[key] {
			newSettings[key] = "false"
		}
	}
	settingsMu.Unlock()

	// Persist to backend
	go saveSettingsToBackend(token, newSettings)

	log.Printf("[SETTINGS] Settings saved successfully")

	// Set flash cookie for success toast, then do a standard HTTP redirect
	http.SetCookie(w, &http.Cookie{
		Name:     "afrita_flash",
		Value:    url.QueryEscape(`{"message":"تم حفظ الإعدادات بنجاح","type":"success"}`),
		Path:     "/",
		MaxAge:   10,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/dashboard/settings", http.StatusSeeOther)
}

// GetSettingValue returns a single setting value (for use by other handlers).
func GetSettingValue(key string) string {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settingsStore[key]
}

// Branch type alias to avoid import cycle — use models.Branch directly.
var _ = models.Branch{}
