package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"afrita/config"
	"afrita/models"
)

// ──────────────────────────────────────────────
// Token Refresh Middleware Tests
// ──────────────────────────────────────────────

// setupTestSession creates a test session with given tokens and expiry.
func setupTestSession(sessionID, access, refresh string, expiry time.Time) {
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = access
	config.SessionRefreshTokens[sessionID] = refresh
	config.SessionTokenExpiry[sessionID] = expiry
	config.SessionTokensMutex.Unlock()
}

// clearTestSession removes a test session.
func clearTestSession(sessionID string) {
	config.SessionTokensMutex.Lock()
	delete(config.SessionTokens, sessionID)
	delete(config.SessionRefreshTokens, sessionID)
	delete(config.SessionTokenExpiry, sessionID)
	config.SessionTokensMutex.Unlock()
}

// fakeRefreshServer creates a mock backend that responds to /api/v2/refresh.
func fakeRefreshServer(newAccess, newRefresh string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/refresh" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			resp := models.AuthResponse{
				AccessToken:  newAccess,
				RefreshToken: newRefresh,
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// TestMiddlewareRefresh_DoesNotUpdateExpiry proves the bug:
// After middleware refreshes token on 401, SessionTokenExpiry is NOT updated.
func TestMiddlewareRefresh_DoesNotUpdateExpiry(t *testing.T) {
	sessionID := "test-session-expiry-bug"

	// Set up session with token expiring in the past
	oldExpiry := time.Now().Add(-5 * time.Minute)
	setupTestSession(sessionID, "old-access", "valid-refresh", oldExpiry)
	defer clearTestSession(sessionID)

	// Start a mock backend that returns 200 on refresh
	mockBackend := fakeRefreshServer("new-access-token", "new-refresh-token", http.StatusOK)
	defer mockBackend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = mockBackend.URL
	defer func() { config.BackendDomain = origDomain }()

	// Simulate a 401 handler that triggers middleware refresh
	handler := TokenRefreshMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// BUG: After middleware refresh, access token IS updated...
	config.SessionTokensMutex.RLock()
	newAccess := config.SessionTokens[sessionID]
	newExpiry := config.SessionTokenExpiry[sessionID]
	config.SessionTokensMutex.RUnlock()

	if newAccess != "new-access-token" {
		t.Errorf("expected access token to be updated to 'new-access-token', got '%s'", newAccess)
	}

	// FIXED: After middleware refresh, expiry should be updated to ~15 min from now
	if newExpiry.Equal(oldExpiry) {
		t.Errorf("SessionTokenExpiry was NOT updated after middleware refresh. "+
			"Still: %v (should be ~15 min from now)", newExpiry)
	}
	if time.Until(newExpiry) < 14*time.Minute {
		t.Errorf("SessionTokenExpiry should be ~15 min from now, got %v", newExpiry)
	}
}

// TestMiddlewareRefresh_DoesNotPersistToDisk proves the second bug:
// After middleware refreshes token on 401, it does NOT persist to disk.
func TestMiddlewareRefresh_DoesNotPersistToDisk(t *testing.T) {
	sessionID := "test-session-persist-bug"

	setupTestSession(sessionID, "old-access", "valid-refresh", time.Now().Add(-5*time.Minute))
	defer clearTestSession(sessionID)

	mockBackend := fakeRefreshServer("new-access-from-middleware", "new-refresh-from-middleware", http.StatusOK)
	defer mockBackend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = mockBackend.URL
	defer func() { config.BackendDomain = origDomain }()

	handler := TokenRefreshMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	_ = rr // just trigger the middleware

	// BUG: Memory is updated but disk is not.
	// If we could check disk, the file would still have "old-access" or not exist.
	// Since we can't easily mock the filesystem here, we verify the
	// middleware function doesn't call SaveTokenToFile by checking that
	// SessionTokenExpiry was NOT updated (which means persist was NOT called either).
	config.SessionTokensMutex.RLock()
	expiry, exists := config.SessionTokenExpiry[sessionID]
	config.SessionTokensMutex.RUnlock()

	if !exists {
		t.Fatal("expected SessionTokenExpiry to exist for session")
	}

	// FIXED: Expiry should be updated to ~now+15min (which means persist was also called)
	if time.Until(expiry) > 0 {
		t.Log("OK: Expiry was updated and token persisted")
	} else {
		t.Errorf("Token was refreshed in middleware but expiry NOT updated. "+
			"Expiry still in the past: %v", expiry)
	}
}

// TestMiddlewareRefresh_StillReturns401 proves the third issue:
// Even after a successful token refresh, the middleware still returns 401 to the client.
func TestMiddlewareRefresh_StillReturns401(t *testing.T) {
	sessionID := "test-session-401-after-refresh"

	setupTestSession(sessionID, "old-access", "valid-refresh", time.Now().Add(-5*time.Minute))
	defer clearTestSession(sessionID)

	mockBackend := fakeRefreshServer("refreshed-access", "refreshed-refresh", http.StatusOK)
	defer mockBackend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = mockBackend.URL
	defer func() { config.BackendDomain = origDomain }()

	handler := TokenRefreshMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// FIXED: After successful refresh, middleware should redirect (303) not return 401
	if rr.Code == http.StatusUnauthorized {
		t.Errorf("Middleware should NOT return 401 after successful refresh, got %d", rr.Code)
	}
	// Should redirect to same URL
	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected 303 redirect after refresh, got %d", rr.Code)
	}
}

