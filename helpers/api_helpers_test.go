package helpers

import (
	"afrita/config"
	"afrita/models"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDecodeListResponseArray(t *testing.T) {
	body := []byte(`[{"id":1},{"id":2}]`)

	invoices, err := decodeListResponse[models.Invoice](body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(invoices) != 2 {
		t.Fatalf("expected 2 invoices, got %d", len(invoices))
	}
}

func TestDecodeListResponseWrapped(t *testing.T) {
	body := []byte(`{"data":[{"id":1},{"id":2}]}`)

	invoices, err := decodeListResponse[models.Invoice](body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(invoices) != 2 {
		t.Fatalf("expected 2 invoices, got %d", len(invoices))
	}
}

func TestDecodeListResponseOrdersWrapped(t *testing.T) {
	body := []byte(`{"orders":[{"client":"Acme"},{"client":"Beta"}]}`)

	orders, err := decodeListResponse[map[string]interface{}](body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}
}

func TestFetchSupplierReportCachesLegacyFallbackOnServerError(t *testing.T) {
	APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		APICache.Flush()
	})

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		switch r.URL.Path {
		case "/api/v2/supplier/77/report":
			http.Error(w, "report failed", http.StatusInternalServerError)
		case "/api/v2/purchase_bill/all":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case "/api/v2/cash_voucher/all":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected legacy fallback path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	config.BackendDomain = server.URL

	for i := 0; i < 2; i++ {
		_, err := FetchSupplierReport("token", 77, "2026-04-01", "2026-04-30")
		if err != nil {
			t.Fatalf("FetchSupplierReport call %d failed: %v", i+1, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected one cached legacy fallback sequence, got %d backend requests", got)
	}
}

func TestFetchSupplierReportCachesSuccessfulFilteredReport(t *testing.T) {
	APICache.Flush()
	oldDomain := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = oldDomain
		APICache.Flush()
	})

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Path != "/api/v2/supplier/77/report" {
			t.Fatalf("unexpected backend path %s", r.URL.Path)
		}
		if r.URL.Query().Get("from") != "2026-04-01" || r.URL.Query().Get("to") != "2026-04-30" {
			t.Fatalf("report filter was not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary":{"bill_count":1,"total_spent":125,"closing_balance":125},
			"bills":[{"id":11,"sequence_number":501,"supplier_sequence_number":"SUP-501","total":125,"state":1,"effective_date":"2026-04-15T00:00:00Z"}],
			"payments":[],"top_items":[],"aging":[],"monthly_spending":[]
		}`))
	}))
	t.Cleanup(server.Close)
	config.BackendDomain = server.URL

	for i := 0; i < 2; i++ {
		report, err := FetchSupplierReport("token", 77, "2026-04-01", "2026-04-30")
		if err != nil {
			t.Fatalf("FetchSupplierReport call %d failed: %v", i+1, err)
		}
		if len(report.Bills) != 1 || report.Bills[0].SequenceNumber != 501 {
			t.Fatalf("unexpected report bills: %+v", report.Bills)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected cached second report request, got %d backend calls", got)
	}
}
