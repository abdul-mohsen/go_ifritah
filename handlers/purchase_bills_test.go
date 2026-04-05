package handlers

import (
	"afrita/config"
	"afrita/helpers"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// TestAddPurchaseBillPageHasManualSection verifies the add-purchase-bill form
// has a manual products section with add button.
func TestAddPurchaseBillPageHasManualSection(t *testing.T) {
	// Create a mock backend that returns empty stores/suppliers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	// Set up test session
	config.SessionTokensMutex.Lock()
	config.SessionTokens["pb-test-session"] = "pb-test-token"
	config.SessionTokensMutex.Unlock()
	defer func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "pb-test-session")
		config.SessionTokensMutex.Unlock()
	}()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/add", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-test-session"})
	w := httptest.NewRecorder()

	HandleAddPurchaseBill(w, req)

	body := w.Body.String()

	// Must have manual products section
	if !strings.Contains(body, "قطع يدوية") {
		t.Error("expected manual products section header 'قطع يدوية' in add-purchase-bill page")
	}

	// Must have manual products container
	if !strings.Contains(body, `id="manual-container"`) {
		t.Error("expected manual-container div in add-purchase-bill page")
	}

	// Must have add manual item button
	if !strings.Contains(body, "addManualItem") {
		t.Error("expected addManualItem button in add-purchase-bill page")
	}

	// Must have manual form fields in the JS template
	if !strings.Contains(body, "manual_part_name") {
		t.Error("expected manual_part_name input field in add-purchase-bill page")
	}
}

// TestAddPurchaseBillTotalIncludesManual verifies the JS recalculateTotal
// function sums both catalog and manual items.
func TestAddPurchaseBillTotalIncludesManual(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	config.SessionTokensMutex.Lock()
	config.SessionTokens["pb-test-session"] = "pb-test-token"
	config.SessionTokensMutex.Unlock()
	defer func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "pb-test-session")
		config.SessionTokensMutex.Unlock()
	}()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/add", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-test-session"})
	w := httptest.NewRecorder()

	HandleAddPurchaseBill(w, req)

	body := w.Body.String()

	// JS must have sumManualItems function
	if !strings.Contains(body, "sumManualItems") {
		t.Error("expected sumManualItems function in add-purchase-bill JS")
	}

	// recalculateTotal must reference both sumCatalogItems and sumManualItems
	if !strings.Contains(body, "sumCatalogItems") || !strings.Contains(body, "sumManualItems") {
		t.Error("expected recalculateTotal to use both sumCatalogItems and sumManualItems")
	}
}

// TestAddPurchaseBillPageHasFileUpload verifies the add form has
// mandatory PDF upload and optional documents upload fields.
func TestAddPurchaseBillPageHasFileUpload(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	config.SessionTokensMutex.Lock()
	config.SessionTokens["pb-upload-test"] = "pb-upload-token"
	config.SessionTokensMutex.Unlock()
	defer func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "pb-upload-test")
		config.SessionTokensMutex.Unlock()
	}()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/add", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-upload-test"})
	w := httptest.NewRecorder()

	HandleAddPurchaseBill(w, req)

	body := w.Body.String()

	// Must have mandatory bill_pdf file input
	if !strings.Contains(body, `name="bill_pdf"`) {
		t.Error("expected mandatory bill_pdf file input in add-purchase-bill page")
	}

	// Must have optional documents file input
	if !strings.Contains(body, `name="documents"`) {
		t.Error("expected optional documents file input in add-purchase-bill page")
	}

	// Must have multipart form encoding for HTMX
	if !strings.Contains(body, `hx-encoding="multipart/form-data"`) {
		t.Error("expected hx-encoding='multipart/form-data' on the form")
	}

	// Must have required attribute on bill_pdf
	if !strings.Contains(body, "accept=\".pdf\"") {
		t.Error("expected accept='.pdf' on bill_pdf input")
	}

	// Arabic label for mandatory PDF
	if !strings.Contains(body, "فاتورة الشراء") {
		t.Error("expected Arabic label for mandatory purchase bill PDF upload")
	}

	// Arabic label for optional documents
	if !strings.Contains(body, "مستندات إضافية") {
		t.Error("expected Arabic label for optional documents upload")
	}
}