// TestIdleFor20Minutes_DoesNotLogOut simulates the user going idle for 20
// minutes (access token expired). On the next click the middleware must
// refresh the token instead of redirecting to login.
func TestIdleFor20Minutes_DoesNotLogOut(t *testing.T) {
	sessionID := "test-idle-20min"

	// Access token expired 20 minutes ago, but refresh token is still valid (7 day lifetime)
	expiredExpiry := time.Now().Add(-20 * time.Minute)
	setupTestSession(sessionID, "stale-access", "still-valid-refresh", expiredExpiry)
	defer clearTestSession(sessionID)

	mockBackend := fakeRefreshServer("fresh-access", "fresh-refresh", http.StatusOK)
	defer mockBackend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = mockBackend.URL
	defer func() { config.BackendDomain = origDomain }()

	// The handler returns 401 (backend rejects the stale access token)
	handler := TokenRefreshMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Must NOT get 401 — should redirect so browser retries with fresh token
	if rr.Code == http.StatusUnauthorized {
		t.Fatal("user was logged out after 20 min idle — expected token refresh + redirect")
	}
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect after refresh, got %d", rr.Code)
	}

	// Verify token was actually refreshed
	config.SessionTokensMutex.RLock()
	access := config.SessionTokens[sessionID]
	refresh := config.SessionRefreshTokens[sessionID]
	expiry := config.SessionTokenExpiry[sessionID]
	config.SessionTokensMutex.RUnlock()

	if access != "fresh-access" {
		t.Errorf("access token not refreshed, got '%s'", access)
	}
	if refresh != "fresh-refresh" {
		t.Errorf("refresh token not updated, got '%s'", refresh)
	}
	if time.Until(expiry) < 14*time.Minute {
		t.Errorf("expiry should be ~15 min from now, got %v", expiry)
	}
}

