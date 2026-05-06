package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"afrita/config"
	"afrita/models"
)

// ============================================================================
// Notifications backend client
// ============================================================================
//
// Talks to the ifritah-go backend's /api/v2/notification* endpoints.
// All calls require a session access token (JWT). Errors fall into three
// buckets: ErrNotifUnauthorized (401), ErrNotifNotFound (404), or a generic
// wrapped error for anything else.

const notifBasePath = "/api/v2/notification"

// errFmtBuildRequest is the wrapper used when http.NewRequestWithContext
// fails inside a notification client method. Extracted to avoid duplicating
// the literal across the six call sites.
const errFmtBuildRequest = "notification: build request: %w"

// ErrNotifUnauthorized signals the access token is invalid / expired.
var ErrNotifUnauthorized = errors.New("notification: unauthorized")

// ErrNotifNotFound signals the resource (e.g. id) does not exist.
var ErrNotifNotFound = errors.New("notification: not found")

// NotificationListResponse mirrors the cursor-paginated envelope returned by
// the backend's GET /api/v2/notification endpoint.
type NotificationListResponse struct {
	Items      []models.Notification `json:"items"`
	NextCursor string                `json:"next_cursor"`
	PrevCursor string                `json:"prev_cursor"`
	HasMore    bool                  `json:"has_more"`
}

// NotificationListParams configures the list query.
type NotificationListParams struct {
	Limit      int
	Cursor     string
	UnreadOnly bool
}

// FetchNotifications returns a page of notifications for the authenticated user.
func FetchNotifications(ctx context.Context, token string, p NotificationListParams) (NotificationListResponse, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.UnreadOnly {
		q.Set("unread_only", "1")
	}

	u := config.BackendDomain + notifBasePath + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return NotificationListResponse{}, fmt.Errorf(errFmtBuildRequest, err)
	}

	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return NotificationListResponse{}, fmt.Errorf("notification: list: %w", err)
	}
	defer resp.Body.Close()

	if err := notifCheckStatus(resp); err != nil {
		return NotificationListResponse{}, err
	}

	var out NotificationListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return NotificationListResponse{}, fmt.Errorf("notification: decode list: %w", err)
	}
	return out, nil
}

// FetchUnreadCount returns the number of unread notifications for the user.
func FetchUnreadCount(ctx context.Context, token string) (int, error) {
	u := config.BackendDomain + notifBasePath + "/unread-count"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, fmt.Errorf(errFmtBuildRequest, err)
	}
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return 0, fmt.Errorf("notification: unread-count: %w", err)
	}
	defer resp.Body.Close()
	if err := notifCheckStatus(resp); err != nil {
		return 0, err
	}
	var out struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("notification: decode count: %w", err)
	}
	return out.Count, nil
}

// MarkNotificationRead marks a single notification as read.
func MarkNotificationRead(ctx context.Context, token string, id int) error {
	u := fmt.Sprintf("%s%s/%d/read", config.BackendDomain, notifBasePath, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return fmt.Errorf(errFmtBuildRequest, err)
	}
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return fmt.Errorf("notification: mark-read: %w", err)
	}
	defer resp.Body.Close()
	return notifCheckStatus(resp)
}

// MarkAllNotificationsRead marks every unread notification as read.
// Returns the count marked when the backend reports it; otherwise 0.
func MarkAllNotificationsRead(ctx context.Context, token string) (int, error) {
	u := config.BackendDomain + notifBasePath + "/read-all"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return 0, fmt.Errorf(errFmtBuildRequest, err)
	}
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return 0, fmt.Errorf("notification: mark-all-read: %w", err)
	}
	defer resp.Body.Close()
	if err := notifCheckStatus(resp); err != nil {
		return 0, err
	}
	var out struct {
		Count int `json:"count"`
	}
	// Count is best-effort — older backends may not return it.
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Count, nil
}

// GetNotificationConfig returns the user's current notification preferences.
// The backend wraps the config in `{"data": {...}}`; if missing it falls back
// to documented defaults.
func GetNotificationConfig(ctx context.Context, token string) (models.NotificationConfig, error) {
	u := config.BackendDomain + notifBasePath + "/config"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return models.NotificationConfig{}, fmt.Errorf(errFmtBuildRequest, err)
	}
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return models.NotificationConfig{}, fmt.Errorf("notification: get-config: %w", err)
	}
	defer resp.Body.Close()
	if err := notifCheckStatus(resp); err != nil {
		return models.NotificationConfig{}, err
	}
	body, _ := io.ReadAll(resp.Body)
	// Try wrapped first, then unwrapped.
	var wrapped struct {
		Data models.NotificationConfig `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Data != (models.NotificationConfig{}) {
		return wrapped.Data, nil
	}
	var direct models.NotificationConfig
	if err := json.Unmarshal(body, &direct); err != nil {
		return models.NotificationConfig{}, fmt.Errorf("notification: decode config: %w", err)
	}
	return direct, nil
}

// UpdateNotificationConfig sends a PUT with the provided partial fields.
// Pass a map so callers can omit keys they don't want to change.
func UpdateNotificationConfig(ctx context.Context, token string, partial map[string]any) (models.NotificationConfig, error) {
	body, err := json.Marshal(partial)
	if err != nil {
		return models.NotificationConfig{}, fmt.Errorf("notification: encode config: %w", err)
	}
	u := config.BackendDomain + notifBasePath + "/config"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return models.NotificationConfig{}, fmt.Errorf(errFmtBuildRequest, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return models.NotificationConfig{}, fmt.Errorf("notification: put-config: %w", err)
	}
	defer resp.Body.Close()
	if err := notifCheckStatus(resp); err != nil {
		return models.NotificationConfig{}, err
	}
	respBody, _ := io.ReadAll(resp.Body)
	var wrapped struct {
		Data models.NotificationConfig `json:"data"`
	}
	if err := json.Unmarshal(respBody, &wrapped); err == nil && wrapped.Data != (models.NotificationConfig{}) {
		return wrapped.Data, nil
	}
	var direct models.NotificationConfig
	if err := json.Unmarshal(respBody, &direct); err != nil {
		return models.NotificationConfig{}, fmt.Errorf("notification: decode config: %w", err)
	}
	return direct, nil
}

// notifCheckStatus translates HTTP status codes into typed errors.
func notifCheckStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrNotifUnauthorized
	case http.StatusNotFound:
		return ErrNotifNotFound
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("notification: backend status %d: %s", resp.StatusCode, string(body))
	}
}

// notifShortContext returns a context with a 10s timeout suitable for the
// bell-badge fast path. Callers wanting a different timeout should construct
// their own context.
func notifShortContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
