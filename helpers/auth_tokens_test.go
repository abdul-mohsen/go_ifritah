package helpers

import (
	"afrita/config"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testJWTWithExpiry(t *testing.T, expiry time.Time) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]interface{}{"exp": expiry.Unix(), "role": "admin"})
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

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

func TestAccessTokenExpiryUsesJWTExp(t *testing.T) {
	issuedAt := time.Now().Truncate(time.Second)
	expectedExpiry := issuedAt.Add(30 * time.Minute)

	got := AccessTokenExpiry(testJWTWithExpiry(t, expectedExpiry), issuedAt)

	if !got.Equal(expectedExpiry) {
		t.Fatalf("expected JWT exp %v, got %v", expectedExpiry, got)
	}
}

func TestAccessTokenExpiryFallsBackForOpaqueToken(t *testing.T) {
	issuedAt := time.Now().Truncate(time.Second)

	got := AccessTokenExpiry("opaque-token", issuedAt)
	want := issuedAt.Add(defaultAccessTokenTTL)

	if !got.Equal(want) {
		t.Fatalf("expected fallback expiry %v, got %v", want, got)
	}
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
	expectedExpiry := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	freshToken := testJWTWithExpiry(t, expectedExpiry)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/refresh" {
			t.Fatalf("unexpected refresh path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer refresh-token" {
			t.Fatalf("unexpected refresh authorization header %q", got)
		}
		refreshSeen = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": freshToken, "refresh_token": "fresh-refresh"})
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
	if token != freshToken {
		t.Fatalf("expected refreshed token, got %q", token)
	}
	if !refreshSeen {
		t.Fatal("expected refresh endpoint to be called")
	}

	config.SessionTokensMutex.RLock()
	storedExpiry := config.SessionTokenExpiry[sessionID]
	config.SessionTokensMutex.RUnlock()
	if !storedExpiry.Equal(expectedExpiry) {
		t.Fatalf("expected stored expiry from JWT exp %v, got %v", expectedExpiry, storedExpiry)
	}
}
