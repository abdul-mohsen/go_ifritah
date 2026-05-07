package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"afrita/helpers"
	"afrita/models"

	"github.com/gorilla/mux"
)

// ─────────────────────────────────────────────────────────────────────────────
// Notification handlers — proxy to ifritah-go /api/v2/notification*
// ─────────────────────────────────────────────────────────────────────────────
//
// Routes (registered in main.go):
//
//   GET  /dashboard/notifications              page
//   POST /api/notifications/{id}/read          mark one
//   POST /api/notifications/read-all           mark all
//   GET  /api/notifications/unread-count       JSON for the bell badge
//   GET  /api/notification-config              prefs
//   POST /api/notification-config              prefs save (kept for FE backwards compat)
//   PUT  /api/notification-config              prefs save (canonical)

// notifPageSize is the number of notifications fetched for the page render.
const notifPageSize = 50

// HandleNotifications renders the notifications list page using live backend data.
func HandleNotifications(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := helpers.FetchNotifications(ctx, token, helpers.NotificationListParams{
		Limit: notifPageSize,
	})
	if err != nil {
		if errors.Is(err, helpers.ErrNotifUnauthorized) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		// Soft-fail: render the page empty rather than 500ing the user out of
		// their dashboard. Log the raw error server-side; surface a generic
		// user-safe message to the template (avoids leaking backend hostnames,
		// internal paths, or DB hints).
		log.Printf("[notifications] backend fetch failed: %v", err)
		helpers.Render(w, r, "notifications", map[string]interface{}{
			"title":         "التنبيهات",
			"notifications": []models.Notification{},
			"unread":        []models.Notification{},
			"read":          []models.Notification{},
			"unread_count":  0,
			"error":         "فشل تحميل التنبيهات",
		})
		return
	}

	var unread, read []models.Notification
	for _, n := range resp.Items {
		if n.IsRead {
			read = append(read, n)
		} else {
			unread = append(unread, n)
		}
	}

	helpers.Render(w, r, "notifications", map[string]interface{}{
		"title":         "التنبيهات",
		"notifications": resp.Items,
		"unread":        unread,
		"read":          read,
		"unread_count":  len(unread),
		"next_cursor":   resp.NextCursor,
		"has_more":      resp.HasMore,
	})
}

// HandleMarkNotificationRead marks a single notification as read.
func HandleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeNotifJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "invalid id",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := helpers.MarkNotificationRead(ctx, token, id); err != nil {
		writeNotifError(w, r, err)
		return
	}
	writeNotifJSON(w, http.StatusOK, map[string]any{"success": true})
}

// HandleMarkAllNotificationsRead marks every unread notification as read.
func HandleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	count, err := helpers.MarkAllNotificationsRead(ctx, token)
	if err != nil {
		writeNotifError(w, r, err)
		return
	}
	writeNotifJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"count":   count,
	})
}

