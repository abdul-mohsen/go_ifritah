package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestAllTemplatesParse verifies that every template in the project
// parses without errors when LoadTemplates is called.
func TestAllTemplatesParse(t *testing.T) {
	// Set BaseDir to the project root (one level up from config/)
	abs, _ := filepath.Abs("..")
	BaseDir = abs

	LoadTemplates()

	// All expected templates
	expected := []string{
		// Layout pages
		"dashboard", "invoices", "add-invoice", "add-credit-note",
		"invoice-detail", "edit-invoice", "credit-invoice-detail",
		"products", "add-product", "product-detail", "edit-product",
		"clients", "add-client", "client-detail", "edit-client",
		"orders", "add-order",
		"branches", "add-branch", "branch-detail", "edit-branch",
		"stores", "add-store", "store-detail", "edit-store",
		"suppliers", "add-supplier", "supplier-detail", "edit-supplier",
		"purchase-bills", "add-purchase-bill", "purchase-bill-detail", "edit-purchase-bill",
		"add-user", "users", "edit-user",
		"settings", "parts-search", "cars-search", "import-bills",
		"cash-vouchers", "add-cash-voucher", "cash-voucher-detail", "edit-cash-voucher",
		"stock-adjustments", "notifications",
		// Standalone pages
		"login", "register", "forgot-password",
		"invoice-preview", "invoice-print", "error-page",
		// Partials
		"vin-result", "parts-results", "cars-results", "stock-movements",
	}

	for _, name := range expected {
		tmpl, ok := Templates[name]
		if !ok || tmpl == nil {
			t.Errorf("Template %q failed to parse or is missing from Templates map", name)
		}
	}

	if len(Templates) != len(expected) {
		t.Errorf("Expected %d templates, got %d", len(expected), len(Templates))
	}
}

