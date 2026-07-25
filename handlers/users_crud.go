package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"

	"github.com/gorilla/mux"
)

var userUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.]{3,30}$`)

func fetchUsers(token string) ([]models.User, error) {
	req, err := http.NewRequest(http.MethodPost, config.BackendDomain+"/api/v2/user/all", bytes.NewBufferString(`{}`))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("backend status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return decodeUsers(body)
}

func decodeUsers(body []byte) ([]models.User, error) {
	var users []models.User
	if err := json.Unmarshal(body, &users); err == nil {
		return users, nil
	}

	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Users json.RawMessage `json:"users"`
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	for _, raw := range []json.RawMessage{envelope.Data, envelope.Users, envelope.Items} {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		if err := json.Unmarshal(raw, &users); err == nil {
			return users, nil
		}
	}
	return nil, fmt.Errorf("invalid user list response")
}

func findUser(users []models.User, id string) (models.User, bool) {
	idValue := helpers.ParseIntValue(id)
	for _, user := range users {
		if user.ID == idValue {
			return user, true
		}
	}
	return models.User{}, false
}

func proxyUserRequest(method, path, token string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, config.BackendDomain+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return helpers.DoAuthedRequest(req, token)
}

func userValidation(r *http.Request, includePassword bool) helpers.FieldErrors {
	rules := []helpers.FieldRule{
		{Field: "username", Value: r.FormValue("username"), Required: true, MinLen: 3, MaxLen: 30, Pattern: userUsernamePattern, Label: "اسم المستخدم", PatternMsg: "اسم المستخدم غير صالح"},
		{Field: "role", Value: r.FormValue("role"), Required: true, Label: "الدور"},
	}
	if role := r.FormValue("role"); role != string(models.RoleAdmin) && role != string(models.RoleManager) && role != string(models.RoleEmployee) {
		rules[1].Pattern = regexp.MustCompile(`^(admin|manager|employee)$`)
		rules[1].PatternMsg = "الدور غير صالح"
	}
	if includePassword {
		rules = append(rules,
			helpers.FieldRule{Field: "password", Value: r.FormValue("password"), Required: true, MinLen: 8, MaxLen: 128, Label: "كلمة المرور"},
			helpers.FieldRule{Field: "confirm_password", Value: r.FormValue("confirm_password"), Required: true, MinLen: 8, MaxLen: 128, Label: "تأكيد كلمة المرور"},
		)
	}
	errs := helpers.Validate(rules)
	if includePassword && errs == nil && r.FormValue("password") != r.FormValue("confirm_password") {
		errs = helpers.FieldErrors{"confirm_password": "كلمات المرور غير متطابقة"}
	}
	return errs
}

func renderUserFormError(w http.ResponseWriter, r *http.Request, templateName, title string, errs helpers.FieldErrors, user models.User) {
	data := helpers.RenderFormWithErrors(map[string]interface{}{
		"title": title,
		"User":  user,
	}, errs, helpers.OldValues([]string{"username", "role", "password", "confirm_password", "active", "new_password"}, r.FormValue))
	helpers.Render(w, r, templateName, data)
}

func formActive(r *http.Request) bool {
	value := strings.ToLower(strings.TrimSpace(r.FormValue("active")))
	return value != "" && value != "false" && value != "0" && value != "off"
}

// HandleUsers displays the users list page.
func HandleUsers(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	users, err := fetchUsers(token)
	backendErr := ""
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		users = []models.User{}
		backendErr = "تعذر تحميل المستخدمين من الخادم حالياً"
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	roleFilter := r.URL.Query().Get("role")
	filtered := make([]models.User, 0, len(users))
	for _, user := range users {
		if query != "" && !helpers.MatchSearchQuery(query, user.Username, user.Email) {
			continue
		}
		if roleFilter != "" && string(user.Role) != roleFilter {
			continue
		}
		filtered = append(filtered, user)
	}
	page := helpers.ParseIntValue(r.URL.Query().Get("page"))
	perPage := helpers.ParseIntValue(r.URL.Query().Get("per"))
	paged, pagination := helpers.PaginateSlice(filtered, page, perPage)
	prevPage, nextPage := -1, -1
	if pagination.Page > 0 {
		prevPage = pagination.Page - 1
	}
	if pagination.Page < pagination.TotalPages-1 {
		nextPage = pagination.Page + 1
	}
	helpers.Render(w, r, "users", map[string]interface{}{
		"title": "إدارة المستخدمين", "users": paged, "query": query, "role_filter": roleFilter,
		"pagination": pagination, "prev_page": prevPage, "next_page": nextPage, "error": backendErr,
	})
}

// HandleAddUser displays the create-user form.
func HandleAddUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}
	helpers.Render(w, r, "add-user", map[string]interface{}{"title": "إضافة مستخدم جديد"})
}

// HandleEditUser displays the edit-user form.
func HandleEditUser(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	users, err := fetchUsers(token)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "تعذر تحميل المستخدم")
		return
	}
	user, found := findUser(users, id)
	if !found {
		helpers.WriteErrorResponse(w, http.StatusNotFound, nil, "المستخدم غير موجود")
		return
	}
	helpers.Render(w, r, "edit-user", map[string]interface{}{"title": "تعديل المستخدم", "User": user})
}

// HandleCreateUser creates a user through the backend API.
func HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	if errs := userValidation(r, true); errs != nil {
		renderUserFormError(w, r, "add-user", "إضافة مستخدم جديد", errs, models.User{Role: models.RoleEmployee, Active: true})
		return
	}
	payload := map[string]interface{}{
		"username":  r.FormValue("username"),
		"role":      r.FormValue("role"),
		"is_active": formActive(r),
		"password":  r.FormValue("password"),
	}
	resp, err := proxyUserRequest(http.MethodPost, "/api/v2/user", token, payload)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "تعذر إنشاء المستخدم")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "تعذر إنشاء المستخدم")
		return
	}
	helpers.WriteSuccessRedirect(w, "/dashboard/users", "تم إنشاء المستخدم بنجاح")
}

// HandleUpdateUser updates user details or, for the separate reset form,
// posts a new password to the administrator reset endpoint.
func HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	if newPassword := r.FormValue("new_password"); newPassword != "" {
		errs := helpers.Validate([]helpers.FieldRule{{Field: "new_password", Value: newPassword, Required: true, MinLen: 8, MaxLen: 128, Label: "كلمة المرور الجديدة"}})
		if errs != nil {
			renderUserFormError(w, r, "edit-user", "تعديل المستخدم", errs, models.User{ID: helpers.ParseIntValue(id), Username: r.FormValue("username"), Role: models.Role(r.FormValue("role")), Active: formActive(r)})
			return
		}
		resp, err := proxyUserRequest(http.MethodPost, "/api/v2/user/"+id+"/password", token, map[string]string{"new_password": newPassword})
		if err != nil {
			helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "تعذر إعادة تعيين كلمة المرور")
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			helpers.WriteErrorResponse(w, resp.StatusCode, resp, "تعذر إعادة تعيين كلمة المرور")
			return
		}
		helpers.WriteSuccessRedirect(w, "/dashboard/users/"+id+"/edit", "تم إعادة تعيين كلمة المرور بنجاح")
		return
	}

	if errs := userValidation(r, false); errs != nil {
		renderUserFormError(w, r, "edit-user", "تعديل المستخدم", errs, models.User{ID: helpers.ParseIntValue(id), Username: r.FormValue("username"), Role: models.Role(r.FormValue("role")), Active: formActive(r)})
		return
	}
	payload := map[string]interface{}{"username": r.FormValue("username"), "role": r.FormValue("role"), "is_active": formActive(r)}
	resp, err := proxyUserRequest(http.MethodPut, "/api/v2/user/"+id, token, payload)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "تعذر تحديث المستخدم")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "تعذر تحديث المستخدم")
		return
	}
	helpers.WriteSuccessRedirect(w, "/dashboard/users", "تم تحديث المستخدم بنجاح")
}

// HandleDeleteUser deletes a user through the backend API.
func HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	resp, err := proxyUserRequest(http.MethodDelete, "/api/v2/user/"+id, token, nil)
	if err != nil {
		helpers.WriteErrorResponse(w, http.StatusBadGateway, nil, "تعذر حذف المستخدم")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		helpers.WriteErrorResponse(w, resp.StatusCode, resp, "تعذر حذف المستخدم")
		return
	}
	helpers.WriteSuccessRedirect(w, "/dashboard/users", "تم حذف المستخدم بنجاح")
}