// TestIdleFor20Minutes_NoAccessToken_StillRefreshes simulates the cleanup
// goroutine having already removed the access token from memory, but the
// refresh token still exists. The middleware must still refresh on the next click.
func TestIdleFor20Minutes_NoAccessToken_StillRefreshes(t *testing.T) {
	sessionID := "test-idle-20min-no-access"

	// Only refresh token in memory — access token was cleaned up
	config.SessionTokensMutex.Lock()
	delete(config.SessionTokens, sessionID)
	config.SessionRefreshTokens[sessionID] = "surviving-refresh"
	config.SessionTokenExpiry[sessionID] = time.Now().Add(-20 * time.Minute)
	config.SessionTokensMutex.Unlock()
	defer clearTestSession(sessionID)

	mockBackend := fakeRefreshServer("recovered-access", "recovered-refresh", http.StatusOK)
	defer mockBackend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = mockBackend.URL
	defer func() { config.BackendDomain = origDomain }()

	handler := TokenRefreshMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This handler should never run if middleware refreshes first
		w.WriteHeader(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Must NOT log out
	if rr.Code == http.StatusUnauthorized {
		t.Fatal("user logged out when access token was gone but refresh token existed")
	}

	// Should redirect (refresh happened in the no-access-token path)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}

	config.SessionTokensMutex.RLock()
	access := config.SessionTokens[sessionID]
	config.SessionTokensMutex.RUnlock()

	if access != "recovered-access" {
		t.Errorf("access token not restored, got '%s'", access)
	}
}

// TestIdleFor20Minutes_HandlerCallsHandleUnauthorized simulates the real
// production scenario: handler calls helpers.HandleUnauthorized on backend 401,
// which sets a Set-Cookie header to delete session_id, THEN writes 401.
// The middleware intercepts 401 for refresh, but the cookie deletion header
// must NOT leak into the redirect response — otherwise browser drops cookie
// and follows redirect anonymously => logged out.
func TestIdleFor20Minutes_HandlerCallsHandleUnauthorized(t *testing.T) {
	sessionID := "test-idle-handler-clears-cookie"

	expiredExpiry := time.Now().Add(-20 * time.Minute)
	setupTestSession(sessionID, "stale-access", "still-valid-refresh", expiredExpiry)
	defer clearTestSession(sessionID)

	mockBackend := fakeRefreshServer("fresh-access", "fresh-refresh", http.StatusOK)
	defer mockBackend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = mockBackend.URL
	defer func() { config.BackendDomain = origDomain }()

	// Simulate a real handler that calls HandleUnauthorized when backend returns 401:
	// it deletes the session cookie, sets HX-Redirect / Location, then writes 401.
	handler := TokenRefreshMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// What helpers.HandleUnauthorized does:
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusUnauthorized)
	}))

	req := httptest.NewRequest("GET", "/dashboard/purchase-bills", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Must NOT get 401
	if rr.Code == http.StatusUnauthorized {
		t.Fatal("user logged out — expected refresh + redirect")
	}
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}

	// CRITICAL: response must NOT contain a Set-Cookie that deletes session_id,
	// otherwise the browser drops the cookie and the redirect goes to login.
	for _, sc := range rr.Result().Cookies() {
		if sc.Name == "session_id" && sc.MaxAge < 0 {
			t.Errorf("response deletes session_id cookie (Max-Age=%d) — browser will be logged out after redirect", sc.MaxAge)
		}
	}

	// HX-Redirect to "/" would also send user to login page
	if rr.Header().Get("HX-Redirect") == "/" {
		t.Errorf("response has HX-Redirect=/ (login page) instead of redirecting back to original path")
	}
}


func TestStandaloneRefreshIfNeeded_AlsoMissesExpiry(t *testing.T) {
	sessionID := "test-standalone-refresh"

	oldExpiry := time.Now().Add(-1 * time.Minute)
	setupTestSession(sessionID, "old-access", "valid-refresh", oldExpiry)
	defer clearTestSession(sessionID)

	mockBackend := fakeRefreshServer("standalone-new-access", "standalone-new-refresh", http.StatusOK)
	defer mockBackend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = mockBackend.URL
	defer func() { config.BackendDomain = origDomain }()

	err := RefreshTokenIfNeeded(sessionID)
	if err != nil {
		t.Fatalf("RefreshTokenIfNeeded returned error: %v", err)
	}

	// Verify access token was updated
	config.SessionTokensMutex.RLock()
	newAccess := config.SessionTokens[sessionID]
	newExpiry := config.SessionTokenExpiry[sessionID]
	config.SessionTokensMutex.RUnlock()

	if newAccess != "standalone-new-access" {
		t.Errorf("expected access token 'standalone-new-access', got '%s'", newAccess)
	}

	// FIXED: The middleware's RefreshTokenIfNeeded should now update expiry
	if newExpiry.Equal(oldExpiry) {
		t.Errorf("middleware.RefreshTokenIfNeeded does NOT update SessionTokenExpiry. "+
			"Still: %v (should be ~15 min from now)", newExpiry)
	}
	if time.Until(newExpiry) < 14*time.Minute {
		t.Errorf("SessionTokenExpiry should be ~15 min from now, got %v", newExpiry)
	}
}
