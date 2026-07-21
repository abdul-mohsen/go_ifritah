package helpers

import (
	"afrita/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchMultiSupplierReportParsesEachEntry(t *testing.T) {
	origDomain := config.BackendDomain
	t.Cleanup(func() { config.BackendDomain = origDomain })

	var capturedQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/supplier/report/multi" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"suppliers":[
			{
				"supplier": {"id":77,"name":"Supplier A"},
				"summary":{"bill_count":2,"total_spent":300,"total_payments":50,"closing_balance":250},
				"bills":[],"payments":[],"top_items":[],"aging":[],"monthly_spending":[]
			},
			{
				"supplier": {"id":88,"name":"Supplier B"},
				"summary":{"bill_count":1,"total_spent":100,"total_payments":0,"closing_balance":100},
				"bills":[],"payments":[],"top_items":[],"aging":[],"monthly_spending":[]
			}
		]}`))
	}))
	defer backend.Close()
	config.BackendDomain = backend.URL

	reports, err := FetchMultiSupplierReport("token", []int{77, 88}, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedQuery, "ids=77%2C88") && !strings.Contains(capturedQuery, "ids=77,88") {
		t.Fatalf("expected ids=77,88 in query, got %q", capturedQuery)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d: %+v", len(reports), reports)
	}
	if reports[77].Summary.TotalSpent != 300 {
		t.Errorf("supplier 77 total_spent = %v, want 300", reports[77].Summary.TotalSpent)
	}
	if reports[88].Summary.TotalSpent != 100 {
		t.Errorf("supplier 88 total_spent = %v, want 100", reports[88].Summary.TotalSpent)
	}
}

func TestFetchMultiSupplierReportOmitsUnresolvedSuppliers(t *testing.T) {
	origDomain := config.BackendDomain
	t.Cleanup(func() { config.BackendDomain = origDomain })

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Backend only resolved one of the two requested ids.
		_, _ = w.Write([]byte(`{"suppliers":[
			{
				"supplier": {"id":77,"name":"Supplier A"},
				"summary":{"total_spent":50},
				"bills":[],"payments":[],"top_items":[],"aging":[],"monthly_spending":[]
			}
		]}`))
	}))
	defer backend.Close()
	config.BackendDomain = backend.URL

	reports, err := FetchMultiSupplierReport("token", []int{77, 999}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected exactly 1 resolved report, got %d: %+v", len(reports), reports)
	}
	if _, ok := reports[999]; ok {
		t.Fatalf("unresolved supplier 999 should be absent from the map")
	}
}

func TestFetchMultiSupplierReportFallsBackWhenEndpointMissing(t *testing.T) {
	origDomain := config.BackendDomain
	t.Cleanup(func() { config.BackendDomain = origDomain })

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/supplier/report/multi":
			w.WriteHeader(http.StatusNotFound)
		case "/api/v2/supplier/77/report":
			_, _ = w.Write([]byte(`{"summary":{"total_spent":42},"bills":[],"payments":[],"top_items":[],"aging":[],"monthly_spending":[]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer backend.Close()
	config.BackendDomain = backend.URL
	APICache.Flush()

	reports, err := FetchMultiSupplierReport("token", []int{77}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 || reports[77].Summary.TotalSpent != 42 {
		t.Fatalf("expected fallback per-supplier fetch to still resolve supplier 77, got %+v", reports)
	}
}
