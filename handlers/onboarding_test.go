package handlers

import (
	"afrita/config"
	"afrita/helpers"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleOnboardingState(t *testing.T) {
	helpers.APICache.Delete("branches")
	helpers.APICache.Delete("stores")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/branch/all", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{{"id": 7, "name": "الفرع الرئيسي"}})
	})
	mux.HandleFunc("/api/v2/stores/all", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{{"id": 9, "name": "المخزن الرئيسي", "branch_id": 7}})
	})
	mux.HandleFunc("/api/v2/branch/7/zatca", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "الفرع الرئيسي", "zatca_status": 1, "csrlen": 1, "prodlen": 1,
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	previousDomain := config.BackendDomain
	previousClient := helpers.HttpClient
	config.BackendDomain = server.URL
	helpers.HttpClient = server.Client()
	defer func() {
		config.BackendDomain = previousDomain
		helpers.HttpClient = previousClient
	}()

	const sessionID = "onboarding-state-session"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "onboarding-token"
	config.SessionTokensMutex.Unlock()
	defer func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/onboarding/state", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	recorder := httptest.NewRecorder()
	HandleOnboardingState(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"branch_id":7`) {
		t.Fatalf("branch state missing: %s", body)
	}
	if !strings.Contains(body, `"configured":true`) {
		t.Fatalf("configured ZATCA state missing: %s", body)
	}
}

func TestHandleCompleteOnboarding(t *testing.T) {
	var saved bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/api/v2/settings" {
			var payload struct {
				Category string            `json:"category"`
				Settings map[string]string `json:"settings"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			saved = payload.Category == "company" && payload.Settings["onboarding_completed"] == "true"
		}
		_, _ = w.Write([]byte(`{"detail":"success"}`))
	}))
	defer server.Close()

	previousDomain := config.BackendDomain
	previousClient := helpers.HttpClient
	config.BackendDomain = server.URL
	helpers.HttpClient = server.Client()
	defer func() {
		config.BackendDomain = previousDomain
		helpers.HttpClient = previousClient
	}()

	const sessionID = "onboarding-complete-session"
	const token = "onboarding-complete-token"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = token
	config.SessionTokensMutex.Unlock()
	defer func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
		settingsByToken.Delete(tokenKey(token))
	}()

	req := httptest.NewRequest(http.MethodPost, "/api/onboarding/complete", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	recorder := httptest.NewRecorder()
	HandleCompleteOnboarding(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !saved {
		t.Fatal("completion marker was not sent to settings backend")
	}
	if got := GetSettingValue(token, "onboarding_completed"); got != "true" {
		t.Fatalf("completion marker = %q, want true", got)
	}
}

func TestHandleDashboardRedirectsIncompleteOnboarding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/settings" {
			_, _ = w.Write([]byte(`{"data":{"company":{"onboarding_completed":""}}}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	previousDomain := config.BackendDomain
	previousClient := helpers.HttpClient
	config.BackendDomain = server.URL
	helpers.HttpClient = server.Client()
	defer func() {
		config.BackendDomain = previousDomain
		helpers.HttpClient = previousClient
	}()

	const sessionID = "onboarding-redirect-session"
	const token = "onboarding-redirect-token"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = token
	config.SessionTokensMutex.Unlock()
	defer func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
		settingsByToken.Delete(tokenKey(token))
	}()

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	recorder := httptest.NewRecorder()
	HandleDashboard(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if location := recorder.Header().Get("Location"); location != "/dashboard/onboarding" {
		t.Fatalf("Location = %q, want /dashboard/onboarding", location)
	}
}

func TestHandleOnboardingRedirectsCompletedOnboarding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/settings" {
			_, _ = w.Write([]byte(`{"data":{"company":{"onboarding_completed":"true"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	previousDomain := config.BackendDomain
	previousClient := helpers.HttpClient
	config.BackendDomain = server.URL
	helpers.HttpClient = server.Client()
	defer func() {
		config.BackendDomain = previousDomain
		helpers.HttpClient = previousClient
	}()

	const sessionID = "onboarding-completed-session"
	const token = "onboarding-completed-token"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = token
	config.SessionTokensMutex.Unlock()
	defer func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
		settingsByToken.Delete(tokenKey(token))
	}()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/onboarding", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	recorder := httptest.NewRecorder()
	HandleOnboarding(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if location := recorder.Header().Get("Location"); location != "/dashboard" {
		t.Fatalf("Location = %q, want /dashboard", location)
	}
}