// HandleNotificationUnreadCount returns the unread badge count.
// Used by the JS bell poller in static/js/notifications.js.
func HandleNotificationUnreadCount(w http.ResponseWriter, r *http.Request) {
	token := helpers.GetTokenFromRequest(r)
	if token == "" {
		writeNotifJSON(w, http.StatusUnauthorized, map[string]any{"count": 0})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	count, err := helpers.FetchUnreadCount(ctx, token)
	if err != nil {
		if errors.Is(err, helpers.ErrNotifUnauthorized) {
			writeNotifJSON(w, http.StatusUnauthorized, map[string]any{"count": 0})
			return
		}
		writeNotifJSON(w, http.StatusBadGateway, map[string]any{
			"count": 0,
			"error": "upstream unavailable",
		})
		return
	}
	writeNotifJSON(w, http.StatusOK, map[string]any{"count": count})
}

// HandleGetNotificationConfig returns the current user's notification preferences.
func HandleGetNotificationConfig(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cfg, err := helpers.GetNotificationConfig(ctx, token)
	if err != nil {
		writeNotifError(w, r, err)
		return
	}
	writeNotifJSON(w, http.StatusOK, cfg)
}

// notificationConfigInbound is the request payload accepted from the FE.
// All fields are optional; unspecified keys are not forwarded so the backend
// preserves their previous value (partial-update contract).
type notificationConfigInbound struct {
	LowStockAlert      *bool `json:"low_stock_alert,omitempty"`
	LowStockThreshold  *int  `json:"low_stock_threshold,omitempty"`
	PendingInvoiceDays *int  `json:"pending_invoice_days,omitempty"`
	NewOrderAlert      *bool `json:"new_order_alert,omitempty"`
	PaymentDueAlert    *bool `json:"payment_due_alert,omitempty"`
	DailySummary       *bool `json:"daily_summary,omitempty"`
	EmailEnabled       *bool `json:"email_enabled,omitempty"`

	// Legacy keys accepted from the older settings page form. Mapped onto the
	// canonical fields if the corresponding canonical key is absent.
	LegacyLowStockThreshold  *int  `json:"lowStockThreshold,omitempty"`
	LegacyPendingInvoiceDays *int  `json:"pendingInvoiceDays,omitempty"`
	LegacyNotifLowStock      *bool `json:"notifLowStock,omitempty"`
	LegacyNotifPending       *bool `json:"notifPending,omitempty"`
}

// HandleNotificationConfig saves notification preferences (POST or PUT).
func HandleNotificationConfig(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	var in notificationConfigInbound
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeNotifJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "طلب غير صالح",
		})
		return
	}

	partial := map[string]any{}
	if in.LowStockAlert != nil {
		partial["low_stock_alert"] = *in.LowStockAlert
	} else if in.LegacyNotifLowStock != nil {
		partial["low_stock_alert"] = *in.LegacyNotifLowStock
	}
	if in.LowStockThreshold != nil {
		partial["low_stock_threshold"] = *in.LowStockThreshold
	} else if in.LegacyLowStockThreshold != nil {
		partial["low_stock_threshold"] = *in.LegacyLowStockThreshold
	}
	if in.PendingInvoiceDays != nil {
		partial["pending_invoice_days"] = *in.PendingInvoiceDays
	} else if in.LegacyPendingInvoiceDays != nil {
		partial["pending_invoice_days"] = *in.LegacyPendingInvoiceDays
	}
	if in.NewOrderAlert != nil {
		partial["new_order_alert"] = *in.NewOrderAlert
	}
	if in.PaymentDueAlert != nil {
		partial["payment_due_alert"] = *in.PaymentDueAlert
	} else if in.LegacyNotifPending != nil {
		partial["payment_due_alert"] = *in.LegacyNotifPending
	}
	if in.DailySummary != nil {
		partial["daily_summary"] = *in.DailySummary
	}
	if in.EmailEnabled != nil {
		partial["email_enabled"] = *in.EmailEnabled
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cfg, err := helpers.UpdateNotificationConfig(ctx, token, partial)
	if err != nil {
		writeNotifError(w, r, err)
		return
	}

	writeNotifJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "تم حفظ إعدادات التنبيهات",
		"config":  cfg,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func writeNotifJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeNotifError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, helpers.ErrNotifUnauthorized):
		helpers.HandleUnauthorized(w, r)
	case errors.Is(err, helpers.ErrNotifNotFound):
		writeNotifJSON(w, http.StatusNotFound, map[string]any{
			"success": false,
			"error":   "التنبيه غير موجود",
		})
	default:
		// Don't echo the raw upstream error to clients (info disclosure).
		// Log it for ops, return a stable user-safe message.
		log.Printf("[notifications] upstream error: %v", err)
		writeNotifJSON(w, http.StatusBadGateway, map[string]any{
			"success": false,
			"error":   "فشل الاتصال بالخادم",
		})
	}
}
