package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

// TokenRefreshMiddleware automatically refreshes expired tokens on 401 responses
func TokenRefreshMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip middleware for login page and static assets
		if r.URL.Path == "/" || r.URL.Path == "/login" || strings.HasPrefix(r.URL.Path, "/static") {
			next.ServeHTTP(w, r)
			return
		}

		// Get session ID from cookie
		cookie, err := r.Cookie("session_id")
		if err != nil {
			// No session cookie, let the handler deal with it
			next.ServeHTTP(w, r)
			return
		}

		sessionID := cookie.Value

		// Check if we have tokens for this session
		config.SessionTokensMutex.RLock()
		accessToken, hasAccess := config.SessionTokens[sessionID]
		refreshToken, hasRefresh := config.SessionRefreshTokens[sessionID]
		config.SessionTokensMutex.RUnlock()

		if !hasAccess {
			// No access token, let the handler redirect to login
			next.ServeHTTP(w, r)
			return
		}

		// Create a response wrapper to capture 401 errors
		wrapper := &responseWrapper{
			ResponseWriter: w,
			sessionID:      sessionID,
			accessToken:    accessToken,
			refreshToken:   refreshToken,
			hasRefresh:     hasRefresh,
			originalPath:   r.URL.RequestURI(),
		}

		next.ServeHTTP(wrapper, r)

		// Token was refreshed — redirect so the browser retries with the new token
		if wrapper.refreshedOK {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", wrapper.originalPath)
				w.WriteHeader(http.StatusOK)
			} else {
				http.Redirect(w, r, wrapper.originalPath, http.StatusSeeOther)
			}
			return
		}

		// If we detected a 401 and refresh failed, redirect to login
		if wrapper.needsLoginRedirect {
			// Clear session cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "session_id",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
			})

			// Check if it's an HTMX request
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Regular redirect
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	})
}

// responseWrapper wraps http.ResponseWriter to detect and handle 401 responses
type responseWrapper struct {
	http.ResponseWriter
	sessionID          string
	accessToken        string
	refreshToken       string
	hasRefresh         bool
	statusCode         int
	needsLoginRedirect bool
	refreshedOK        bool
	headerWritten      bool
	originalPath       string
}

func (rw *responseWrapper) WriteHeader(statusCode int) {
	rw.statusCode = statusCode

	// If we get a 401 Unauthorized, try to refresh the token
	if statusCode == http.StatusUnauthorized && rw.hasRefresh && rw.refreshToken != "" {
		log.Printf("🔄 Detected 401 Unauthorized, attempting token refresh for session: %s", rw.sessionID)

		if rw.attemptTokenRefresh() {
			log.Printf("✅ Token refreshed successfully for session: %s — marking for redirect", rw.sessionID)
			// Token refreshed: signal outer handler to redirect so the
			// request is retried with the fresh token.
			rw.refreshedOK = true
			return
		}

		log.Printf("❌ Token refresh failed for session: %s, redirecting to login", rw.sessionID)
		rw.needsLoginRedirect = true
		return
	}

	rw.ResponseWriter.WriteHeader(statusCode)
	rw.headerWritten = true
}

