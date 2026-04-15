package config

import (
	"afrita/resources"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// templateFuncs provides custom functions available in all templates.
var templateFuncs = template.FuncMap{
	"L":   resources.L,
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"mul": func(a, b interface{}) float64 {
		return toFloat64(a) * toFloat64(b)
	},
	"safeURL": func(s string) template.URL {
		return template.URL(s)
	},
	"derefInt": func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	},
	"derefStr": func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	},
	"formatAddress": func(addr string) string {
		return strings.TrimSpace(addr)
	},
	// dict creates a map from key-value pairs for passing data to sub-templates.
	// Usage: {{ template "my-component" dict "Title" "Hello" "Count" 5 }}
	"dict": func(pairs ...interface{}) map[string]interface{} {
		m := make(map[string]interface{}, len(pairs)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			key, _ := pairs[i].(string)
			m[key] = pairs[i+1]
		}
		return m
	},
	"json": func(v interface{}) template.JS {
		b, _ := json.Marshal(v)
		return template.JS(b)
	},
	// hasError checks if a field has a validation error.
	// Usage: {{ if hasError .errors "phone" }}...{{ end }}
	"hasError": func(errors interface{}, field string) bool {
		if errors == nil {
			return false
		}
		if m, ok := errors.(map[string]string); ok {
			_, exists := m[field]
			return exists
		}
		return false
	},
	// fieldError returns the error message for a specific field.
	// Usage: {{ fieldError .errors "phone" }}
	"fieldError": func(errors interface{}, field string) string {
		if errors == nil {
			return ""
		}
		if m, ok := errors.(map[string]string); ok {
			return m[field]
		}
		return ""
	},
	// old returns the previously submitted value for re-populating fields.
	// Usage: value="{{ old .old "name" }}"
	"old": func(oldValues interface{}, field string) string {
		if oldValues == nil {
			return ""
		}
		if m, ok := oldValues.(map[string]string); ok {
			return m[field]
		}
		return ""
	},
	// formatSAR formats a number as SAR currency (e.g., "١٬٢٣٤٫٥٦ ر.س")
	"formatSAR": func(amount interface{}) string {
		v := toFloat64(amount)
		return formatSAR(v)
	},
}

// toFloat64 converts any numeric type to float64 for template math operations.
func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	default:
		return 0
	}
}

// formatSAR formats a float as Saudi Riyal currency with Arabic-Indic numerals
// and the SAR symbol. Examples: 1234.5 → "١٬٢٣٤٫٥٠ ر.س", 0 → "٠٫٠٠ ر.س"
func formatSAR(amount float64) string {
	amount = math.Round(amount*100) / 100
	negative := amount < 0
	if negative {
		amount = -amount
	}

	// Split into integer and decimal parts
	intPart := int64(amount)
	decPart := int64(math.Round((amount - float64(intPart)) * 100))

	// Format integer part with grouping (thousands separator)
	intStr := fmt.Sprintf("%d", intPart)
	// Add grouping separators (every 3 digits from right)
	if len(intStr) > 3 {
		var grouped []string
		for len(intStr) > 3 {
			grouped = append([]string{intStr[len(intStr)-3:]}, grouped...)
			intStr = intStr[:len(intStr)-3]
		}
		grouped = append([]string{intStr}, grouped...)
		intStr = strings.Join(grouped, ",")
	}
	result := fmt.Sprintf("%s.%02d", intStr, decPart)

	// Convert to Arabic-Indic numerals
	arabicDigits := []rune{'٠', '١', '٢', '٣', '٤', '٥', '٦', '٧', '٨', '٩'}
	var arabicResult strings.Builder
	for _, ch := range result {
		switch {
		case ch >= '0' && ch <= '9':
			arabicResult.WriteRune(arabicDigits[ch-'0'])
		case ch == ',':
			arabicResult.WriteRune('٬') // Arabic thousands separator
		case ch == '.':
			arabicResult.WriteRune('٫') // Arabic decimal separator
		default:
			arabicResult.WriteRune(ch)
		}
	}

	prefix := ""
	if negative {
		prefix = "-"
	}
	return prefix + arabicResult.String() + " ر.س"
}

// Global configuration variables
var (
	AppDomain     string
	AppPort       string
	BackendDomain string
	AppVersion    string
	BaseDir       string // Project root directory (defaults to ".")
)

// Templates holds all pre-parsed templates keyed by name.
// Parsed once at startup — never re-read from disk on each request.
var Templates map[string]*template.Template

// Legacy aliases kept so dashboard.go compiles without changes.
var DashboardTemplate *template.Template
var DashboardTestTemplate *template.Template

