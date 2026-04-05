package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"afrita/helpers"
	"afrita/models"

	"github.com/gorilla/mux"
)

// HandleUsers displays the users management page
func HandleUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	// TODO (backend): Replace mock with real API call to /api/v2/users
	now := time.Now()
	lastLogin := now.Add(-2 * time.Hour)
	lastLogin2 := now.Add(-5 * time.Hour)

	mockUsers := []models.User{
		{ID: 1, Username: "admin", Email: "admin@afrita.com", Role: models.RoleAdmin, Active: true, CreatedAt: now.Add(-30 * 24 * time.Hour), LastLogin: &lastLogin},
		{ID: 2, Username: "manager1", Email: "manager@afrita.com", Role: models.RoleManager, Active: true, CreatedAt: now.Add(-20 * 24 * time.Hour), LastLogin: &lastLogin},
		{ID: 3, Username: "employee1", Email: "emp1@afrita.com", Role: models.RoleEmployee, Active: true, CreatedAt: now.Add(-10 * 24 * time.Hour), LastLogin: &lastLogin2},
		{ID: 4, Username: "employee2", Email: "emp2@afrita.com", Role: models.RoleEmployee, Active: false, CreatedAt: now.Add(-5 * 24 * time.Hour)},
		{ID: 5, Username: "manager2", Email: "manager2@afrita.com", Role: models.RoleManager, Active: true, CreatedAt: now.Add(-3 * 24 * time.Hour), LastLogin: &lastLogin2, ManagerID: func() *int { v := 1; return &v }()},
	}

	// Apply search filter
	query := r.URL.Query().Get("q")
	roleFilter := r.URL.Query().Get("role")
	filtered := make([]models.User, 0, len(mockUsers))
	for _, u := range mockUsers {
		if query != "" && !helpers.ContainsInsensitive(u.Username, query) && !helpers.ContainsInsensitive(u.Email, query) {
			continue
		}
		if roleFilter != "" && string(u.Role) != roleFilter {
			continue
		}
		filtered = append(filtered, u)
	}

	// Paginate
	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	pagedUsers, pagination := helpers.PaginateSlice(filtered, page, perPage)
	prevPage := 0
	nextPage := 0
	if pagination.Page > 1 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages {
		nextPage = pagination.Page + 1
	}

	helpers.Render(w, r, "users", map[string]interface{}{
		"title":       "إدارة المستخدمين",
		"users":       pagedUsers,
		"query":       query,
		"role_filter": roleFilter,
		"pagination":  pagination,
		"prev_page":   prevPage,
		"next_page":   nextPage,
	})
}

// HandleAddUser displays the add user form
func HandleAddUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	// TODO (backend): Fetch real managers list from /api/v2/users?role=manager
	now := time.Now()
	mockManagers := []models.User{
		{ID: 1, Username: "admin", Email: "admin@afrita.com", Role: models.RoleAdmin, CreatedAt: now},
		{ID: 2, Username: "manager1", Email: "manager@afrita.com", Role: models.RoleManager, CreatedAt: now},
	}

	helpers.Render(w, r, "add-user", map[string]interface{}{
		"title":    "إضافة مستخدم",
		"Managers": mockManagers,
	})
}

// HandleEditUser displays the edit user form
func HandleEditUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	now := time.Now()
	mockUser := models.User{
		ID:        helpers.ParseIntValue(id),
		Username:  "user_" + id,
		Email:     "user" + id + "@afrita.com",
		Role:      models.RoleEmployee,
		Active:    true,
		CreatedAt: now.Add(-30 * 24 * time.Hour),
	}

	helpers.Render(w, r, "edit-user", map[string]interface{}{
		"title": "تعديل المستخدم",
		"User":  mockUser,
	})
}

// HandleCreateUser creates a new user
func HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	// Accept JSON body
	var payload map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	// TODO: Forward to backend when user creation API is available
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "تم إنشاء المستخدم بنجاح",
	})
}

// HandleUpdateUser updates an existing user
func HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	var payload map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "تم تحديث المستخدم بنجاح",
	})
}

// HandleDeleteUser deletes a user
func HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	helpers.WriteSuccessRedirect(w, "/dashboard/users", "تم حذف المستخدم بنجاح")
}

// HandleUpdateUserPermissions updates user permissions
func HandleUpdateUserPermissions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	var payload map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "تم تحديث الصلاحيات بنجاح",
	})
}
