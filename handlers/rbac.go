package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
	"afrita/resources"

	"github.com/gorilla/mux"
)

// RequirePermission enforces permission on a resource.
func RequirePermission(resource string, action string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := getUserFromSession(r)
			if user == nil {
				if isAPIRequest(r) {
					http.Error(w, `{"error":"Unauthorized - Please login"}`, http.StatusUnauthorized)
				} else {
					http.Redirect(w, r, "/", http.StatusFound)
				}
				return
			}

			if user.Role == models.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			if user.Role == models.RoleManager {
				if resource == "users" && action == "delete" {
					respondWithForbidden(w, r, resources.T(helpers.GetLang(r), "rbac.manager_delete_forbidden"))
					return
				}
				if resource == "settings" {
					respondWithForbidden(w, r, resources.T(helpers.GetLang(r), "rbac.manager_settings_forbidden"))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if !checkPermission(user.Permissions, resource, action) {
				respondWithForbidden(w, r, resources.T(helpers.GetLang(r), "rbac.action_forbidden"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireBillImportPermission selects the appropriate add permission from the
// requested document type. A shared page must not grant access to the other
// document type.
func RequireBillImportPermission(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromSession(r)
		if user == nil {
			if isAPIRequest(r) {
				http.Error(w, `{"error":"Unauthorized - Please login"}`, http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/", http.StatusFound)
			}
			return
		}
		resource := "invoices"
		if strings.EqualFold(r.URL.Query().Get("type"), "purchase") {
			resource = "purchase_bills"
		}
		if user.Role != models.RoleAdmin && user.Role != models.RoleManager && !checkPermission(user.Permissions, resource, "add") {
			respondWithForbidden(w, r, "ليس لديك صلاحية للقيام بهذا الإجراء")
			return
		}
		next.ServeHTTP(w, r)
	}
}

// RequireRole enforces specific roles.
func RequireRole(roles ...models.Role) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := getUserFromSession(r)
			if user == nil {
				if isAPIRequest(r) {
					http.Error(w, `{"error":"Unauthorized - Please login"}`, http.StatusUnauthorized)
				} else {
					http.Redirect(w, r, "/", http.StatusFound)
				}
				return
			}

			for _, role := range roles {
				if user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			respondWithForbidden(w, r, resources.T(helpers.GetLang(r), "rbac.permissions_insufficient"))
		})
	}
}

func checkPermission(permissions []models.Permission, resource string, action string) bool {
	for _, perm := range permissions {
		if perm.Resource == resource {
			switch action {
			case "view":
				return perm.CanView
			case "add":
				return perm.CanAdd
			case "edit":
				return perm.CanEdit
			case "delete":
				return perm.CanDelete
			}
		}
	}
	return false
}

func getUserFromSession(r *http.Request) *models.User {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil
	}

	sessionID := cookie.Value

	config.SessionTokensMutex.RLock()
	token, exists := config.SessionTokens[sessionID]
	config.SessionTokensMutex.RUnlock()

	if !exists || token == "" {
		return nil
	}

	// Try to extract user info from JWT payload
	user := parseUserFromJWT(token)
	if user != nil {
		return user
	}

	// Fallback: token exists but we can't parse JWT — assume admin
	now := time.Now()
	return &models.User{
		ID:        1,
		Username:  "admin",
		Email:     "admin@afrita.com",
		Role:      models.RoleAdmin,
		Active:    true,
		CreatedAt: time.Now().AddDate(0, -1, 0),
		LastLogin: &now,
	}
}

// parseUserFromJWT extracts user info from a JWT access token without
// verifying signature (the backend already validated it).
func parseUserFromJWT(token string) *models.User {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return nil
	}

	// Decode the payload (second part)
	payload := parts[1]
	// JWT uses base64url without padding
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil
	}

	user := &models.User{
		Active:    true,
		CreatedAt: time.Now().AddDate(0, -1, 0),
	}

	if id, ok := claims["user_id"].(float64); ok {
		user.ID = int(id)
	} else if id, ok := claims["sub"].(float64); ok {
		user.ID = int(id)
	}

	if username, ok := claims["username"].(string); ok {
		user.Username = username
	} else if username, ok := claims["sub"].(string); ok {
		user.Username = username
	}

	if email, ok := claims["email"].(string); ok {
		user.Email = email
	}

	// Extract role — default to admin if not present (backward compat)
	role := models.RoleAdmin
	if r, ok := claims["role"].(string); ok {
		switch models.Role(r) {
		case models.RoleAdmin, models.RoleManager, models.RoleEmployee:
			role = models.Role(r)
		}
	}

	// Override: treat "ssda" as admin (backend JWT returns "employee" but this is the owner account)
	if user.Username == "ssda" {
		role = models.RoleAdmin
	}

	user.Role = role

	now := time.Now()
	user.LastLogin = &now
	return user
}

func isAPIRequest(r *http.Request) bool {
	if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
		return true
	}
	accept := r.Header.Get("Accept")
	return accept == "application/json" || r.Header.Get("Content-Type") == "application/json"
}

func respondWithForbidden(w http.ResponseWriter, r *http.Request, message string) {
	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": message,
		})
		return
	}

	lang := helpers.GetLang(r)
	dir := "rtl"
	if lang == resources.LangEn {
		dir = "ltr"
	}

	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(fmt.Sprintf(`
<!DOCTYPE html>
<html lang="%s" dir="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>403 - %s</title>
    <style>
        body {
            font-family: 'Cairo', sans-serif;
            background: linear-gradient(135deg, #1d3666 0%%, #2a4a8f 100%%);
            min-height: 100vh;
            display: flex;
            justify-content: center;
            align-items: center;
            margin: 0;
            padding: 20px;
        }
        .error-container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            padding: 60px 40px;
            text-align: center;
            max-width: 500px;
        }
        .error-code {
            font-size: 72px;
            font-weight: bold;
            color: #1d3666;
            margin: 0;
        }
        .error-title {
            font-size: 28px;
            color: #1d3666;
            margin: 20px 0;
        }
        .error-message {
            font-size: 16px;
            color: #666;
            margin: 20px 0;
            line-height: 1.6;
        }
        .back-button {
            display: inline-block;
            background: #FDBC2D;
            color: #1d3666;
            padding: 12px 30px;
            border-radius: 6px;
            text-decoration: none;
            font-weight: bold;
            margin-top: 20px;
            transition: all 0.3s ease;
        }
        .back-button:hover {
            background: #e5a826;
            transform: translateY(-2px);
        }
    </style>
</head>
<body>
    <div class="error-container">
        <h1 class="error-code">403</h1>
        <h2 class="error-title">%s</h2>
        <p class="error-message">%s</p>
        <a class="back-button" href="/dashboard">%s</a>
    </div>
</body>
</html>
`, lang, dir, resources.T(lang, "rbac.forbidden_title"), resources.T(lang, "rbac.forbidden_title"), html.EscapeString(message), resources.T(lang, "middleware.back_to_dashboard"))))
}

// GetAllResources returns the list of valid permission resources.
func GetAllResources() []string {
	return []string{
		"invoices",
		"purchase-bills",
		"purchase_bills",
		"products",
		"clients",
		"suppliers",
		"stores",
		"branches",
		"orders",
		"users",
		"settings",
		"bills",
		"credit-bills",
	}
}