// TestAllTemplatesExecute verifies that every template can be executed
// (rendered to HTML) with minimal sample data without panicking or erroring.
func TestAllTemplatesExecute(t *testing.T) {
	abs, _ := filepath.Abs("..")
	BaseDir = abs

	LoadTemplates()

	// Minimal data that satisfies the most common template variables.
	// Templates use {{ .field }} which returns zero-value "" for missing keys
	// in map[string]interface{}, so we only need to supply data that templates
	// iterate over (range) or call methods on.
	minimalData := map[string]interface{}{
		"title":   "Test",
		"version": "0.0.0-test",

		// Common page data
		"invoices":        []interface{}{},
		"products":        []interface{}{},
		"clients":         []interface{}{},
		"suppliers":       []interface{}{},
		"stores":          []interface{}{},
		"orders":          []interface{}{},
		"branches":        []interface{}{},
		"users":           []interface{}{},
		"all_products":    []interface{}{},
		"manual_products": []interface{}{},

		// Pagination
		"pagination": map[string]interface{}{
			"Page":    1,
			"PerPage": 20,
			"Total":   0,
		},
		"prev_page":    0,
		"next_page":    0,
		"query":        "",
		"extra_params": "",

		// Invoice specific
		"invoice": map[string]interface{}{
			"ID":              0,
			"SequenceNumber":  0,
			"StoreID":         0,
			"Products":        []interface{}{},
			"ManualProducts":  []interface{}{},
			"TotalAmount":     0.0,
			"Discount":        0.0,
			"MaintenanceCost": 0.0,
			"State":           0,
			"UserName":        "",
			"UserPhoneNumber": "",
			"Note":            "",
		},
		"id":                  "0",
		"status":              "",
		"status_class":        "",
		"type":                "",
		"effective_date":      "",
		"payment_due_date":    "",
		"deliver_date":        "",
		"bill_payment_method": "",
		"store_name":          "",
		"company_name":        "",
		"vat_registration":    "",
		"commercial_reg":      "",
		"user_name":           "",
		"user_phone":          "",
		"note":                "",
		"maintenance_cost":    0.0,
		"url":                 "",
		"is_draft":            false,
		"total_display":       "0.00",

		// Product specific
		"product": map[string]interface{}{
			"ID":       0,
			"PartName": "",
			"Price":    0.0,
			"Quantity": 0,
		},

		// Client specific
		"client": map[string]interface{}{
			"ID":   0,
			"Name": "",
		},

		// Supplier specific
		"supplier": map[string]interface{}{
			"ID":   0,
			"Name": "",
		},

		// Store specific
		"store": map[string]interface{}{
			"ID":   0,
			"Name": "",
		},
		"address": map[string]interface{}{
			"BuildingNumber":     "",
			"StreetName":         "",
			"District":           "",
			"City":               "",
			"PostalCode":         "",
			"AdditionalNumber":   "",
			"CountryCode":        "",
		},

		// Branch specific
		"branch": map[string]interface{}{
			"ID":   0,
			"Name": "",
		},

		// User specific
		"user": map[string]interface{}{
			"ID":          0,
			"Username":    "",
			"Email":       "",
			"Role":        "",
			"Active":      true,
			"Permissions": []interface{}{},
		},

		// Dashboard specific
		"stats":            map[string]interface{}{},
		"recentInvoices":   []interface{}{},
		"purchaseBills":    []interface{}{},
		"monthLabels":      []string{},
		"monthlyRevenue":   []float64{},
		"monthlyExpenses":  []float64{},
		"invoiceStateData": []interface{}{},
		"topClients":       []interface{}{},
		"kpi_trends": map[string]interface{}{
			"invoices":        map[string]interface{}{"value": 0, "change": 0.0, "direction": ""},
			"revenue":         map[string]interface{}{"value": 0, "change": 0.0, "direction": ""},
			"purchases_total": map[string]interface{}{"value": 0, "change": 0.0, "direction": ""},
			"gross_profit":    map[string]interface{}{"value": 0, "change": 0.0, "direction": ""},
		},

		// Common flags
		"is_company":             false,
		"today":                  "2026-01-01",
		"state":                  "0",
		"default_store_id":       0,
		"default_payment_method": "10",

		// Search
		"results":      []interface{}{},
		"cars_results": []interface{}{},
		"search_query": "",

		// CSV import
		"import_errors": []interface{}{},

		// Error page
		"error_code":    404,
		"error_message": "Not Found",

		// Settings
		"settings": map[string]interface{}{},
		"Settings": map[string]string{
			"vat_rate": "15", "currency": "SAR", "language": "ar",
			"date_format": "DD/MM/YYYY", "theme": "light",
			"low_stock_threshold": "10", "zatca_enabled": "false",
			"invoice_prefix": "INV-", "payment_terms": "",
			"show_vat_breakdown": "true", "auto_calculate_vat": "true",
			"prices_include_vat": "false", "pb_pdf_required": "required",
			"default_payment_method": "10", "invoice_footer": "",
			"paper_size": "A4", "print_copies": "",
			"show_logo_print": "true", "show_company_info_print": "true",
			"show_qr_print": "true", "show_bank_details": "false",
			"bank_details": "", "number_format": "ar",
			"notif_invoices": "true", "notif_stock": "true",
			"notif_payments": "true", "notif_orders": "true",
			"notif_session": "true", "session_duration": "",
			"max_login_attempts": "", "require_strong_password": "true",
			"auto_logout_inactive": "true", "default_unit": "piece",
			"stock_enforcement": "disable", "track_inventory": "true",
			"allow_negative_stock": "false", "show_cost_price": "false",
			"company_name": "", "company_email": "",
			"company_vat": "", "company_cr": "",
			"company_description": "",
		},
		"Branches":      []interface{}{},
		"ZatcaByBranch": map[int]map[string]string{},

		// Purchase bill
		"purchase_bill": map[string]interface{}{
			"ID":             0,
			"Products":       []interface{}{},
			"ManualProducts": []interface{}{},
		},

		// Cash voucher detail
		"voucher": map[string]interface{}{
			"voucher_type":   "payment",
			"recipient_type": "supplier",
			"payment_method": "cash",
			"store_id":       0,
		},
		"voucher_number":  0,
		"type_label":      "",
		"amount":          "0.00",
		"recipient_name":  "",
		"recipient_type":  "",
		"payment_method":  "",
		"notes":           "",
		"voucher_type":    "",

		// Credit note
		"credit_note": map[string]interface{}{},

		// Clients and Stores for modals
		"Clients": []interface{}{},
		"Stores":  []interface{}{},

		// User role for RBAC in sidebar
		"user_role": "admin",

		// Managers list for branch edit
		"managers": []interface{}{},
	}

	// Suppress template execution output
	devNull := io.Discard

	for name, tmpl := range Templates {
		t.Run(name, func(t *testing.T) {
			// Make a copy of the data map per template to avoid cross-contamination
			data := make(map[string]interface{}, len(minimalData))
			for k, v := range minimalData {
				data[k] = v
			}

			var buf bytes.Buffer

			// Determine which entry point to execute
			var err error
			if tmpl.Lookup("base.html") != nil {
				err = tmpl.ExecuteTemplate(&buf, "base.html", data)
			} else {
				// Standalone/partial templates: use "<name>.html"
				templateName := name + ".html"
				err = tmpl.ExecuteTemplate(&buf, templateName, data)
			}

			if err != nil {
				t.Errorf("Template %q execution failed: %v", name, err)
				return
			}

			// Basic sanity: output should not be empty for non-partial templates
			output := buf.String()
			if len(output) == 0 {
				t.Errorf("Template %q produced empty output", name)
			}

			// Suppress stdout noise
			_ = devNull
		})
	}
}

// TestTemplateCount verifies the expected number of templates are loaded
// and no template was silently dropped.
func TestTemplateCount(t *testing.T) {
	abs, _ := filepath.Abs("..")
	BaseDir = abs

	// Capture stderr to detect parse warnings
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	LoadTemplates()

	w.Close()
	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, r)
	os.Stderr = oldStderr

	if len(Templates) < 55 {
		t.Errorf("Expected at least 55 templates, got %d. Some templates failed to parse.", len(Templates))
		t.Logf("Stderr output during LoadTemplates:\n%s", stderrBuf.String())
	}
}
