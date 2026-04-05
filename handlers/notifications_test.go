//go:build ignore
// +build ignore

// Disabled: depends on notifications_store.go.disabled mock stores.

package handlers

import (
	"afrita/config"
	"afrita/helpers"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/models"

	"github.com/gorilla/mux"
)

// ============================================================================
// Notification Store Tests
// ============================================================================

func TestNotificationStoreList(t *testing.T) {
	store := NewMockNotificationStore()
	list := store.List()

	if len(list) != 8 {
		t.Fatalf("expected 8 seed notifications, got %d", len(list))
	}

	// Should be sorted by CreatedAt desc (newest first)
	for i := 1; i < len(list); i++ {
		if list[i].CreatedAt.After(list[i-1].CreatedAt) {
			t.Errorf("notifications not sorted desc: %v after %v", list[i].CreatedAt, list[i-1].CreatedAt)
		}
	}
}

func TestNotificationStoreUnreadCount(t *testing.T) {
	store := NewMockNotificationStore()
	count := store.UnreadCount()

	// 6 seeded as unread, 2 as read
	if count != 6 {
		t.Errorf("expected 6 unread, got %d", count)
	}
}

func TestNotificationStoreMarkRead(t *testing.T) {
	store := NewMockNotificationStore()

	if !store.MarkRead(1) {
		t.Fatal("expected MarkRead(1) to return true")
	}

	n := store.GetByID(1)
	if n == nil || !n.IsRead {
		t.Error("notification 1 should be marked as read")
	}

	// Unread count should decrease
	if count := store.UnreadCount(); count != 5 {
		t.Errorf("expected 5 unread after marking 1, got %d", count)
	}

	// Non-existent ID
	if store.MarkRead(999) {
		t.Error("expected MarkRead(999) to return false")
	}
}

func TestNotificationStoreMarkAllRead(t *testing.T) {
	store := NewMockNotificationStore()

	marked := store.MarkAllRead()
	if marked != 6 {
		t.Errorf("expected 6 marked as read, got %d", marked)
	}

	if count := store.UnreadCount(); count != 0 {
		t.Errorf("expected 0 unread after mark-all, got %d", count)
	}

	// Calling again should mark 0
	marked = store.MarkAllRead()
	if marked != 0 {
		t.Errorf("expected 0 newly marked, got %d", marked)
	}
}

func TestNotificationStoreCreate(t *testing.T) {
	store := NewMockNotificationStore()
	initial := len(store.List())

	n := &models.Notification{
		UserID:  1,
		Type:    "info",
		Title:   "إشعار اختبار",
		Message: "هذا إشعار اختباري",
	}
	id := store.Create(n)

	if id < 9 {
		t.Errorf("expected new ID >= 9, got %d", id)
	}
	if len(store.List()) != initial+1 {
		t.Error("list length should increase by 1")
	}

	created := store.GetByID(id)
	if created == nil {
		t.Fatal("created notification not found")
	}
	if created.Title != "إشعار اختبار" {
		t.Errorf("title = %q, want 'إشعار اختبار'", created.Title)
	}
}

func TestNotificationStoreGetByID(t *testing.T) {
	store := NewMockNotificationStore()

	n := store.GetByID(1)
	if n == nil {
		t.Fatal("expected notification 1 to exist")
	}
	if n.Type != "low_stock" {
		t.Errorf("type = %q, want 'low_stock'", n.Type)
	}

	if store.GetByID(999) != nil {
		t.Error("expected nil for non-existent ID")
	}
}

// ============================================================================
// Notification Config Store Tests
// ============================================================================

func TestNotifConfigStoreGetDefault(t *testing.T) {
	store := NewMockNotifConfigStore()

	cfg := store.Get(1) // seeded user
	if !cfg.LowStockAlert {
		t.Error("expected LowStockAlert = true for seeded user")
	}
	if cfg.LowStockThreshold != 5 {
		t.Errorf("LowStockThreshold = %d, want 5", cfg.LowStockThreshold)
	}

	// Unknown user gets defaults
	cfg2 := store.Get(999)
	if !cfg2.LowStockAlert {
		t.Error("expected default LowStockAlert = true")
	}
	if cfg2.UserID != 999 {
		t.Errorf("UserID = %d, want 999", cfg2.UserID)
	}
}

func TestNotifConfigStoreSave(t *testing.T) {
	store := NewMockNotifConfigStore()

	cfg := models.NotificationConfig{
		UserID:             2,
		LowStockAlert:      false,
		LowStockThreshold:  10,
		PendingInvoiceDays: 14,
		EmailEnabled:       true,
	}
	store.Save(cfg)

	got := store.Get(2)
	if got.LowStockAlert {
		t.Error("expected LowStockAlert = false after save")
	}
	if got.LowStockThreshold != 10 {
		t.Errorf("LowStockThreshold = %d, want 10", got.LowStockThreshold)
	}
	if !got.EmailEnabled {
		t.Error("expected EmailEnabled = true after save")
	}
}