// pbDetailMockBackend creates a mock backend that returns a purchase bill detail
// with the given pdf_link and attachments. Also handles store/supplier list calls.
func pbDetailMockBackend(billID, pdfLink string, attachments []string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		path := r.URL.Path
		if strings.Contains(path, "/api/v2/store/all") ||
			strings.Contains(path, "/api/v2/supplier/all") {
			w.Write([]byte(`{"data":[]}`))
			return
		}

		if strings.Contains(path, "/api/v2/purchase_bill/"+billID) {
			detail := map[string]interface{}{
				"id":         billID,
				"bill_type":  5,
				"state":      0,
				"sub_total":  100.0,
				"discount":   0,
				"total_vat":  15.0,
				"total":      115.0,
				"products":   []interface{}{},
				"store_id":   1,
				"pdf_link":   pdfLink,
			}
			if attachments != nil {
				detail["attachments"] = attachments
			}
			json.NewEncoder(w).Encode(detail)
			return
		}

		w.Write([]byte(`{"data":[]}`))
	}))
}

// setupPBTestSession creates a test session and returns a cleanup function.
func setupPBTestSession(sessionID, token string) func() {
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = token
	config.SessionTokensMutex.Unlock()
	return func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	}
}

// TestPBDetailShowsPdfLinkFromBackend verifies that when the backend returns a
// real pdf_link (with file extension), it is shown in the detail page.
func TestPBDetailShowsPdfLinkFromBackend(t *testing.T) {
	backend := pbDetailMockBackend("999", "/api/v2/files/abc123def456.pdf", nil)
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-pdf-test", "pb-pdf-token")
	defer cleanup()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/999", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-pdf-test"})
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	w := httptest.NewRecorder()

	HandleGetPurchaseBill(w, req)

	body := w.Body.String()

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", w.Code, body[:min(500, len(body))])
	}

	// The PDF link should appear in the rendered page
	if !strings.Contains(body, "abc123def456.pdf") {
		t.Error("expected pdf_link with file extension to be shown in purchase bill detail page")
	}
}

// TestPBDetailHidesJunkPdfLink verifies that when the backend returns a junk
// pdf_link (no file extension), it is NOT shown in the detail page.
func TestPBDetailHidesJunkPdfLink(t *testing.T) {
	backend := pbDetailMockBackend("888", "/api/v2/files/3dd", nil)
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-junk-test", "pb-junk-token")
	defer cleanup()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/888", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-junk-test"})
	req = mux.SetURLVars(req, map[string]string{"id": "888"})
	w := httptest.NewRecorder()

	HandleGetPurchaseBill(w, req)

	body := w.Body.String()

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", w.Code, body[:min(500, len(body))])
	}

	// The junk PDF link should NOT appear
	if strings.Contains(body, "/api/v2/files/3dd") || strings.Contains(body, "files/3dd") {
		t.Error("junk pdf_link '/api/v2/files/3dd' should NOT be shown in purchase bill detail page")
	}
}

// TestAddPBNoDefaultProductRow verifies that the add purchase bill form does
// NOT auto-add an empty product row on page load.
func TestAddPBNoDefaultProductRow(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-norow-test", "pb-norow-token")
	defer cleanup()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/add", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-norow-test"})
	w := httptest.NewRecorder()

	HandleAddPurchaseBill(w, req)

	body := w.Body.String()

	// There must NOT be a standalone addItem(); call before recalculateTotal.
	// Pattern: a line that is just "addItem();" (with optional whitespace) followed by recalculateTotal.
	// The existing code has:
	//     addItem();
	//     recalculateTotal();
	// We want only recalculateTotal() without a preceding standalone addItem().
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "addItem();" {
			// Check it's not inside a conditional (forEach, if block)
			// A standalone addItem(); is one that appears at the top level of the script
			// Look at previous non-empty line to see if it's inside a block
			prevLine := ""
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) != "" {
					prevLine = strings.TrimSpace(lines[j])
					break
				}
			}
			// If previous line ends with { or is a forEach call, it's inside a block
			if strings.HasSuffix(prevLine, "{") || strings.Contains(prevLine, "forEach") {
				continue
			}
			t.Errorf("found standalone 'addItem();' at line %d — should not auto-add empty product row", i+1)
		}
	}
}

