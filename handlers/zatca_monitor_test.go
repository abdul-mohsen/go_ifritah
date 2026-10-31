package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"
)

func TestHandleZatcaMonitorUsesDurableBackendRows(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/zatca/monitor/stats":
			_ = json.NewEncoder(w).Encode(map[string]int{
				"total_submitted": 2,
				"accepted":        1,
				"warnings":        0,
				"rejected":        1,
				"pending":         0,
			})
		case "/api/v2/zatca/monitor/branches":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{{
				"branch_id":          7,
				"branch_name":        "الفرع النظيف",
				"zatca_status":       1,
				"today_count":        2,
				"success_rate":       50.0,
				"last_submission_at": "2026-10-31T08:00:00Z",
			}})
		case "/api/v2/zatca/monitor/submissions":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"document_type":   "bill",
					"source_id":       42,
					"document_number": "INV-42",
					"branch_name":     "الفرع النظيف",
					"status":          "rejected",
					"status_code":     2,
					"rejection_msg":   "invalid invoice",
				},
				{
					"document_type":   "bill",
					"source_id":       43,
					"document_number": "INV-43",
					"branch_name":     "الفرع النظيف",
					"status":          "",
					"status_code":     2,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	previousBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = previousBackend })

	sessionID := "zatca-monitor-clean"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "test-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/zatca-monitor", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	w := httptest.NewRecorder()
	HandleZatcaMonitor(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"الفرع النظيف", "INV-42", "INV-43", "مرفوضة", "مقبولة"} {
		if !strings.Contains(body, want) {
			t.Errorf("monitor body missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "1247") || strings.Contains(body, "الفرع الرئيسي") {
		t.Fatal("monitor rendered legacy mock data")
	}
	if !strings.Contains(body, `data-bill-state="rejected"`) {
		t.Fatal("rejected durable monitor row was not projected as rejected")
	}
}