// ============================================================================
// Notification Handler Tests
// ============================================================================

func TestHandleNotificationsPage(t *testing.T) {
	NotificationStore = NewMockNotificationStore()
	config.SessionTokensMutex.Lock()
	config.SessionTokens["test-session"] = "test-token"
	config.SessionTokensMutex.Unlock()

	req := httptest.NewRequest("GET", "/dashboard/notifications", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleNotifications(w, req)
	body := w.Body.String()

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Should contain notification titles
	mustContain := []string{
		"الإشعارات",          // page title
		"غير مقروءة",        // unread section header
		"مقروءة",            // read section header
		"تنبيه مخزون منخفض", // notification 1 title
		"فاتورة مستحقة",     // notification 2 title fragment
		"تعيين الكل كمقروء", // mark-all button
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("notifications page missing: %s", s)
		}
	}
}

func TestHandleNotificationsRequiresAuth(t *testing.T) {
	req := httptest.NewRequest("GET", "/dashboard/notifications", nil)
	w := httptest.NewRecorder()

	HandleNotifications(w, req)

	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (no auth), got %d", w.Code)
	}
}

func TestHandleMarkNotificationRead(t *testing.T) {
	NotificationStore = NewMockNotificationStore()

	req := httptest.NewRequest("POST", "/api/notifications/1/read", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	HandleMarkNotificationRead(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success: true")
	}
}

func TestHandleMarkNotificationReadNotFound(t *testing.T) {
	NotificationStore = NewMockNotificationStore()

	req := httptest.NewRequest("POST", "/api/notifications/999/read", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	w := httptest.NewRecorder()

	HandleMarkNotificationRead(w, req)

	// Now returns 200 always (best-effort API call, no mock validation)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleMarkNotificationReadInvalidID(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/notifications/abc/read", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "abc"})
	w := httptest.NewRecorder()

	HandleMarkNotificationRead(w, req)

	// Now returns 200 always (best-effort API call)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleMarkAllNotificationsRead(t *testing.T) {
	NotificationStore = NewMockNotificationStore()

	req := httptest.NewRequest("POST", "/api/notifications/read-all", nil)
	w := httptest.NewRecorder()

	HandleMarkAllNotificationsRead(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success: true")
	}
}

func TestHandleNotificationConfig(t *testing.T) {
	NotifConfigStore = NewMockNotifConfigStore()

	payload := `{"lowStockThreshold":10,"pendingInvoiceDays":14,"notifLowStock":true,"notifPending":false,"notifOrders":true}`
	req := httptest.NewRequest("POST", "/api/notification-config", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleNotificationConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success: true")
	}

	// Verify config was saved
	cfg := NotifConfigStore.Get(1)
	if cfg.LowStockThreshold != 10 {
		t.Errorf("LowStockThreshold = %d, want 10", cfg.LowStockThreshold)
	}
	if !cfg.NewOrderAlert {
		t.Error("expected NewOrderAlert = true")
	}
}

func TestHandleNotificationConfigInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/notification-config", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleNotificationConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetNotificationConfig(t *testing.T) {
	NotifConfigStore = NewMockNotifConfigStore()

	req := httptest.NewRequest("GET", "/api/notification-config", nil)
	w := httptest.NewRecorder()

	HandleGetNotificationConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success: true")
	}
}

