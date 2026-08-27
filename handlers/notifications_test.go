package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"

	"github.com/gorilla/mux"
)

// notifTestBackend wires a fake backend, registers a session token, and
// returns helpers to build authenticated requests + cleanup.
type notifTestBackend struct {
	srv     *httptest.Server
	sessKey string
}

func newNotifTestBackend(t *testing.T, h http.Handler) *notifTestBackend {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	prev := config.BackendDomain
	config.BackendDomain = srv.URL
	t.Cleanup(func() { config.BackendDomain = prev })

	const sessKey = "notif-test-sess"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessKey] = "notif-test-token"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessKey)
		config.SessionTokensMutex.Unlock()
	})

	return &notifTestBackend{srv: srv, sessKey: sessKey}
}

func (b *notifTestBackend) authedRequest(method, path string, body string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.AddCookie(&http.Cookie{Name: "session_id", Value: b.sessKey})
	return r
}

// ─────────────────────────────────────────────────────────────────────────────

func TestHandleNotificationUnreadCount_ProxiesToBackend(t *testing.T) {
	var hit string
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = r.Method + " " + r.URL.Path
		_, _ = io.WriteString(w, `{"count": 3}`)
	}))

	req := b.authedRequest(http.MethodGet, "/api/notifications/unread-count", "")
	w := httptest.NewRecorder()
	HandleNotificationUnreadCount(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if hit != "GET /api/v2/notification/unread-count" {
		t.Errorf("backend hit = %q", hit)
	}
	var got map[string]int
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["count"] != 3 {
		t.Errorf("count=%v", got)
	}
}

func TestHandleNotificationUnreadCount_BackendDownReturnsZero(t *testing.T) {
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	req := b.authedRequest(http.MethodGet, "/api/notifications/unread-count", "")
	w := httptest.NewRecorder()
	HandleNotificationUnreadCount(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["count"] != float64(0) {
		t.Errorf("expected count=0 on failure, got %v", got)
	}
}

func TestHandleNotificationsRendersBackendItems(t *testing.T) {
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/notification" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"items":[{"id":7,"type":"system","title":"تنبيه","message":"تم الحفظ","read":false,"created_at":"2026-02-14T10:00:00Z"}]}`)
	}))
	req := b.authedRequest(http.MethodGet, "/dashboard/notifications", "")
	w := httptest.NewRecorder()
	HandleNotifications(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "تم الحفظ") || !strings.Contains(body, "تنبيه") {
		t.Fatalf("notification item not rendered: %s", body[:min(len(body), 500)])
	}
}

func TestHandleMarkNotificationRead_HappyPath(t *testing.T) {
	var hit string
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	req := b.authedRequest(http.MethodPost, "/api/notifications/42/read", "")
	req = mux.SetURLVars(req, map[string]string{"id": "42"})
	w := httptest.NewRecorder()
	HandleMarkNotificationRead(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if hit != "PUT /api/v2/notification/42/read" {
		t.Errorf("backend hit = %q", hit)
	}
}

func TestHandleMarkNotificationRead_BadID(t *testing.T) {
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatalf("backend should not be called on bad id")
	}))
	req := b.authedRequest(http.MethodPost, "/api/notifications/not-a-number/read", "")
	req = mux.SetURLVars(req, map[string]string{"id": "not-a-number"})
	w := httptest.NewRecorder()
	HandleMarkNotificationRead(w, req)

	_ = b // unused
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleMarkAllNotificationsRead(t *testing.T) {
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/notification/read-all" {
			t.Errorf("path=%s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"count": 5}`)
	}))
	req := b.authedRequest(http.MethodPost, "/api/notifications/read-all", "")
	w := httptest.NewRecorder()
	HandleMarkAllNotificationsRead(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["count"] != float64(5) || got["success"] != true {
		t.Errorf("body=%v", got)
	}
}

func TestHandleNotificationConfig_ForwardsPartialPayload(t *testing.T) {
	var captured map[string]any
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = io.WriteString(w, `{"data":{"low_stock_threshold":7}}`)
	}))
	body := `{"low_stock_threshold": 7, "low_stock_alert": true}`
	req := b.authedRequest(http.MethodPost, "/api/notification-config", body)
	w := httptest.NewRecorder()
	HandleNotificationConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if captured["low_stock_threshold"] != float64(7) || captured["low_stock_alert"] != true {
		t.Errorf("forwarded payload = %v", captured)
	}
	if _, ok := captured["payment_due_alert"]; ok {
		t.Errorf("untouched fields should not be forwarded; got %v", captured)
	}
}

func TestHandleNotificationConfig_AcceptsLegacyKeys(t *testing.T) {
	var captured map[string]any
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = io.WriteString(w, `{}`)
	}))
	body := `{"lowStockThreshold": 8, "notifLowStock": true, "notifPending": false}`
	req := b.authedRequest(http.MethodPost, "/api/notification-config", body)
	w := httptest.NewRecorder()
	HandleNotificationConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if captured["low_stock_threshold"] != float64(8) {
		t.Errorf("legacy lowStockThreshold not mapped: %v", captured)
	}
	if captured["low_stock_alert"] != true {
		t.Errorf("legacy notifLowStock not mapped: %v", captured)
	}
	if captured["payment_due_alert"] != false {
		t.Errorf("legacy notifPending not mapped: %v", captured)
	}
}

func TestHandleGetNotificationConfig(t *testing.T) {
	b := newNotifTestBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"low_stock_threshold":4,"low_stock_alert":true}}`)
	}))
	req := b.authedRequest(http.MethodGet, "/api/notification-config", "")
	w := httptest.NewRecorder()
	HandleGetNotificationConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var cfg map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &cfg)
	if cfg["low_stock_threshold"] != float64(4) {
		t.Errorf("threshold not surfaced: %v", cfg)
	}
}
