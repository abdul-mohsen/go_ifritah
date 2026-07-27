package helpers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"afrita/config"
	"afrita/helpers"
)

// withFakeBackend points config.BackendDomain at an httptest server for the
// duration of the test, restoring the previous value on cleanup.
func withFakeBackend(t *testing.T, handler http.Handler) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prev := config.BackendDomain
	config.BackendDomain = srv.URL
	t.Cleanup(func() { config.BackendDomain = prev })
	return srv.URL
}

// ─────────────────────────────────────────────────────────────────────────────
// FetchNotifications
// ─────────────────────────────────────────────────────────────────────────────

func TestFetchNotifications_HappyPath(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/notification" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Errorf("limit = %q, want 20", got)
		}
		if r.Header.Get("Authorization") != "Bearer t-1" {
			t.Errorf("missing auth header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"id":          7,
				"user_id":     1,
				"type":        "low_stock",
				"title":       "مخزون منخفض",
				"message":     "Test",
				"resource":    "product",
				"resource_id": "12",
				"read":        false,
				"created_at":  time.Now().UTC().Format(time.RFC3339),
			}},
			"next_cursor": "abc",
			"has_more":    true,
		})
	}))

	resp, err := helpers.FetchNotifications(context.Background(), "t-1", helpers.NotificationListParams{})
	if err != nil {
		t.Fatalf("FetchNotifications: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].ID != 7 {
		t.Fatalf("items = %#v", resp.Items)
	}
	if resp.Items[0].IsRead {
		t.Errorf("expected unread")
	}
	if !resp.HasMore || resp.NextCursor != "abc" {
		t.Errorf("envelope wrong: %#v", resp)
	}
}

func TestFetchNotifications_AcceptsLegacyIsReadKey(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[{"id":1,"is_read":true}],"has_more":false}`)
	}))
	resp, err := helpers.FetchNotifications(context.Background(), "t", helpers.NotificationListParams{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Items[0].IsRead {
		t.Errorf("legacy is_read=true was not honoured")
	}
}

func TestFetchNotifications_AcceptsLegacyDataEnvelope(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"id":2,"read":false}],"has_more":false}`)
	}))
	resp, err := helpers.FetchNotifications(context.Background(), "t", helpers.NotificationListParams{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].ID != 2 {
		t.Fatalf("legacy data envelope was not normalized: %#v", resp.Items)
	}
}

func TestFetchNotifications_UnauthorizedTyped(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	_, err := helpers.FetchNotifications(context.Background(), "t", helpers.NotificationListParams{})
	if err != helpers.ErrNotifUnauthorized {
		t.Fatalf("err = %v, want ErrNotifUnauthorized", err)
	}
}

func TestFetchNotifications_LimitClamping(t *testing.T) {
	var seen string
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query().Get("limit")
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	_, _ = helpers.FetchNotifications(context.Background(), "t", helpers.NotificationListParams{Limit: 9999})
	if seen != "100" {
		t.Errorf("limit not clamped, got %q", seen)
	}
}

func TestFetchNotifications_UnreadOnlyAndCursor(t *testing.T) {
	var path string
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	_, _ = helpers.FetchNotifications(context.Background(), "t",
		helpers.NotificationListParams{Limit: 5, Cursor: "c1", UnreadOnly: true})
	if !strings.Contains(path, "unread_only=1") || !strings.Contains(path, "cursor=c1") {
		t.Errorf("query missing expected params: %q", path)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// FetchUnreadCount
// ─────────────────────────────────────────────────────────────────────────────

func TestFetchUnreadCount(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/notification/unread-count" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"count": 4}`)
	}))
	count, err := helpers.FetchUnreadCount(context.Background(), "t")
	if err != nil || count != 4 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MarkNotificationRead / MarkAllNotificationsRead
// ─────────────────────────────────────────────────────────────────────────────

func TestMarkNotificationRead(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v2/notification/9/read" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	if err := helpers.MarkNotificationRead(context.Background(), "t", 9); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestMarkNotificationRead_NotFoundTyped(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	err := helpers.MarkNotificationRead(context.Background(), "t", 9)
	if err != helpers.ErrNotifNotFound {
		t.Fatalf("err = %v, want ErrNotifNotFound", err)
	}
}

func TestMarkAllNotificationsRead(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v2/notification/read-all" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"count": 12}`)
	}))
	count, err := helpers.MarkAllNotificationsRead(context.Background(), "t")
	if err != nil || count != 12 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Config
// ─────────────────────────────────────────────────────────────────────────────

func TestGetNotificationConfig_Wrapped(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"low_stock_alert":true,"low_stock_threshold":7,"new_order_alert":true}}`)
	}))
	cfg, err := helpers.GetNotificationConfig(context.Background(), "t")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !cfg.LowStockAlert || cfg.LowStockThreshold != 7 || !cfg.NewOrderAlert {
		t.Errorf("cfg = %#v", cfg)
	}
}

func TestGetNotificationConfig_Unwrapped(t *testing.T) {
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"low_stock_alert":true,"low_stock_threshold":3}`)
	}))
	cfg, err := helpers.GetNotificationConfig(context.Background(), "t")
	if err != nil || cfg.LowStockThreshold != 3 {
		t.Fatalf("cfg=%#v err=%v", cfg, err)
	}
}

func TestUpdateNotificationConfig_PartialBodyForwarded(t *testing.T) {
	var seen map[string]any
	withFakeBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v2/notification/config" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&seen)
		_, _ = io.WriteString(w, `{"data":{"low_stock_threshold":11}}`)
	}))
	cfg, err := helpers.UpdateNotificationConfig(context.Background(), "t", map[string]any{
		"low_stock_threshold": 11,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg.LowStockThreshold != 11 {
		t.Errorf("cfg=%#v", cfg)
	}
	if _, ok := seen["low_stock_alert"]; ok {
		t.Errorf("partial PUT should not have sent low_stock_alert; payload=%v", seen)
	}
	if v, _ := seen["low_stock_threshold"].(float64); v != 11 {
		t.Errorf("low_stock_threshold not forwarded: %v", seen)
	}
}
