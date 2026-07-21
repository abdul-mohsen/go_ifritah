package handlers

import (
	"afrita/config"
	"afrita/helpers"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// twoSupplierStatementBackend returns a mock backend serving two suppliers
// (77 and 88) plus their combined report via the single
// /api/v2/supplier/report/multi request, for exercising the multi-supplier
// ledger statement feature (mirrors the real backend's response shape:
// {"suppliers": [ {"supplier": {...}, "summary": {...}, ...}, ... ]}).
func twoSupplierStatementBackend(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/supplier/all":
			_, _ = w.Write([]byte(`[
				{"id":77,"name":"Supplier A","vat_number":"300000000000003"},
				{"id":88,"name":"Supplier B","vat_number":"300000000000004"}
			]`))
		case "/api/v2/supplier/report/multi":
			_, _ = w.Write([]byte(`{"suppliers":[
				{
					"supplier": {"id":77,"name":"Supplier A"},
					"summary":{"bill_count":1,"total_spent":125,"total_before_vat":100,"total_vat":25,"total_payments":50,"closing_balance":75,"payment_count":1},
					"bills":[{"id":11,"sequence_number":501,"supplier_sequence_number":"SUP-501","total":125,"total_before_vat":100,"total_vat":25,"discount":0,"state":1,"effective_date":"2026-04-15T00:00:00Z","item_count":2}],
					"payments":[],
					"top_items":[],
					"aging":[],
					"monthly_spending":[]
				},
				{
					"supplier": {"id":88,"name":"Supplier B"},
					"summary":{"bill_count":1,"total_spent":200,"total_before_vat":170,"total_vat":30,"total_payments":0,"closing_balance":200,"payment_count":0},
					"bills":[{"id":22,"sequence_number":502,"supplier_sequence_number":"SUP-502","total":200,"total_before_vat":170,"total_vat":30,"discount":0,"state":0,"effective_date":"2026-04-18T00:00:00Z","item_count":1}],
					"payments":[],
					"top_items":[],
					"aging":[],
					"monthly_spending":[]
				}
			]}`))
		default:
			t.Fatalf("unexpected backend path %s", r.URL.Path)
		}
	}))
}

func TestSupplierStatementCombinesMultipleSuppliers(t *testing.T) {
	helpers.APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		helpers.APICache.Flush()
	})

	backend := twoSupplierStatementBackend(t)
	t.Cleanup(backend.Close)
	config.BackendDomain = backend.URL

	cleanup := setupPBTestSession("supplier-statement-test", "supplier-statement-token")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/suppliers/statement?ids=77,88&from=2026-04-01&to=2026-04-30", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "supplier-statement-test"})
	w := httptest.NewRecorder()

	HandleSupplierStatement(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"Supplier A", "Supplier B", "SUP-501", "SUP-502"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected combined statement page to include %q, got:\n%.2000s", want, body)
		}
	}
	// Combined total_spent must be the sum across both suppliers (125 + 200 = 325).
	if !strings.Contains(body, "325.00") {
		t.Fatalf("expected combined total spent 325.00 in page, got:\n%.2000s", body)
	}
}

func TestSupplierStatementRejectsEmptySelection(t *testing.T) {
	cleanup := setupPBTestSession("supplier-statement-empty", "supplier-statement-empty-token")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/suppliers/statement", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "supplier-statement-empty"})
	w := httptest.NewRecorder()

	HandleSupplierStatement(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no suppliers are selected, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportSupplierStatementCSVIncludesBothSuppliers(t *testing.T) {
	helpers.APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		helpers.APICache.Flush()
	})

	backend := twoSupplierStatementBackend(t)
	t.Cleanup(backend.Close)
	config.BackendDomain = backend.URL

	cleanup := setupPBTestSession("supplier-statement-csv", "supplier-statement-csv-token")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/suppliers/statement/export-csv?ids=77,88&from=2026-04-01&to=2026-04-30", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "supplier-statement-csv"})
	w := httptest.NewRecorder()

	HandleExportSupplierStatementCSV(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/csv") {
		t.Fatalf("expected CSV content type, got %q", got)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Supplier A") || !strings.Contains(body, "Supplier B") {
		t.Fatalf("expected CSV to include a Supplier column identifying both suppliers, got:\n%s", body)
	}
}

func TestExportSupplierStatementExcelIncludesBothSuppliers(t *testing.T) {
	helpers.APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		helpers.APICache.Flush()
	})

	backend := twoSupplierStatementBackend(t)
	t.Cleanup(backend.Close)
	config.BackendDomain = backend.URL

	cleanup := setupPBTestSession("supplier-statement-excel", "supplier-statement-excel-token")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/suppliers/statement/export-excel?ids=77,88&from=2026-04-01&to=2026-04-30", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "supplier-statement-excel"})
	w := httptest.NewRecorder()

	HandleExportSupplierStatementExcel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/vnd.ms-excel") {
		t.Fatalf("expected Excel content type, got %q", got)
	}
	body := w.Body.String()
	for _, want := range []string{"Supplier A", "Supplier B", "SUP-501", "SUP-502"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected combined Excel export to include %q, got:\n%s", want, body)
		}
	}
	// Exactly one HTML document (shared shell), not two concatenated documents.
	if strings.Count(body, "<html") != 1 {
		t.Fatalf("expected exactly one <html> shell wrapping both supplier sections, got %d in:\n%s", strings.Count(body, "<html"), body)
	}
}

func TestSuppliersListPageHasStatementCheckboxes(t *testing.T) {
	helpers.APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		helpers.APICache.Flush()
	})

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":77,"name":"Supplier A"}]`))
	}))
	t.Cleanup(backend.Close)
	config.BackendDomain = backend.URL

	cleanup := setupPBTestSession("suppliers-list-checkbox", "suppliers-list-checkbox-token")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/suppliers", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "suppliers-list-checkbox"})
	w := httptest.NewRecorder()

	HandleSuppliers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `class="supplier-statement-check"`) {
		t.Fatalf("expected suppliers list rows to have a statement-selection checkbox")
	}
	if !strings.Contains(body, `generateSupplierStatement()`) {
		t.Fatalf("expected suppliers list page to expose the generate-statement action")
	}
	if !strings.Contains(body, `id="generateStatementBtn"`) {
		t.Fatalf("expected suppliers list page to have the generate-statement button")
	}
}