// Session storage (in production, use Redis or database)
var (
	TokenStoreDir        string                       // Initialized in Initialize()
	SessionTokens        = make(map[string]string)    // sessionID -> accessToken
	SessionRefreshTokens = make(map[string]string)    // sessionID -> refreshToken
	SessionTokenExpiry   = make(map[string]time.Time) // sessionID -> expiryTime
	SessionTokensMutex   sync.RWMutex
)

// Initialize loads configuration from environment and sets up the application
func Initialize() {
	_ = godotenv.Load() // .env file is optional

	BackendDomain = os.Getenv("BACKEND_URL")
	if BackendDomain == "" {
		BackendDomain = "https://dev.ifritah.com"
	}

	AppDomain = os.Getenv("APP_DOMAIN")
	if AppDomain == "" {
		AppDomain = "dev.ifritah.com"
	}

	AppPort = os.Getenv("PORT")
	if AppPort == "" {
		AppPort = "8000"
	}

	// Load version from VERSION file
	versionBytes, err := os.ReadFile("VERSION")
	if err == nil {
		AppVersion = strings.TrimSpace(string(versionBytes))
	} else {
		AppVersion = "0.0.0"
	}

	log.Printf("Frontend: %s | Backend: %s | Version: %s", AppDomain, BackendDomain, AppVersion)

	// Initialize token store directory under OS user config dir (not world-readable /tmp)
	TokenStoreDir = os.Getenv("AFRITA_TOKEN_DIR")
	if TokenStoreDir == "" {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			userConfigDir = os.TempDir()
		}
		TokenStoreDir = filepath.Join(userConfigDir, "afrita", "tokens")
	}
	if err := os.MkdirAll(TokenStoreDir, 0700); err != nil {
		log.Printf("⚠️  Warning: Failed to create token store directory: %v", err)
	}

	log.Printf("🔐 Token storage initialized at: %s", TokenStoreDir)
}

