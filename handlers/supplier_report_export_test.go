package handlers

import (
	"afrita/config"
	"afrita/helpers"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestExportSupplierReportExcelIncludesFullFilteredReport(t *testing.T) {
	helpers.APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		helpers.APICache.Flush()
	})

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/supplier/all":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":77,"name":"Supplier QA","vat_number":"300000000000003","phone_number":"0500000000","credit_limit":1000,"payment_terms_days":30}]`))
		case "/api/v2/supplier/77/report":
			if r.URL.Query().Get("from") != "2026-04-01" || r.URL.Query().Get("to") != "2026-04-30" {
				t.Fatalf("expected filtered report query, got %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"summary":{"bill_count":1,"total_spent":125,"total_before_vat":100,"total_vat":25,"total_payments":50,"closing_balance":75,"payment_count":1},
				"bills":[{"id":11,"sequence_number":501,"supplier_sequence_number":"SUP-501","total":125,"total_before_vat":100,"total_vat":25,"discount":0,"state":1,"effective_date":"2026-04-15T00:00:00Z","payment_due_date":"2026-04-30T00:00:00Z","item_count":2}],
				"payments":[{"id":9,"voucher_number":301,"voucher_type":"payment","effective_date":"2026-04-20T00:00:00Z","amount":50,"payment_method":"cash","description":"April payment"}],
				"top_items":[{"item_name":"Brake Pad","total_qty":2,"total_value":100,"avg_price":50,"bill_count":1}],
				"aging":[{"bucket":"current","bill_count":1,"bucket_total":75}],
				"monthly_spending":[{"month":"2026-04","total_spent":125}]
			}`))
		default:
			t.Fatalf("unexpected backend path %s", r.URL.Path)
		}
	}))
	t.Cleanup(backend.Close)
	config.BackendDomain = backend.URL

	config.SessionTokensMutex.Lock()
	config.SessionTokens["supplier-report-test"] = "token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, "supplier-report-test")
		config.SessionTokensMutex.Unlock()
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/suppliers/77/report/export-excel?from=2026-04-01&to=2026-04-30", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "supplier-report-test"})
	req = mux.SetURLVars(req, map[string]string{"id": "77"})
	w := httptest.NewRecorder()

	HandleExportSupplierReportExcel(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/vnd.ms-excel") {
		t.Fatalf("expected Excel content type, got %q", got)
	}
	body := w.Body.String()
	for _, want := range []string{
		"Supplier QA",
		"2026-04-01",
		"2026-04-30",
		"Account Ledger",
		"Bills",
		"Top Items",
		"Monthly Spending",
		"Payment Methods",
		"SUP-501",
		"Brake Pad",
		"April payment",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected Excel export to include %q in full report body:\n%s", want, body)
		}
	}
}
