package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"afrita/config"
	"afrita/helpers"
)

func TestHandleZatcaMonitorUsesAuthenticatedBackendData(t *testing.T) {
	const sessionID = "zatca-monitor-regression"

	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "live-monitor-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})

	var (
		requestsMu sync.Mutex
		requests   = make(map[string]int)
		authHeader = make(map[string]string)
	)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()
		requests[r.URL.Path]++
		authHeader[r.URL.Path] = r.Header.Get("Authorization")
		requestsMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/zatca/monitor/stats":
			_ = json.NewEncoder(w).Encode(map[string]int{
				"total_submitted":       7,
				"accepted":              4,
				"accepted_with_warning": 1,
				"warnings":              1,
				"rejected":              1,
				"pending":               1,
			})
		case "/api/v2/zatca/monitor/branches":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"branch_id":          42,
					"branch_name":        "فرع API الحقيقي",
					"zatca_status":       4,
					"cert_expiry":        "2027-03-10",
					"today_count":        5,
					"success_rate":       88.5,
					"last_submission_at": "2026-10-29T15:00:00Z",
				},
			})
		case "/api/v2/zatca/monitor/submissions":
			if r.URL.Query().Get("limit") != "50" {
				t.Errorf("submissions limit = %q, want 50", r.URL.Query().Get("limit"))
			}
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"invoice_id":         9001,
					"invoice_no":         "INV-9001",
					"branch_name":        "فرع API الحقيقي",
					"status":             "pending",
					"state":              "pending",
					"provider_reference": "PROVIDER-API-9001",
					"provider_warning":   "بانتظار نتيجة المزود",
					"submitted_at":       "2026-10-29T15:00:00Z",
					"last_attempt_at":    "2026-10-29T15:05:00Z",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(backend.Close)

	previousBackend := config.BackendDomain
	previousClient := helpers.HttpClient
	config.BackendDomain = backend.URL
	helpers.HttpClient = backend.Client()
	t.Cleanup(func() {
		config.BackendDomain = previousBackend
		helpers.HttpClient = previousClient
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/zatca-monitor", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	recorder := httptest.NewRecorder()

	HandleZatcaMonitor(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		"فرع API الحقيقي",
		"INV-9001",
		"PROVIDER-API-9001",
		"7",
		"بانتظار نتيجة المزود",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("live monitor response missing %q", expected)
		}
	}
	if strings.Contains(body, "1247") {
		t.Fatal("monitor rendered hard-coded statistics")
	}
	if strings.Contains(body, "الفرع الرئيسي") {
		t.Fatal("monitor rendered hard-coded branch data")
	}

	requestsMu.Lock()
	defer requestsMu.Unlock()
	for _, endpoint := range []string{
		"/api/v2/zatca/monitor/stats",
		"/api/v2/zatca/monitor/branches",
		"/api/v2/zatca/monitor/submissions",
	} {
		if requests[endpoint] != 1 {
			t.Errorf("backend request count for %s = %d, want 1", endpoint, requests[endpoint])
		}
		if authHeader[endpoint] != "Bearer live-monitor-token" {
			t.Errorf("authorization for %s = %q, want bearer token", endpoint, authHeader[endpoint])
		}
	}
}
