package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// TestEditUserPageUsesBaseLayout verifies edit-user renders inside base layout
func TestEditUserPageUsesBaseLayout(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/users/2/edit", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	// HandleEditUser uses mux.Vars for {id}
	req = mux.SetURLVars(req, map[string]string{"id": "2"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleEditUser).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("HandleEditUser returned status %d, expected 200", status)
	}

	body := rr.Body.String()

	checks := []string{
		"sidebar",            // base layout sidebar
		"تعديل المستخدم",     // page title
		"المعلومات الأساسية", // basic info section header
		"مصفوفة الصلاحيات",   // permission matrix header
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("Edit user page missing base layout element: %q", check)
		}
	}
}

// TestEditUserPageHasUserData verifies mock user data renders correctly
func TestEditUserPageHasUserData(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/users/3/edit", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})
	req = mux.SetURLVars(req, map[string]string{"id": "3"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleEditUser).ServeHTTP(rr, req)

	body := rr.Body.String()

	// Should contain the user's data
	if !strings.Contains(body, "user_3") {
		t.Errorf("Edit user page should contain username 'user_3'")
	}
	if !strings.Contains(body, "user3@afrita.com") {
		t.Errorf("Edit user page should contain email 'user3@afrita.com'")
	}
}

// TestEditUserPageHasPermissionMatrix verifies the permission matrix renders
func TestEditUserPageHasPermissionMatrix(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/users/2/edit", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})
	req = mux.SetURLVars(req, map[string]string{"id": "2"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleEditUser).ServeHTTP(rr, req)

	body := rr.Body.String()

	// Permission matrix resources (Arabic labels)
	permChecks := []string{
		"الفواتير",   // Invoices
		"المنتجات",   // Products
		"العملاء",    // Clients
		"الموردين",   // Suppliers
		"المستودعات", // Stores
	}

	for _, check := range permChecks {
		if !strings.Contains(body, check) {
			t.Errorf("Edit user page missing permission resource: %q", check)
		}
	}
}
