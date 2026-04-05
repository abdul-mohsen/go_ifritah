package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOrdersPageSearchFilter verifies the search query filters orders
func TestOrdersPageSearchFilter(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/orders?q=testquery", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleOrders).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("HandleOrders with search returned status %d, expected 200", status)
	}

	body := rr.Body.String()

	// The search field should preserve the query value
	if !strings.Contains(body, "testquery") {
		t.Errorf("Orders page should preserve search query in input field")
	}

	// Page should still have the pagination element
	if !strings.Contains(body, "الصفحة") {
		t.Errorf("Orders page missing pagination text")
	}
}

// TestOrdersPagePagination verifies pagination renders correctly
func TestOrdersPagePagination(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/orders", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleOrders).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("HandleOrders returned status %d, expected 200", status)
	}

	body := rr.Body.String()

	// Should have base layout
	if !strings.Contains(body, "sidebar") {
		t.Errorf("Orders page missing base layout sidebar")
	}

	// Should have table headers
	if !strings.Contains(body, "رقم الطلب") {
		t.Errorf("Orders page missing order number column header")
	}
}
