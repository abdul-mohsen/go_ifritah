package handlers

import (
	"afrita/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// seedTestSession stores a fake token in config.SessionTokens so that
// GetTokenOrRedirect doesn't redirect during tests.
func seedTestSession() {
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()
}

// TestUsersPageUsesBaseLayout verifies the users page renders inside the base layout
// (sidebar, toast container, dark mode support) instead of standalone HTML.
func TestUsersPageUsesBaseLayout(t *testing.T) {
	seedTestSession()

	req, err := http.NewRequest("GET", "/dashboard/users", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleUsers)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("HandleUsers returned status %d, expected 200", status)
	}

	body := rr.Body.String()

	// The base layout should contain the sidebar and common structure
	baseLayoutChecks := []string{
		"sidebar",          // sidebar element from base.html
		"إدارة المستخدمين", // page title in Arabic
		"إضافة مستخدم",     // add user button
	}

	for _, check := range baseLayoutChecks {
		if !strings.Contains(body, check) {
			t.Errorf("Users page missing base layout element: %q", check)
		}
	}
}

// TestUsersPageHasMockData verifies the users page displays mock user data
func TestUsersPageHasMockData(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/users", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleUsers).ServeHTTP(rr, req)

	body := rr.Body.String()

	// Should show Arabic role badges
	roleChecks := []string{
		"مدير", // admin badge
		"مشرف", // manager badge
		"موظف", // employee badge
		"نشط",  // active status
	}

	for _, check := range roleChecks {
		if !strings.Contains(body, check) {
			t.Errorf("Users page missing role/status element: %q", check)
		}
	}
}

// TestUsersPageHasPagination verifies pagination displays on the users page
func TestUsersPageHasPagination(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/users", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleUsers).ServeHTTP(rr, req)

	body := rr.Body.String()

	// Should have pagination element
	if !strings.Contains(body, "الصفحة") {
		t.Errorf("Users page missing pagination text 'الصفحة'")
	}
}

// TestUsersPageSearchFilter verifies search filtering works
func TestUsersPageSearchFilter(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/users?q=admin", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleUsers).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("HandleUsers with search returned status %d, expected 200", status)
	}

	body := rr.Body.String()
	// Should contain admin user
	if !strings.Contains(body, "admin@afrita.com") {
		t.Errorf("Search for 'admin' should show admin user email")
	}
}

// TestUsersPageRoleFilter verifies role filter works
func TestUsersPageRoleFilter(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/users?role=manager", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleUsers).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("HandleUsers with role filter returned status %d, expected 200", status)
	}

	body := rr.Body.String()
	// When filtering by manager, should contain manager email
	if !strings.Contains(body, "manager@afrita.com") {
		t.Errorf("Role filter 'manager' should show manager user")
	}
}
