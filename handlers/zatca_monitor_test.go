package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"afrita/config"
	"afrita/helpers"
)

func setupZatcaMonitorSession(t *testing.T, sessionID string) {
	t.Helper()
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "live-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})
}

func configureZatcaMonitorBackend(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldBackend := config.BackendDomain
	oldClient := helpers.HttpClient
	config.BackendDomain = server.URL
	helpers.HttpClient = server.Client()
	t.Cleanup(func() {
		config.BackendDomain = oldBackend
		helpers.HttpClient = oldClient
	})
}

func TestHandleZatcaMonitorUsesLiveBackendDataConcurrently(t *testing.T) {
	setupZatcaMonitorSession(t, "zatca-monitor-live")

	var active, maxActive int32
	var requestsMu sync.Mutex
	requests := make(map[string]int)
	authHeaders := make(map[string]string)
	configureZatcaMonitorBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()
		requests[r.URL.Path]++
		authHeaders[r.URL.Path] = r.Header.Get("Authorization")
		requestsMu.Unlock()

		current := atomic.AddInt32(&active, 1)
		for {
			highest := atomic.LoadInt32(&maxActive)
			if current <= highest || atomic.CompareAndSwapInt32(&maxActive, highest, current) {
				break
			}
		}
		defer atomic.AddInt32(&active, -1)
		time.Sleep(30 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case zatcaMonitorStatsEndpoint:
			_ = json.NewEncoder(w).Encode(map[string]int{
				"total_submitted": 9,
				"accepted":        6,
				"warnings":        1,
				"rejected":        1,
				"pending":         1,
			})
		case zatcaMonitorBranchesEndpoint:
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"branch_id":          7,
					"branch_name":        "فرع حي",
					"zatca_status":       1,
					"cert_expiry":        "2027-01-02",
					"today_count":        4,
					"success_rate":       91.5,
					"last_submission_at": "2027-01-02T10:20:30Z",
				},
			})
		case zatcaMonitorSubmissionsPath:
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"invoice_id":   77,
					"invoice_no":   "INV-77",
					"branch_name":  "فرع حي",
					"status":       "accepted",
					"zatca_ref":    "ZAT-77",
					"warning_msg":  nil,
					"submitted_at": "2027-01-02T10:20:30Z",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/zatca-monitor", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "zatca-monitor-live"})
	recorder := httptest.NewRecorder()

	HandleZatcaMonitor(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"فرع حي", "INV-77", "9", "91"} {
		if !strings.Contains(body, expected) {
			t.Errorf("live monitor response is missing %q", expected)
		}
	}
	if strings.Contains(body, "1247") {
		t.Fatal("monitor still rendered hardcoded statistics")
	}
	if strings.Contains(body, "تعذر تحميل بيانات مراقبة زاتكا") {
		t.Fatal("successful monitor response rendered an error banner")
	}
	if got := atomic.LoadInt32(&maxActive); got < 2 {
		t.Fatalf("expected concurrent backend requests, maximum active requests was %d", got)
	}

	requestsMu.Lock()
	defer requestsMu.Unlock()
	if requests[zatcaMonitorStatsEndpoint] != 1 || requests[zatcaMonitorBranchesEndpoint] != 1 ||
		requests[zatcaMonitorSubmissionsPath] != 1 {
		t.Fatalf("unexpected backend request counts: %#v", requests)
	}
	for endpoint, auth := range authHeaders {
		if auth != "Bearer live-token" {
			t.Errorf("request %s used authorization %q", endpoint, auth)
		}
	}
}

func TestHandleZatcaMonitorRendersErrorBannerForPartialFailure(t *testing.T) {
	setupZatcaMonitorSession(t, "zatca-monitor-error")

	configureZatcaMonitorBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == zatcaMonitorBranchesEndpoint {
			http.Error(w, `{"error":"branches unavailable"}`, http.StatusBadGateway)
			return
		}
		switch r.URL.Path {
		case zatcaMonitorStatsEndpoint:
			_ = json.NewEncoder(w).Encode(map[string]int{"total_submitted": 3, "accepted": 3})
		case zatcaMonitorSubmissionsPath:
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"invoice_id": 88, "invoice_no": "INV-88", "branch_name": "فرع متاح", "status": "pending"},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/zatca-monitor", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "zatca-monitor-error"})
	recorder := httptest.NewRecorder()

	HandleZatcaMonitor(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected soft-failure status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `role="alert"`) {
		t.Fatal("partial backend failure did not render an error banner")
	}
	if !strings.Contains(body, "تعذر تحميل بيانات مراقبة زاتكا") {
		t.Fatal("error banner did not contain the localized monitor message")
	}
	if !strings.Contains(body, "INV-88") || !strings.Contains(body, "3") {
		t.Fatal("successful monitor sections were discarded after a partial failure")
	}
}