// TestEditPBManualProductNameField verifies that the edit template JS reads
// product.name (matching BillItem JSON tag) for manual product names.
func TestEditPBManualProductNameField(t *testing.T) {
	// Create mock backend returning a PB with a manual product
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		if strings.Contains(path, "/api/v2/store/all") ||
			strings.Contains(path, "/api/v2/supplier/all") ||
			strings.Contains(path, "/api/v2/product/all") ||
			strings.Contains(path, "/api/v2/purchase_bill/all") {
			w.Write([]byte(`{"data":[]}`))
			return
		}

		if strings.Contains(path, "/api/v2/purchase_bill/") {
			detail := map[string]interface{}{
				"id":              "777",
				"bill_type":       5,
				"state":           0,
				"sub_total":       50.0,
				"discount":        0,
				"products":        []interface{}{},
				"manual_products": []interface{}{},
				"store_id":        1,
			}
			json.NewEncoder(w).Encode(detail)
			return
		}

		w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-editname-test", "pb-editname-token")
	defer cleanup()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/edit/777", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-editname-test"})
	req = mux.SetURLVars(req, map[string]string{"id": "777"})
	w := httptest.NewRecorder()

	HandleEditPurchaseBill(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", w.Code, body[:min(500, len(body))])
	}

	// The manualItemRow JS must read product.name (matching BillItem JSON tag "name")
	// Check for the specific pattern: product.name || product.part_name
	if !strings.Contains(body, "product.name || product.part_name") {
		t.Error("edit template manualItemRow should use 'product.name || product.part_name' to match BillItem JSON tag")
	}
}

// TestPBDetailShowsSupplierDetails verifies that the purchase bill detail page
// shows full supplier details (phone, email, VAT, CR, etc.) — not just name.
func TestPBDetailShowsSupplierDetails(t *testing.T) {
	// Clear cache so our mock data is used (not stale cache from other tests)
	helpers.APICache.Delete("suppliers")
	helpers.APICache.Delete("stores")

	supplier := map[string]interface{}{
		"id":                      42,
		"name":                    "شركة الفاخرة للقطع",
		"email":                   "info@fakhera.sa",
		"phone_number":            "0551234567",
		"address":                 "شارع الملك فهد",
		"short_address":           "RBAD1234",
		"vat_number":              "300012345678901",
		"commercial_registration": "1010123456",
		"bank_account":            "SA1234567890123456789012",
		"is_post_paid":            true,
		"credit_limit":            50000,
		"payment_terms_days":      30,
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		if strings.Contains(path, "/api/v2/store/all") {
			w.Write([]byte(`{"data":[]}`))
			return
		}
		if strings.Contains(path, "/api/v2/supplier/all") {
			resp := map[string]interface{}{"data": []interface{}{supplier}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.Contains(path, "/api/v2/purchase_bill/") {
			detail := map[string]interface{}{
				"id":          "500",
				"bill_type":   5,
				"state":       0,
				"sub_total":   1000.0,
				"discount":    0,
				"total_vat":   150.0,
				"total":       1150.0,
				"products":    []interface{}{},
				"store_id":    1,
				"merchant_id": 1,
				"supplier_id": 42, // matches supplier ID
			}
			json.NewEncoder(w).Encode(detail)
			return
		}
		w.Write([]byte(`{"data":[]}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-supplier-test", "pb-supplier-token")
	defer cleanup()

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills/500", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-supplier-test"})
	req = mux.SetURLVars(req, map[string]string{"id": "500"})
	w := httptest.NewRecorder()

	HandleGetPurchaseBill(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. body: %s", w.Code, body[:min(500, len(body))])
	}

	// Supplier name must appear
	if !strings.Contains(body, "شركة الفاخرة للقطع") {
		t.Error("expected supplier name in purchase bill detail page")
	}

	// Supplier phone must appear
	if !strings.Contains(body, "0551234567") {
		t.Error("expected supplier phone number in purchase bill detail page")
	}

	// Supplier email must appear
	if !strings.Contains(body, "info@fakhera.sa") {
		t.Error("expected supplier email in purchase bill detail page")
	}

	// Supplier VAT number must appear
	if !strings.Contains(body, "300012345678901") {
		t.Error("expected supplier VAT number in purchase bill detail page")
	}

	// Supplier CR must appear
	if !strings.Contains(body, "1010123456") {
		t.Error("expected supplier commercial registration in purchase bill detail page")
	}

	// Supplier bank account must appear
	if !strings.Contains(body, "SA1234567890123456789012") {
		t.Error("expected supplier bank account in purchase bill detail page")
	}

	// Credit limit must appear (supplier is postpaid)
	if !strings.Contains(body, "50000") {
		t.Error("expected supplier credit limit in purchase bill detail page")
	}
}

// min returns the smaller of two ints (needed for Go 1.21 compat).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ensure fmt is used
var _ = fmt.Sprintf