// LoadTemplates pre-parses every template at startup into the Templates map.
// Each page that uses the base layout is parsed as base.html + page.html.
// Standalone pages (login, register, etc.) are parsed individually.
// Partials are parsed individually for HTMX fragment responses.
func LoadTemplates() {
	Templates = make(map[string]*template.Template)

	if BaseDir == "" {
		BaseDir = "."
	}

	base := filepath.Join(BaseDir, "templates/layouts/base.html")

	// ── Pages with base layout ─────────────────────────────────────
	layoutPages := map[string]string{
		"dashboard":             filepath.Join(BaseDir, "templates/dashboard.html"),
		"invoices":              filepath.Join(BaseDir, "templates/invoices.html"),
		"add-invoice":           filepath.Join(BaseDir, "templates/add-invoice.html"),
		"add-credit-note":       filepath.Join(BaseDir, "templates/add-credit-note.html"),
		"invoice-detail":        filepath.Join(BaseDir, "templates/invoice-detail.html"),
		"edit-invoice":          filepath.Join(BaseDir, "templates/edit-invoice.html"),
		"credit-invoice-detail": filepath.Join(BaseDir, "templates/credit-invoice-detail.html"),
		"products":              filepath.Join(BaseDir, "templates/products.html"),
		"add-product":           filepath.Join(BaseDir, "templates/add-product.html"),
		"product-detail":        filepath.Join(BaseDir, "templates/product-detail.html"),
		"edit-product":          filepath.Join(BaseDir, "templates/edit-product.html"),
		"clients":               filepath.Join(BaseDir, "templates/clients.html"),
		"add-client":            filepath.Join(BaseDir, "templates/add-client.html"),
		"client-detail":         filepath.Join(BaseDir, "templates/client-detail.html"),
		"edit-client":           filepath.Join(BaseDir, "templates/edit-client.html"),
		"orders":                filepath.Join(BaseDir, "templates/orders.html"),
		"add-order":             filepath.Join(BaseDir, "templates/add-order.html"),
		"branches":              filepath.Join(BaseDir, "templates/branches.html"),
		"add-branch":            filepath.Join(BaseDir, "templates/add-branch.html"),
		"branch-detail":         filepath.Join(BaseDir, "templates/branch-detail.html"),
		"edit-branch":           filepath.Join(BaseDir, "templates/edit-branch.html"),
		"stores":                filepath.Join(BaseDir, "templates/stores.html"),
		"add-store":             filepath.Join(BaseDir, "templates/add-store.html"),
		"store-detail":          filepath.Join(BaseDir, "templates/store-detail.html"),
		"edit-store":            filepath.Join(BaseDir, "templates/edit-store.html"),
		"suppliers":             filepath.Join(BaseDir, "templates/suppliers.html"),
		"add-supplier":          filepath.Join(BaseDir, "templates/add-supplier.html"),
		"supplier-detail":       filepath.Join(BaseDir, "templates/supplier-detail.html"),
		"edit-supplier":         filepath.Join(BaseDir, "templates/edit-supplier.html"),
		"purchase-bills":        filepath.Join(BaseDir, "templates/purchase-bills.html"),
		"add-purchase-bill":     filepath.Join(BaseDir, "templates/add-purchase-bill.html"),
		"purchase-bill-detail":  filepath.Join(BaseDir, "templates/purchase-bill-detail.html"),
		"edit-purchase-bill":    filepath.Join(BaseDir, "templates/edit-purchase-bill.html"),
		"add-user":              filepath.Join(BaseDir, "templates/add-user.html"),
		"users":                 filepath.Join(BaseDir, "templates/users.html"),
		"edit-user":             filepath.Join(BaseDir, "templates/edit-user.html"),
		"settings":              filepath.Join(BaseDir, "templates/settings.html"),
		"parts-search":          filepath.Join(BaseDir, "templates/parts-search.html"),
		"cars-search":           filepath.Join(BaseDir, "templates/cars-search.html"),
		"import-bills":          filepath.Join(BaseDir, "templates/import-bills.html"),
		"cash-vouchers":         filepath.Join(BaseDir, "templates/cash-vouchers.html"),
		"add-cash-voucher":      filepath.Join(BaseDir, "templates/add-cash-voucher.html"),
		"cash-voucher-detail":   filepath.Join(BaseDir, "templates/cash-voucher-detail.html"),
		"edit-cash-voucher":     filepath.Join(BaseDir, "templates/edit-cash-voucher.html"),
		"stock-adjustments":     filepath.Join(BaseDir, "templates/stock-adjustments.html"),
		"notifications":         filepath.Join(BaseDir, "templates/notifications.html"),
		"zatca-monitor":         filepath.Join(BaseDir, "templates/zatca-monitor.html"),
	}

	for name, page := range layoutPages {
		// Include base layout + shared component partials + page template
		files := []string{base, page}
		componentGlob := filepath.Join(BaseDir, "templates/components/*.html")
		if components, err := filepath.Glob(componentGlob); err == nil {
			files = append(files, components...)
		}
		t, err := template.New("").Funcs(templateFuncs).ParseFiles(files...)
		if err != nil {
			log.Printf("⚠️  Template parse error (%s): %v", name, err)
			continue
		}
		Templates[name] = t
	}

	// ── Standalone pages (no base layout) ──────────────────────────
	standalonePages := map[string]string{
		"login":           filepath.Join(BaseDir, "templates/login.html"),
		"register":        filepath.Join(BaseDir, "templates/register.html"),
		"forgot-password": filepath.Join(BaseDir, "templates/forgot-password.html"),
		"invoice-preview": filepath.Join(BaseDir, "templates/invoice-preview.html"),
		"invoice-print":   filepath.Join(BaseDir, "templates/invoice-print.html"),
		"error-page":      filepath.Join(BaseDir, "templates/error-page.html"),
	}

	for name, page := range standalonePages {
		files := []string{page}
		componentGlob := filepath.Join(BaseDir, "templates/components/*.html")
		if components, err := filepath.Glob(componentGlob); err == nil {
			files = append(files, components...)
		}
		t, err := template.New("").Funcs(templateFuncs).ParseFiles(files...)
		if err != nil {
			log.Printf("⚠️  Template parse error (%s): %v", name, err)
			continue
		}
		Templates[name] = t
	}

	// ── Partials (HTMX fragments) ─────────────────────────────────
	partials := map[string]string{
		"vin-result":      filepath.Join(BaseDir, "templates/partials/vin-result.html"),
		"parts-results":   filepath.Join(BaseDir, "templates/partials/parts-results.html"),
		"cars-results":    filepath.Join(BaseDir, "templates/partials/cars-results.html"),
		"stock-movements": filepath.Join(BaseDir, "templates/partials/stock-movements.html"),
	}

	for name, page := range partials {
		t, err := template.New("").Funcs(templateFuncs).ParseFiles(page)
		if err != nil {
			log.Printf("⚠️  Template parse error (%s): %v", name, err)
			continue
		}
		Templates[name] = t
	}

	// Legacy aliases for dashboard handler
	DashboardTemplate = Templates["dashboard"]
	DashboardTestTemplate = Templates["dashboard"]

	log.Printf("✅ Loaded %d templates (cached at startup)", len(Templates))
}
