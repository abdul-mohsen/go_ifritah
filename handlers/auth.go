package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

// HandleLogin renders the login page
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	helpers.RenderStandalone(w, "login", map[string]interface{}{"title": "Login"})
}

// HandleLoginPost processes login form submission
func HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		helpers.WriteErrorToast(w, "بيانات غير صالحة")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		helpers.WriteErrorToast(w, "يرجى إدخال اسم المستخدم وكلمة المرور")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Call the real backend API with JSON payload
	loginData := models.AuthRequest{Username: username, Password: password}
	jsonPayload, _ := json.Marshal(loginData)
	log.Printf("🔑 Attempting login to: %s/api/v2/login with username: %s", config.BackendDomain, username)

	resp, err := http.Post(
		config.BackendDomain+"/api/v2/login",
		"application/json",
		bytes.NewBuffer(jsonPayload),
	)
	if err != nil {
		log.Printf("❌ Backend connection error: %v", err)
		helpers.WriteErrorToast(w, "خطأ في الاتصال بالخادم")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var backendResp models.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&backendResp); err != nil {
		log.Printf("Failed to decode backend response: %v", err)
	}
	log.Printf("Backend response status: %d, error: %s", resp.StatusCode, backendResp.Error)

	if resp.StatusCode != http.StatusOK || backendResp.AccessToken == "" {
		errMsg := backendResp.Error
		if errMsg == "" {
			errMsg = "اسم المستخدم أو كلمة المرور غير صحيحة"
		}
		helpers.WriteErrorToast(w, errMsg)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Success - store token in session and redirect
	sessionID := generateSecureSessionID()
	expiryTime := time.Now().Add(15 * time.Minute)

	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = backendResp.AccessToken
	config.SessionRefreshTokens[sessionID] = backendResp.RefreshToken
	config.SessionTokenExpiry[sessionID] = expiryTime
	config.SessionUserRoles[sessionID] = helpers.DecodeJWTRole(backendResp.AccessToken)
	config.SessionTokensMutex.Unlock()

	// Persist tokens to disk for app restart recovery
	token := &models.Token{
		AccessToken:  backendResp.AccessToken,
		RefreshToken: backendResp.RefreshToken,
		ExpiresAt:    expiryTime,
		CreatedAt:    time.Now(),
	}

	if err := helpers.SaveTokenToFile(sessionID, token); err != nil {
		log.Printf("⚠️  Failed to persist token: %v", err)
	}

	// Set secure HttpOnly cookie; Secure=true when not localhost
	isSecure := !isLocalhost()
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 60 * 60, // 7 days — matches refresh token lifetime
	})

	log.Printf("✅ Secure token storage: HttpOnly=true, Secure=%v, SameSite=Strict for user: %s", isSecure, username)

	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(backendResp)
}

// HandleLogout clears session and removes persisted tokens
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Get session ID from cookie
	cookie, err := r.Cookie("session_id")
	if err == nil {
		sessionID := cookie.Value

		// Delete persisted token file
		if err := helpers.DeleteTokenFile(sessionID); err != nil {
			log.Printf("⚠️  Failed to delete token file during logout: %v", err)
		}

		// Clear from memory
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		delete(config.SessionRefreshTokens, sessionID)
		delete(config.SessionTokenExpiry, sessionID)
		config.SessionTokensMutex.Unlock()

		log.Printf("✅ Session cleared for session: %s", sessionID)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleRefreshToken handles token refresh requests from client
func HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get session ID from cookie
	cookie, err := r.Cookie("session_id")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "No session"})
		return
	}

	sessionID := cookie.Value

	// Attempt token refresh
	if !helpers.RefreshTokenIfNeeded(w, r, sessionID) {
		// Refresh failed, clear session
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		delete(config.SessionRefreshTokens, sessionID)
		delete(config.SessionTokenExpiry, sessionID)
		config.SessionTokensMutex.Unlock()
		_ = helpers.DeleteTokenFile(sessionID)

		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Token refresh failed"})
		return
	}

	// Get updated token
	config.SessionTokensMutex.RLock()
	newToken := config.SessionTokens[sessionID]
	config.SessionTokensMutex.RUnlock()

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"access_token": newToken})
}

// HandleRegister renders the register page
func HandleRegister(w http.ResponseWriter, r *http.Request) {
	helpers.RenderStandalone(w, "register", nil)
}

// HandleRegisterPost forwards registration to backend POST /api/v2/register.
func HandleRegisterPost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		FullName string `json:"full_name"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "طلب غير صالح"})
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "اسم المستخدم والبريد وكلمة المرور مطلوبة"})
		return
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(config.BackendDomain+"/api/v2/register", "application/json", bytes.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "تعذر الاتصال بالخادم"})
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// HandleForgotPassword renders the forgot password page
func HandleForgotPassword(w http.ResponseWriter, r *http.Request) {
	helpers.RenderStandalone(w, "forgot-password", nil)
}

// HandleForgotPasswordPost forwards to backend POST /api/v2/forgot-password.
func HandleForgotPasswordPost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "طلب غير صالح"})
		return
	}
	if req.Email == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "البريد الإلكتروني مطلوب"})
		return
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(config.BackendDomain+"/api/v2/forgot-password", "application/json", bytes.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "تعذر الاتصال بالخادم"})
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// generateSecureSessionID creates a cryptographically random session identifier.
func generateSecureSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback (should never happen)
		return fmt.Sprintf("session_%d", time.Now().UnixNano())
	}
	return "s_" + hex.EncodeToString(b)
}

// isLocalhost returns true when running on localhost (no HTTPS available).
func isLocalhost() bool {
	domain := strings.ToLower(os.Getenv("APP_DOMAIN"))
	return domain == "" ||
		domain == "localhost" ||
		strings.HasPrefix(domain, "127.0.0.1") ||
		domain == "localtest.me" ||
		strings.HasSuffix(domain, ".localtest.me")
}
