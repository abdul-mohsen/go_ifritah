package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"afrita/helpers"

	"github.com/gorilla/mux"
)

// ─────────────────────────────────────────────────────────────────────────────
// Notification Config - Mock Endpoint
// ─────────────────────────────────────────────────────────────────────────────
//
// POST /api/notification-config
//
// Expected Request:
//
//	{
//	    "lowStockThreshold":  5,
//	    "pendingInvoiceDays": 7,
//	    "notifLowStock":      true,
//	    "notifPending":       true
//	}
//
// Expected Response:
//
//	{
//	    "success": true,
//	    "message": "تم حفظ إعدادات التنبيهات"
//	}
//
// TODO (backend): Persist notification config per user in the database.
//
//	Needs: POST /api/v2/user/{id}/notification-config on the backend.
//	       The Go handler should forward the payload to the real backend once available.
//	       File to change: this file (handlers/notifications.go) — replace the mock with a real API call.

// NotificationConfigRequest is the expected request payload for saving notification settings.
type NotificationConfigRequest struct {
	LowStockThreshold  int  `json:"lowStockThreshold"`
	PendingInvoiceDays int  `json:"pendingInvoiceDays"`
	NotifLowStock      bool `json:"notifLowStock"`
	NotifPending       bool `json:"notifPending"`
}

// NotificationConfigResponse is the expected response after saving notification settings.
type NotificationConfigResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// HandleNotificationConfig saves notification preferences.
// Currently a mock — config is saved in the browser (localStorage).
// When the real backend endpoint is ready, this will forward to:
//
//	POST /api/v2/user/{id}/notification-config
func HandleNotificationConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req NotificationConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(NotificationConfigResponse{
			Success: false,
			Message: "طلب غير صالح",
		})
		return
	}

	// TODO (backend): Forward to real backend API
	// token, ok := helpers.GetTokenOrRedirect(w, r)
	// if !ok { return }
	// payload, _ := json.Marshal(req)
	// apiReq, _ := http.NewRequest("POST", config.BackendDomain+"/api/v2/user/me/notification-config", bytes.NewBuffer(payload))
	// apiReq.Header.Set("Content-Type", "application/json")
	// resp, err := helpers.DoAuthedRequest(apiReq, token)
	// ... handle real response

	// Mock: just acknowledge
	_ = json.NewEncoder(w).Encode(NotificationConfigResponse{
		Success: true,
		Message: "تم حفظ إعدادات التنبيهات",
	})
}

// HandleNotifications renders the notifications list page.
func HandleNotifications(w http.ResponseWriter, r *http.Request) {
	_, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	all := NotificationStore.List()

	var unread, read []interface{}
	for i := range all {
		n := all[i]
		if n.IsRead {
			read = append(read, n)
		} else {
			unread = append(unread, n)
		}
	}

	helpers.Render(w, r, "notifications", map[string]interface{}{
		"title":         "التنبيهات",
		"notifications": all,
		"unread":        unread,
		"read":          read,
		"unread_count":  len(unread),
	})
}

// HandleMarkNotificationRead marks a single notification as read (HTMX/JSON).
func HandleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	_, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	found := NotificationStore.MarkRead(id)
	w.Header().Set("Content-Type", "application/json")
	if !found {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "التنبيه غير موجود"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// HandleMarkAllNotificationsRead marks all notifications as read.
func HandleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	_, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	count := NotificationStore.MarkAllRead()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   count,
	})
}

// HandleGetNotificationConfig returns current notification config (mock).
func HandleGetNotificationConfig(w http.ResponseWriter, r *http.Request) {
	_, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	cfg := NotifConfigStore.Get(1) // Default user ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}
