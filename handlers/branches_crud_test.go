package handlers

import (
	"afrita/config"
	"afrita/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// branchMockBackend returns a test server that handles the branch backend API endpoints.
func branchMockBackend(t *testing.T) *httptest.Server {
	t.Helper()
	branches := []models.Branch{
		{ID: 1, Name: "الفرع الرئيسي", Address: "شارع الملك فهد، الرياض", Phone: "0112345678", IsActive: true, ManagerName: "سالم الدوسري", Stores: []models.Store{{ID: 1, Name: "المخزن الرئيسي"}, {ID: 2, Name: "مخزن القطع"}}},
		{ID: 2, Name: "فرع جدة", Address: "شارع التحلية، جدة", Phone: "0122222222", IsActive: true},
		{ID: 3, Name: "فرع الدمام", Address: "شارع الأمير محمد، الدمام", Phone: "0133333333", IsActive: true},
		{ID: 4, Name: "فرع المدينة", Address: "طريق الملك عبدالعزيز", Phone: "0144444444", IsActive: false},
		{ID: 5, Name: "فرع مكة", Address: "حي العزيزية", Phone: "0155555555", IsActive: true},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v2/branch/all" && r.Method == "POST":
			resp := map[string]interface{}{"data": branches}
			json.NewEncoder(w).Encode(resp)
		case strings.HasPrefix(r.URL.Path, "/api/v2/branch/") && r.Method == "GET":
			idStr := strings.TrimPrefix(r.URL.Path, "/api/v2/branch/")
			for _, b := range branches {
				if idStr == strconv.Itoa(b.ID) {
					json.NewEncoder(w).Encode(map[string]interface{}{"detail": b})
					return
				}
			}
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"detail": "not found"})
		case r.URL.Path == "/api/v2/branch" && r.Method == "POST":
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]interface{}{"detail": map[string]interface{}{"id": 6, "name": "new"}})
		case strings.HasPrefix(r.URL.Path, "/api/v2/branch/") && r.Method == "PUT":
			idStr := strings.TrimPrefix(r.URL.Path, "/api/v2/branch/")
			found := false
			for _, b := range branches {
				if idStr == strconv.Itoa(b.ID) {
					found = true
					break
				}
			}
			if !found {
				w.WriteHeader(404)
				json.NewEncoder(w).Encode(map[string]string{"detail": "not found"})
				return
			}
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]string{"detail": "success"})
		case strings.HasPrefix(r.URL.Path, "/api/v2/branch/") && r.Method == "DELETE":
			idStr := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v2/branch/"), "/")
			if idStr == "1" {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"detail": "لا يمكن حذف فرع يحتوي على مخازن"})
				return
			}
			found := false
			for _, b := range branches {
				if idStr == strconv.Itoa(b.ID) {
					found = true
					break
				}
			}
			if !found {
				w.WriteHeader(404)
				json.NewEncoder(w).Encode(map[string]string{"detail": "not found"})
				return
			}
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]string{"detail": "success"})
		default:
			w.WriteHeader(404)
		}
	}))
}

// setupBranchBackend creates a mock backend and sets config.BackendDomain.
func setupBranchBackend(t *testing.T) {
	t.Helper()
	server := branchMockBackend(t)
	prev := config.BackendDomain
	config.BackendDomain = server.URL
	t.Cleanup(func() {
		server.Close()
		config.BackendDomain = prev
	})
}

// --- Handler integration tests ---

func TestHandleBranchesListPage(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	req, _ := http.NewRequest("GET", "/dashboard/branches", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleBranches).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "الفرع الرئيسي") {
		t.Error("List page should contain 'الفرع الرئيسي'")
	}
	if !strings.Contains(body, "فرع جدة") {
		t.Error("List page should contain 'فرع جدة'")
	}
	if !strings.Contains(body, "فرع الدمام") {
		t.Error("List page should contain 'فرع الدمام'")
	}
	if !strings.Contains(body, "نشط") {
		t.Error("List page should contain active badge")
	}
}

func TestHandleBranchesSearchFilter(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	req, _ := http.NewRequest("GET", "/dashboard/branches?q=جدة", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleBranches).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	// Should find "فرع جدة" (name match) and possibly others with جدة in address
	if !strings.Contains(body, "فرع جدة") {
		t.Error("Search for 'جدة' should find 'فرع جدة'")
	}
	// Should NOT contain branches without جدة in name or address
	if strings.Contains(body, "فرع الدمام") {
		t.Error("Search for 'جدة' should not show 'فرع الدمام'")
	}
}

func TestHandleAddBranchPage(t *testing.T) {
	seedTestSession()

	req, _ := http.NewRequest("GET", "/dashboard/branches/add", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleAddBranch).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "إضافة فرع") {
		t.Error("Add page should have 'إضافة فرع' title")
	}
}

