package helpers

import (
	"afrita/config"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func cloneStringMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneTimeMap(source map[string]time.Time) map[string]time.Time {
	cloned := make(map[string]time.Time, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func TestGetTokenOrRedirectRefreshesExpiredAccessToken(t *testing.T) {
	oldBackendDomain := config.BackendDomain
	oldTokenStoreDir := config.TokenStoreDir
	config.SessionTokensMutex.Lock()
	oldSessionTokens := cloneStringMap(config.SessionTokens)
	oldRefreshTokens := cloneStringMap(config.SessionRefreshTokens)
	oldTokenExpiry := cloneTimeMap(config.SessionTokenExpiry)
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.BackendDomain = oldBackendDomain
		config.TokenStoreDir = oldTokenStoreDir
		config.SessionTokensMutex.Lock()
		config.SessionTokens = oldSessionTokens
		config.SessionRefreshTokens = oldRefreshTokens
		config.SessionTokenExpiry = oldTokenExpiry
		config.SessionTokensMutex.Unlock()
	})

	refreshSeen := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/refresh" {
			t.Fatalf("unexpected refresh path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer refresh-token" {
			t.Fatalf("unexpected refresh authorization header %q", got)
		}
		refreshSeen = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fresh-token","refresh_token":"fresh-refresh"}`))
	}))
	t.Cleanup(backend.Close)

	config.BackendDomain = backend.URL
	config.TokenStoreDir = t.TempDir()
	sessionID := "session-refresh-test"
	config.SessionTokensMutex.Lock()
	config.SessionTokens = map[string]string{sessionID: "expired-token"}
	config.SessionRefreshTokens = map[string]string{sessionID: "refresh-token"}
	config.SessionTokenExpiry = map[string]time.Time{sessionID: time.Now().Add(-time.Minute)}
	config.SessionTokensMutex.Unlock()

	request := httptest.NewRequest(http.MethodGet, "/dashboard/invoices/add-invoice", nil)
	request.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	recorder := httptest.NewRecorder()

	token, ok := GetTokenOrRedirect(recorder, request)
	if !ok {
		t.Fatal("expected token lookup to succeed")
	}
	if token != "fresh-token" {
		t.Fatalf("expected refreshed token, got %q", token)
	}
	if !refreshSeen {
		t.Fatal("expected refresh endpoint to be called")
	}
}
