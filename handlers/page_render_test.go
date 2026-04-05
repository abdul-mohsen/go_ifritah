package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"
	"afrita/helpers"

	"github.com/gorilla/mux"
)

// seedRenderTestSession creates a test session token for render tests.
func seedRenderTestSession(t *testing.T) {
	t.Helper()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["render-test"] = "mock-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "render-test")
		config.SessionTokensMutex.Unlock()
	})
}

// setupMockBackendAll creates a mock backend that handles all API endpoints
// used by page-rendering handlers. Returns a cleanup function.
func setupMockBackendAll(t *testing.T) {
	t.Helper()

	// Clear any cached data so mock backend is actually called
	helpers.APICache.Delete("products")
	helpers.APICache.Delete("clients")
	helpers.APICache.Delete("suppliers")
	helpers.APICache.Delete("stores")
	helpers.APICache.Delete("orders")
	helpers.APICache.Delete("invoices_all")
	helpers.APICache.Delete("purchase_bills")
	helpers.APICache.Delete("branches")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		// List endpoints
		case path == "/api/v2/bill/all":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id": 1, "sequence_number": 1001,
					"total": 100.0, "total_before_vat": 86.96, "total_vat": 13.04,
					"discount": 0.0, "state": 3, "credit_state": 0, "type": false,
					"effective_date": map[string]interface{}{"Time": "2026-01-01T00:00:00Z", "Valid": true},
				},
			})

		case path == "/api/v2/product/all":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "part_name": "قطعة تجريبية", "price": 50.0, "quantity": 10, "article_id": 100, "store_id": 1},
			})

		case path == "/api/v2/client/all":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "عميل تجريبي", "phone": "0500000000"},
			})

		case path == "/api/v2/supplier/all":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "مورد تجريبي", "phone_number": "0500000001"},
			})

		case path == "/api/v2/store/all":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "المستودع الرئيسي"},
			})

		case path == "/api/v2/order/all":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "customer_name": "عميل", "total": 200.0, "status": "pending"},
			})

		case path == "/api/v2/purchase_bill/all":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id": 1, "sequence_number": 2001, "total": 500.0,
					"discount": 0.0, "state": 3, "type": false,
					"effective_date": map[string]interface{}{"Time": "2026-01-15T00:00:00Z", "Valid": true},
				},
			})

		case path == "/api/v2/branch/all":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "الفرع الرئيسي", "address": "الرياض", "phone": "0500000000"},
			})

		// Detail endpoints
		case strings.HasPrefix(path, "/api/v2/bill/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "sequence_number": 1001,
				"total_amount": 100.0, "total_before_vat": 86.96, "total_vat": 13.04,
				"discount": 0.0, "state": 3, "credit_state": 0, "type": false,
				"products": []interface{}{
					map[string]interface{}{"product_id": 1, "name": "قطعة", "price": 50.0, "quantity": 2, "discount": 0},
				},
				"manual_products":   []interface{}{},
				"maintenance_cost":  0.0,
				"user_name":         "تجريبي",
				"user_phone_number": "0500000000",
				"note":              "",
				"store_id":          1,
				"store_name":        "المستودع",
				"effective_date":    "2026-01-01",
				"payment_method":    10,
				"payment_due_date":  "",
				"deliver_date":      "",
				"branch_id":         1,
			})

		case strings.HasPrefix(path, "/api/v2/product/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "part_name": "قطعة تجريبية", "price": 50.0, "quantity": 10, "store_id": 1,
			})

		case strings.HasPrefix(path, "/api/v2/client/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "name": "عميل تجريبي", "phone": "0500000000",
			})

		case strings.HasPrefix(path, "/api/v2/store/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "name": "المستودع الرئيسي",
			})

		case strings.HasPrefix(path, "/api/v2/purchase_bill/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "sequence_number": 2001,
				"total_amount": 500.0, "total_before_vat": 434.78, "total_vat": 65.22,
				"discount": 0.0, "state": 3, "type": false,
				"products":                []interface{}{},
				"manual_products":         []interface{}{},
				"store_id":                1,
				"merchant_id":             1,
				"supplier_sequence_number": 0,
				"effective_date":          "2026-01-15",
				"payment_method":          10,
			})

		case strings.HasPrefix(path, "/api/v2/credit_bill/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "sequence_number": 3001,
				"total_amount": 50.0, "total_before_vat": 43.48, "total_vat": 6.52,
				"discount": 0.0, "state": 3, "credit_state": 1,
				"products":        []interface{}{},
				"manual_products": []interface{}{},
				"effective_date":  "2026-01-10",
			})

		default:
			// Return empty JSON for any unknown endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))

	prevBackend := config.BackendDomain
	config.BackendDomain = server.URL
	t.Cleanup(func() {
		server.Close()
		config.BackendDomain = prevBackend
		// Clear cache after test
		helpers.APICache.Delete("products")
		helpers.APICache.Delete("clients")
		helpers.APICache.Delete("suppliers")
		helpers.APICache.Delete("stores")
		helpers.APICache.Delete("orders")
		helpers.APICache.Delete("invoices_all")
		helpers.APICache.Delete("purchase_bills")
	})
}