func TestHandleBranchDetailPage(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	req, _ := http.NewRequest("GET", "/dashboard/branches/1", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleBranchDetail).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "الفرع الرئيسي") {
		t.Error("Detail page should show branch name")
	}
	if !strings.Contains(body, "شارع الملك فهد") {
		t.Error("Detail page should show branch address")
	}
}

func TestHandleBranchDetailNotFound(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	req, _ := http.NewRequest("GET", "/dashboard/branches/999", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})
	req = mux.SetURLVars(req, map[string]string{"id": "999"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleBranchDetail).ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Should return error for non-existent branch, not 200")
	}
}

func TestHandleEditBranchPage(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	req, _ := http.NewRequest("GET", "/dashboard/branches/2/edit", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})
	req = mux.SetURLVars(req, map[string]string{"id": "2"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleEditBranch).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "فرع جدة") {
		t.Error("Edit page should show branch name")
	}
	if !strings.Contains(body, "شارع التحلية") {
		t.Error("Edit page should show branch address")
	}
}

func TestHandleEditBranchNotFound(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	req, _ := http.NewRequest("GET", "/dashboard/branches/999/edit", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req.AddCookie(&http.Cookie{Name: "user_role", Value: "admin"})
	req = mux.SetURLVars(req, map[string]string{"id": "999"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleEditBranch).ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Should return error for non-existent branch")
	}
}

func TestHandleCreateBranchSuccess(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	form := url.Values{}
	form.Set("name", "فرع أبها")
	form.Set("location", "شارع الملك عبدالعزيز، أبها")
	form.Set("phone", "0512345678")

	req, _ := http.NewRequest("POST", "/dashboard/branches/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleCreateBranch).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 with HX-Redirect, got %d. Body: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("HX-Redirect") != "/dashboard/branches" {
		t.Errorf("Expected HX-Redirect to /dashboard/branches, got '%s'", rr.Header().Get("HX-Redirect"))
	}
}

func TestHandleCreateBranchValidationError(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	// Missing required fields
	form := url.Values{}
	form.Set("name", "")
	form.Set("location", "")
	form.Set("phone", "invalid")

	req, _ := http.NewRequest("POST", "/dashboard/branches/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleCreateBranch).ServeHTTP(rr, req)

	// Should re-render the form with errors (still 200, not redirect)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 (form re-render), got %d", rr.Code)
	}
	// Should NOT have HX-Redirect (stayed on form)
	if rr.Header().Get("HX-Redirect") != "" {
		t.Error("Should NOT redirect when validation fails")
	}
}

func TestHandleUpdateBranchSuccess(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	form := url.Values{}
	form.Set("name", "فرع جدة المحدث")
	form.Set("location", "حي الروضة، جدة")
	form.Set("phone", "0522222222")

	req, _ := http.NewRequest("POST", "/dashboard/branches/2/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "2"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleUpdateBranch).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("HX-Redirect") != "/dashboard/branches" {
		t.Errorf("Expected HX-Redirect to /dashboard/branches")
	}
}

func TestHandleUpdateBranchNotFound(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	form := url.Values{}
	form.Set("name", "فرع وهمي")
	form.Set("location", "مكان وهمي")
	form.Set("phone", "0500000000")

	req, _ := http.NewRequest("POST", "/dashboard/branches/999/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "999"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleUpdateBranch).ServeHTTP(rr, req)

	// Should NOT redirect for non-existent branch
	if rr.Code == http.StatusOK && rr.Header().Get("HX-Redirect") != "" {
		t.Error("Should NOT redirect on update of non-existent branch")
	}
}

func TestHandleDeleteBranchSuccess(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	req, _ := http.NewRequest("POST", "/dashboard/branches/4/delete", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "4"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDeleteBranch).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 with HX-Redirect, got %d", rr.Code)
	}
	if rr.Header().Get("HX-Redirect") != "/dashboard/branches" {
		t.Errorf("Expected HX-Redirect to /dashboard/branches")
	}
}

func TestHandleDeleteBranchWithStores(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	// Branch 1 has stores — mock backend returns 400
	req, _ := http.NewRequest("POST", "/dashboard/branches/1/delete", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDeleteBranch).ServeHTTP(rr, req)

	// Should return error, not success redirect
	if rr.Header().Get("HX-Redirect") == "/dashboard/branches" {
		t.Error("Should NOT redirect when branch has stores")
	}
}

func TestHandleDeleteBranchNotFound(t *testing.T) {
	seedTestSession()
	setupBranchBackend(t)

	req, _ := http.NewRequest("POST", "/dashboard/branches/999/delete", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	req = mux.SetURLVars(req, map[string]string{"id": "999"})

	rr := httptest.NewRecorder()
	http.HandlerFunc(HandleDeleteBranch).ServeHTTP(rr, req)

	if rr.Header().Get("HX-Redirect") == "/dashboard/branches" {
		t.Error("Should NOT redirect for non-existent branch")
	}
}