func (rw *responseWrapper) Write(b []byte) (int, error) {
	if rw.needsLoginRedirect || rw.refreshedOK {
		// Silently discard writes — middleware will handle redirect
		return len(b), nil
	}
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// attemptTokenRefresh tries to refresh the access token using the refresh token
func (rw *responseWrapper) attemptTokenRefresh() bool {
	// Request new access token using refresh token
	refreshReq, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/refresh", nil)
	if err != nil {
		log.Printf("Failed to create refresh request: %v", err)
		return false
	}

	refreshReq.Header.Set("Authorization", "Bearer "+rw.refreshToken)

	client := &http.Client{}
	refreshResp, err := client.Do(refreshReq)
	if err != nil {
		log.Printf("Refresh request failed: %v", err)
		return false
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(refreshResp.Body)
		log.Printf("Refresh returned non-OK status %d: %s", refreshResp.StatusCode, string(bodyBytes))
		return false
	}

	var authResp models.AuthResponse
	if err := json.NewDecoder(refreshResp.Body).Decode(&authResp); err != nil {
		log.Printf("Failed to decode refresh response: %v", err)
		return false
	}

	if authResp.AccessToken == "" {
		log.Printf("Refresh response missing access token")
		return false
	}

	// Update session with new tokens AND expiry
	newExpiry := time.Now().Add(15 * time.Minute)
	config.SessionTokensMutex.Lock()
	config.SessionTokens[rw.sessionID] = authResp.AccessToken
	if authResp.RefreshToken != "" {
		config.SessionRefreshTokens[rw.sessionID] = authResp.RefreshToken
	}
	config.SessionTokenExpiry[rw.sessionID] = newExpiry
	config.SessionTokensMutex.Unlock()

	// Persist to disk so token survives app restart
	refTok := rw.refreshToken
	if authResp.RefreshToken != "" {
		refTok = authResp.RefreshToken
	}
	token := &models.Token{
		AccessToken:  authResp.AccessToken,
		RefreshToken: refTok,
		ExpiresAt:    newExpiry,
		CreatedAt:    time.Now(),
	}
	if err := helpers.SaveTokenToFile(rw.sessionID, token); err != nil {
		log.Printf("⚠️  Failed to persist refreshed token: %v", err)
	}

	log.Printf("🔑 Token refreshed, expiry updated, and persisted for session: %s", rw.sessionID)
	return true
}

// RefreshTokenIfNeeded checks if token needs refresh and attempts it
// This can be used by handlers that need to ensure fresh tokens before API calls
func RefreshTokenIfNeeded(sessionID string) error {
	config.SessionTokensMutex.RLock()
	refreshToken, hasRefresh := config.SessionRefreshTokens[sessionID]
	config.SessionTokensMutex.RUnlock()

	if !hasRefresh || refreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	// Request new access token
	refreshReq, err := http.NewRequest("POST", config.BackendDomain+"/api/v2/refresh", nil)
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %v", err)
	}

	refreshReq.Header.Set("Authorization", "Bearer "+refreshToken)

	client := &http.Client{}
	refreshResp, err := client.Do(refreshReq)
	if err != nil {
		return fmt.Errorf("refresh request failed: %v", err)
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(refreshResp.Body)
		return fmt.Errorf("refresh failed with status %d: %s", refreshResp.StatusCode, string(bodyBytes))
	}

	var authResp models.AuthResponse
	bodyBytes, _ := io.ReadAll(refreshResp.Body)
	if err := json.Unmarshal(bodyBytes, &authResp); err != nil {
		return fmt.Errorf("failed to decode refresh response: %v", err)
	}

	if authResp.AccessToken == "" {
		return fmt.Errorf("refresh response missing access token")
	}

	// Update session with new tokens AND expiry
	newExpiry := time.Now().Add(15 * time.Minute)
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = authResp.AccessToken
	if authResp.RefreshToken != "" {
		config.SessionRefreshTokens[sessionID] = authResp.RefreshToken
	}
	config.SessionTokenExpiry[sessionID] = newExpiry
	config.SessionTokensMutex.Unlock()

	// Persist to disk
	refTok := refreshToken
	if authResp.RefreshToken != "" {
		refTok = authResp.RefreshToken
	}
	token := &models.Token{
		AccessToken:  authResp.AccessToken,
		RefreshToken: refTok,
		ExpiresAt:    newExpiry,
		CreatedAt:    time.Now(),
	}
	if err := helpers.SaveTokenToFile(sessionID, token); err != nil {
		log.Printf("⚠️  Failed to persist refreshed token: %v", err)
	}

	log.Printf("✅ Token proactively refreshed for session: %s", sessionID)
	return nil
}
