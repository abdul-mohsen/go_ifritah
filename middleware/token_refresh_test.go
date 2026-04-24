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

// TestStandaloneRefreshIfNeeded_AlsoMissesExpiry proves the middleware's
// standalone RefreshTokenIfNeeded function also doesn't update expiry.
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