// TestAllPagesRender verifies that every page-rendering handler produces
// valid HTML output (HTTP 200) when given mock backend data.
func TestAllPagesRender(t *testing.T) {
	seedRenderTestSession(t)
	setupMockBackendAll(t)

	type pageTest struct {
		name    string
		handler http.HandlerFunc
		path    string
		vars    map[string]string // mux path variables for {id} routes
		noAuth  bool              // skip session cookie (public pages)
	}

	pages := []pageTest{
		// ── Auth / Public pages (standalone templates) ──
		{name: "login", handler: HandleLogin, path: "/login", noAuth: true},
		{name: "register", handler: HandleRegister, path: "/register", noAuth: true},
		{name: "forgot-password", handler: HandleForgotPassword, path: "/forgot-password", noAuth: true},

		// ── Dashboard ──
		{name: "dashboard", handler: HandleDashboard, path: "/dashboard"},

		// ── Invoice pages ──
		{name: "invoices-list", handler: HandleInvoices, path: "/dashboard/invoices"},
		{name: "add-invoice", handler: HandleAddInvoice, path: "/dashboard/invoices/add-invoice"},
		{name: "invoice-detail", handler: HandleGetInvoice, path: "/bill/1", vars: map[string]string{"id": "1"}},
		{name: "edit-invoice", handler: HandleEditInvoice, path: "/dashboard/invoices/edit/1", vars: map[string]string{"id": "1"}},
		{name: "add-credit-note", handler: HandleAddCreditNote, path: "/dashboard/invoices/credit/1", vars: map[string]string{"id": "1"}},
		{name: "invoice-preview", handler: HandleInvoicePreview, path: "/bill/1/preview", vars: map[string]string{"id": "1"}},
		{name: "import-bills", handler: HandleImportBillsPage, path: "/dashboard/invoices/import-csv"},

		// ── Purchase Bill pages ──
		{name: "purchase-bills-list", handler: HandlePurchaseBills, path: "/dashboard/purchase-bills"},
		{name: "add-purchase-bill", handler: HandleAddPurchaseBill, path: "/dashboard/purchase-bills/add"},
		{name: "purchase-bill-detail", handler: HandleGetPurchaseBill, path: "/dashboard/purchase-bills/1", vars: map[string]string{"id": "1"}},
		{name: "edit-purchase-bill", handler: HandleEditPurchaseBill, path: "/dashboard/purchase-bills/edit/1", vars: map[string]string{"id": "1"}},

		// ── Product pages ──
		{name: "products-list", handler: HandleProducts, path: "/dashboard/products"},
		{name: "add-product", handler: HandleAddProduct, path: "/dashboard/products/add"},
		{name: "product-detail", handler: HandleProductDetail, path: "/dashboard/products/1", vars: map[string]string{"id": "1"}},
		{name: "edit-product", handler: HandleEditProduct, path: "/dashboard/products/1/edit", vars: map[string]string{"id": "1"}},

		// ── Client pages ──
		{name: "clients-list", handler: HandleClients, path: "/dashboard/clients"},
		{name: "add-client", handler: HandleAddClient, path: "/dashboard/clients/add"},
		{name: "client-detail", handler: HandleClientDetail, path: "/dashboard/clients/1", vars: map[string]string{"id": "1"}},
		{name: "edit-client", handler: HandleEditClient, path: "/dashboard/clients/1/edit", vars: map[string]string{"id": "1"}},

		// ── Supplier pages ──
		{name: "suppliers-list", handler: HandleSuppliers, path: "/dashboard/suppliers"},
		{name: "add-supplier", handler: HandleAddSupplier, path: "/dashboard/suppliers/add"},
		{name: "supplier-detail", handler: HandleSupplierDetail, path: "/dashboard/suppliers/1", vars: map[string]string{"id": "1"}},
		{name: "edit-supplier", handler: HandleEditSupplier, path: "/dashboard/suppliers/1/edit", vars: map[string]string{"id": "1"}},

		// ── Store pages ──
		{name: "stores-list", handler: HandleStores, path: "/dashboard/stores"},
		{name: "add-store", handler: HandleAddStore, path: "/dashboard/stores/add"},
		{name: "store-detail", handler: HandleStoreDetail, path: "/dashboard/stores/1", vars: map[string]string{"id": "1"}},
		{name: "edit-store", handler: HandleEditStore, path: "/dashboard/stores/1/edit", vars: map[string]string{"id": "1"}},

		// ── Order pages ──
		{name: "orders-list", handler: HandleOrders, path: "/dashboard/orders"},
		{name: "add-order", handler: HandleAddOrder, path: "/dashboard/orders/add"},

		// ── Branch pages (mock data) ──
		{name: "branches-list", handler: HandleBranches, path: "/dashboard/branches"},
		{name: "add-branch", handler: HandleAddBranch, path: "/dashboard/branches/add"},
		{name: "branch-detail", handler: HandleBranchDetail, path: "/dashboard/branches/1", vars: map[string]string{"id": "1"}},
		{name: "edit-branch", handler: HandleEditBranch, path: "/dashboard/branches/1/edit", vars: map[string]string{"id": "1"}},

		// ── User pages (mock data) ──
		{name: "users-list", handler: HandleUsers, path: "/dashboard/users"},
		{name: "add-user", handler: HandleAddUser, path: "/dashboard/users/add"},
		{name: "edit-user", handler: HandleEditUser, path: "/dashboard/users/1/edit", vars: map[string]string{"id": "1"}},

		// ── Settings ──
		{name: "settings", handler: HandleSettingsPage, path: "/dashboard/settings"},

		// ── Search pages ──
		{name: "parts-search", handler: HandlePartsSearch, path: "/dashboard/parts"},
		{name: "cars-search", handler: HandleCarsSearch, path: "/dashboard/cars"},

		// ── Credit bill detail ──
		{name: "credit-bill", handler: HandleCreditBill, path: "/credit_bill/1", vars: map[string]string{"id": "1"}},

		// ── Error pages ──
		{name: "not-found", handler: HandleNotFound, path: "/nonexistent", noAuth: true},
	}

	for _, tc := range pages {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			if !tc.noAuth {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: "render-test"})
			}
			if tc.vars != nil {
				req = mux.SetURLVars(req, tc.vars)
			}

			w := httptest.NewRecorder()
			tc.handler(w, req)

			code := w.Code
			// Accept 200, 302, 303 (redirects are valid); 404 for not-found handler
			if code != http.StatusOK && code != http.StatusFound && code != http.StatusSeeOther && code != http.StatusNotFound {
				t.Errorf("status %d, want 200/302/303\nBody: %.500s", code, w.Body.String())
				return
			}

			if code == http.StatusOK || code == http.StatusNotFound {
				body := w.Body.String()
				if len(body) < 100 {
					t.Errorf("body too short (%d bytes), likely empty render", len(body))
				}
				// Check for Go template errors in output
				if strings.Contains(body, "template:") && strings.Contains(body, "executing") {
					t.Errorf("template execution error in HTML output: %.500s", body)
				}
				// Check for Go fmt format-verb errors (e.g. %!s(int=N) from printf "%s" on int)
				if strings.Contains(body, "%!s(") || strings.Contains(body, "%!d(") || strings.Contains(body, "%!v(") {
					t.Errorf("Go format verb error in HTML output (wrong printf format): %.500s", body)
				}
			}
		})
	}
}