func TestHandleCurrentUser(t *testing.T) {
	UserStore = NewMockUserStore()

	req := httptest.NewRequest("GET", "/api/users/me", nil)
	w := httptest.NewRecorder()

	HandleCurrentUser(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp["username"] == nil || resp["username"] == "" {
		t.Error("expected non-empty username")
	}
	if resp["email"] == nil || resp["email"] == "" {
		t.Error("expected non-empty email")
	}
	if resp["role"] == nil || resp["role"] == "" {
		t.Error("expected non-empty role")
	}
}

// ============================================================================
// Register Integration with UserStore
// ============================================================================

func TestHandleRegisterPostCreatesUser(t *testing.T) {
	UserStore = NewMockUserStore()
	initial := len(UserStore.List())

	payload := `{"username":"testuser","email":"test@new.com","password":"Test1234","full_name":"مستخدم اختبار","phone":"0501111111"}`
	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleRegisterPost(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("expected success: true, got %v", resp)
	}

	// User should be in store
	if len(UserStore.List()) != initial+1 {
		t.Errorf("expected %d users after register, got %d", initial+1, len(UserStore.List()))
	}
}

func TestHandleRegisterPostDuplicateUsername(t *testing.T) {
	UserStore = NewMockUserStore()

	// Get an existing username
	users := UserStore.List()
	existingUsername := users[0].Username

	payload := `{"username":"` + existingUsername + `","email":"unique@test.com","password":"Test1234"}`
	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleRegisterPost(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d", w.Code)
	}
}

func TestHandleRegisterPostDuplicateEmail(t *testing.T) {
	UserStore = NewMockUserStore()

	users := UserStore.List()
	existingEmail := users[0].Email

	payload := `{"username":"uniqueuser","email":"` + existingEmail + `","password":"Test1234"}`
	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleRegisterPost(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d", w.Code)
	}
}

func TestHandleRegisterPostMissingFields(t *testing.T) {
	payload := `{"username":"","email":"","password":""}`
	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleRegisterPost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ============================================================================
// Backend Integration Tests — JSON deserialization + caching
// ============================================================================

// TestNotificationJSONReadField verifies that backend JSON with "read" field
// (not "is_read") correctly deserializes into the Go Notification model.
func TestNotificationJSONReadField(t *testing.T) {
	backendJSON := `{"data": [
		{"id": 1, "type": "low_stock", "title": "مخزون منخفض", "message": "فلتر زيت - الكمية: 2", "read": true, "created_at": "2026-03-25T10:00:00Z"},
		{"id": 2, "type": "new_order", "title": "طلب جديد", "message": "طلب كبير", "read": false, "created_at": "2026-03-25T09:00:00Z"}
	]}`

	var wrapper struct {
		Data []models.Notification `json:"data"`
	}
	if err := json.Unmarshal([]byte(backendJSON), &wrapper); err != nil {
		t.Fatalf("failed to unmarshal backend JSON: %v", err)
	}

	if len(wrapper.Data) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(wrapper.Data))
	}

	// ID 1 has "read": true → IsRead must be true
	if !wrapper.Data[0].IsRead {
		t.Error("notification 1: expected IsRead=true (backend sent 'read': true)")
	}
	// ID 2 has "read": false → IsRead must be false
	if wrapper.Data[1].IsRead {
		t.Error("notification 2: expected IsRead=false (backend sent 'read': false)")
	}
}

// TestFetchNotificationsCaching verifies that FetchNotifications caches
// results and subsequent calls return cached data.
func TestFetchNotificationsCaching(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data": [{"id": %d, "type": "info", "title": "test", "message": "msg", "read": false, "created_at": "2026-03-25T10:00:00Z"}]}`, callCount)
	}))
	defer ts.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = ts.URL
	defer func() { config.BackendDomain = origDomain }()

	helpers.APICache.Delete("notifications")

	// First call should hit the server
	notifs, err := helpers.FetchNotifications("test-token")
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	if callCount != 1 {
		t.Errorf("expected 1 server call, got %d", callCount)
	}

	// Second call should use cache (no new server hit)
	notifs2, err := helpers.FetchNotifications("test-token")
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if len(notifs2) != 1 {
		t.Fatalf("expected 1 cached notification, got %d", len(notifs2))
	}
	if callCount != 1 {
		t.Errorf("expected still 1 server call (cached), got %d", callCount)
	}

	// Clean up
	helpers.APICache.Delete("notifications")
}

// TestMarkReadInvalidatesCache verifies that marking a notification as read
// clears the notifications cache.
func TestMarkReadInvalidatesCache(t *testing.T) {
	helpers.APICache.Set("notifications", []models.Notification{
		{ID: 1, IsRead: false, Title: "cached"},
	}, 5*60*1000)

	_, found := helpers.APICache.Get("notifications")
	if !found {
		t.Fatal("expected notifications to be in cache before mark-read")
	}

	req := httptest.NewRequest("POST", "/api/notifications/1/read", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	HandleMarkNotificationRead(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	_, found = helpers.APICache.Get("notifications")
	if found {
		t.Error("expected notifications cache to be cleared after mark-read")
	}
}

// TestMarkAllReadInvalidatesCache verifies that mark-all-read clears cache.
func TestMarkAllReadInvalidatesCache(t *testing.T) {
	helpers.APICache.Set("notifications", []models.Notification{
		{ID: 1, IsRead: false, Title: "cached"},
	}, 5*60*1000)

	req := httptest.NewRequest("POST", "/api/notifications/read-all", nil)
	w := httptest.NewRecorder()

	HandleMarkAllNotificationsRead(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	_, found := helpers.APICache.Get("notifications")
	if found {
		t.Error("expected notifications cache to be cleared after mark-all-read")
	}
}

// TestNotificationCountFromCache verifies the bell badge count uses
// cached notifications when available.
func TestNotificationCountFromCache(t *testing.T) {
	// Set up cached notifications with 2 unread, 1 read
	cached := []models.Notification{
		{ID: 1, IsRead: false, Title: "unread 1"},
		{ID: 2, IsRead: false, Title: "unread 2"},
		{ID: 3, IsRead: true, Title: "read 1"},
	}
	helpers.APICache.Set("notifications", cached, 5*60*1000)

	if helpers.NotificationCountFunc == nil {
		t.Skip("NotificationCountFunc not wired")
	}
	count := helpers.NotificationCountFunc()

	if count != 2 {
		t.Errorf("expected bell badge count=2 from cache, got %d", count)
	}

	helpers.APICache.Delete("notifications")
}
