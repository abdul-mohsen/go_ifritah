package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"afrita/config"
	"afrita/models"
)

// createTestSession sets up a session with a JWT token containing the given role.
func createTestSession(role models.Role, userID int, username string) (string, func()) {
	claims := map[string]interface{}{
		"user_id":  float64(userID),
		"username": username,
		"email":    username + "@test.com",
		"role":     string(role),
		"exp":      float64(time.Now().Add(time.Hour).Unix()),
	}
	payload, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	fakeJWT := "eyJhbGciOiJIUzI1NiJ9." + encoded + ".fakesig"

	sessionID := "test-session-" + username
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = fakeJWT
	config.SessionTokensMutex.Unlock()

	cleanup := func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	}
	return sessionID, cleanup
}

func TestRequirePermission_AdminBypass(t *testing.T) {
	config.Initialize()
	sessionID, cleanup := createTestSession(models.RoleAdmin, 1, "admin")
	defer cleanup()

	handler := RequirePermission("invoices", "delete")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("DELETE", "/api/invoices/1", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Admin should bypass all permission checks, got %d", rr.Code)
	}
}

func TestRequirePermission_ManagerCannotDeleteUsers(t *testing.T) {
	config.Initialize()
	sessionID, cleanup := createTestSession(models.RoleManager, 2, "manager")
	defer cleanup()

	handler := RequirePermission("users", "delete")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/dashboard/users/1/delete", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Manager should not delete users, got %d", rr.Code)
	}
}

func TestRequirePermission_ManagerCannotAccessSettings(t *testing.T) {
	config.Initialize()
	sessionID, cleanup := createTestSession(models.RoleManager, 2, "manager")
	defer cleanup()

	handler := RequirePermission("settings", "view")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard/settings", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Manager should not access settings, got %d", rr.Code)
	}
}

func TestRequirePermission_ManagerCanViewInvoices(t *testing.T) {
	config.Initialize()
	sessionID, cleanup := createTestSession(models.RoleManager, 2, "manager")
	defer cleanup()

	handler := RequirePermission("invoices", "view")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/dashboard/invoices", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Manager should view invoices, got %d", rr.Code)
	}
}

func TestRequirePermission_NoSession_RedirectsToLogin(t *testing.T) {
	config.Initialize()

	handler := RequirePermission("invoices", "view")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard/invoices", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("No session should redirect to login, got %d", rr.Code)
	}
}

func TestRequirePermission_NoSession_APIReturns401(t *testing.T) {
	config.Initialize()

	handler := RequirePermission("invoices", "view")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/invoices", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("API without session should return 401, got %d", rr.Code)
	}
}

func TestRequireRole_AdminCanAccess(t *testing.T) {
	config.Initialize()
	sessionID, cleanup := createTestSession(models.RoleAdmin, 1, "admin2")
	defer cleanup()

	handler := RequireRole(models.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/dashboard/settings", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Admin should access admin-only routes, got %d", rr.Code)
	}
}

func TestRequireRole_EmployeeBlocked(t *testing.T) {
	config.Initialize()
	sessionID, cleanup := createTestSession(models.RoleEmployee, 3, "employee1")
	defer cleanup()

	handler := RequireRole(models.RoleAdmin, models.RoleManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard/users", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Employee should not access manager+ routes, got %d", rr.Code)
	}
}

func TestRequireRole_ManagerCanAccessManagerUp(t *testing.T) {
	config.Initialize()
	sessionID, cleanup := createTestSession(models.RoleManager, 2, "manager2")
	defer cleanup()

	handler := RequireRole(models.RoleAdmin, models.RoleManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/dashboard/users", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Manager should access manager+ routes, got %d", rr.Code)
	}
}

func TestCheckPermission_CorrectlyChecks(t *testing.T) {
	perms := []models.Permission{
		{Resource: "invoices", CanView: true, CanAdd: true, CanEdit: false, CanDelete: false},
		{Resource: "products", CanView: true, CanAdd: false, CanEdit: true, CanDelete: false},
	}

	tests := []struct {
		resource string
		action   string
		expected bool
	}{
		{"invoices", "view", true},
		{"invoices", "add", true},
		{"invoices", "edit", false},
		{"invoices", "delete", false},
		{"products", "view", true},
		{"products", "add", false},
		{"products", "edit", true},
		{"products", "delete", false},
		{"clients", "view", false}, // resource not in permissions
	}

	for _, tt := range tests {
		result := checkPermission(perms, tt.resource, tt.action)
		if result != tt.expected {
			t.Errorf("checkPermission(%s, %s) = %v, want %v", tt.resource, tt.action, result, tt.expected)
		}
	}
}

func TestGetAllResources_ReturnsExpected(t *testing.T) {
	resources := GetAllResources()
	if len(resources) == 0 {
		t.Fatal("GetAllResources should return non-empty list")
	}

	expected := map[string]bool{
		"invoices": true, "products": true, "clients": true,
		"suppliers": true, "stores": true, "branches": true,
		"orders": true, "users": true, "settings": true,
	}

	for key := range expected {
		found := false
		for _, r := range resources {
			if r == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected resource %q not found in GetAllResources()", key)
		}
	}
}
