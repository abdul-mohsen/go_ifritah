package handlers

import (
	"afrita/helpers"
	"afrita/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDashboardTestEndpoint verifies the test dashboard works
func TestDashboardTestEndpoint(t *testing.T) {
	req, err := http.NewRequest("GET", "/dashboard-test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleDashboardTest)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Dashboard test endpoint returned %v, expected 200", status)
	}

	body := rr.Body.String()

	checks := []string{
		"<!DOCTYPE html>",
		"لوحة التحكم",
		"revenueChart",
		"statusChart",
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("Dashboard HTML missing: %s", check)
		}
	}
}

func TestChartsAreAnimated(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	animationChecks := []string{
		"animation",
		"new Chart",
	}
	for _, check := range animationChecks {
		if !strings.Contains(body, check) {
			t.Errorf("Animation config missing: %s", check)
		}
	}
}

func TestDashboardDataStructure(t *testing.T) {
	testInvoice := models.Invoice{
		ID:             1,
		SequenceNumber: 1,
		Total:          15500.00,
		State:          3,
		CreditState:    0,
		EffectiveDate: struct {
			Time  string `json:"Time"`
			Valid bool   `json:"Valid"`
		}{
			Time:  "2026-02-13T00:00:00Z",
			Valid: true,
		},
	}

	status, statusClass := helpers.InvoiceStatus(testInvoice)
	if status != "صادرة" {
		t.Errorf("Expected 'صادرة', got '%s'", status)
	}
	if statusClass != "badge-issued" {
		t.Errorf("Expected 'badge-issued' status class, got '%s'", statusClass)
	}
}

func TestInvoiceTypeLabel(t *testing.T) {
	tests := []struct {
		invoice models.Invoice
		want    string
	}{
		{models.Invoice{State: 0, Type: true}, "فاتورة ضريبية"},
		{models.Invoice{State: 1, Type: true}, "فاتورة ضريبية"},
		{models.Invoice{CreditState: 1}, "إشعار دائن"},
	}

	for i, tt := range tests {
		got := helpers.InvoiceTypeLabel(tt.invoice)
		if got != tt.want {
			t.Errorf("test %d: InvoiceTypeLabel() = %v, want %v", i, got, tt.want)
		}
	}
}

func TestNavigationSidebar(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	navLinks := []string{
		"/dashboard",
		"/dashboard/invoices",
		"/dashboard/purchase-bills",
		"/dashboard/products",
		"/dashboard/parts",
		"/dashboard/cars",
		"/dashboard/suppliers",
		"/dashboard/clients",
		"/dashboard/orders",
		"/dashboard/users",
		"/dashboard/settings",
		"/logout",
	}

	for _, link := range navLinks {
		if !strings.Contains(body, link) {
			t.Errorf("Navigation link missing: %s", link)
		}
	}
}

func TestHTMXLoaded(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	body := rr.Body.String()

	if !strings.Contains(body, "htmx.org") {
		t.Error("HTMX library not loaded")
	}
}

func TestContentLength(t *testing.T) {
	req, _ := http.NewRequest("GET", "/dashboard-test", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)

	if len(rr.Body.String()) < 5000 {
		t.Errorf("Dashboard content too small: %d bytes", len(rr.Body.String()))
	}
}

func BenchmarkDashboardRender(b *testing.B) {
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/dashboard-test", nil)
		rr := httptest.NewRecorder()
		http.HandlerFunc(HandleDashboardTest).ServeHTTP(rr, req)
	}
}
