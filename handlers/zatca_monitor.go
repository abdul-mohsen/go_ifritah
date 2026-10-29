package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"afrita/config"
	"afrita/helpers"
	"afrita/resources"
)

const (
	zatcaMonitorStatsEndpoint       = "/api/v2/zatca/monitor/stats"
	zatcaMonitorBranchesEndpoint    = "/api/v2/zatca/monitor/branches"
	zatcaMonitorSubmissionsEndpoint = "/api/v2/zatca/monitor/submissions?limit=50"
	zatcaMonitorUnavailable         = "—"
)

// ZatcaMonitorStats holds aggregate submission statistics.
type ZatcaMonitorStats struct {
	TotalSubmitted      int64
	Accepted            int64
	AcceptedWithWarning int64
	Warnings            int64
	Rejected            int64
	Ambiguous           int64
	TerminalFailure     int64
	Pending             int64
}

// ZatcaBranchMonitor holds per-branch ZATCA connection + submission info.
type ZatcaBranchMonitor struct {
	BranchID       int64
	BranchName     string
	StatusText     string
	StatusLabelKey string
	StatusDot      string
	StatusBg       string
	CertExpiry     string
	TodayCount     int64
	SuccessRate    float64
	LastSubmission string
}

// ZatcaSubmissionRow holds one row in the recent submissions log.
type ZatcaSubmissionRow struct {
	InvoiceID           int
	InvoiceNo           string
	InvoiceType         string
	BranchName          string
	Status              string
	State               string
	FilterStatus        string
	StatusLabelKey      string
	StatusBg            string
	ZatcaRef            string
	ProviderReference   string
	ProviderCode        string
	ProviderWarningCode string
	WarningMsg          string
	SubmittedAt         string
	LastAttemptAt       string
	PendingAge          string
	DocumentID          string
}

type zatcaMonitorStatsResponse struct {
	TotalSubmitted      int64 `json:"total_submitted"`
	Accepted            int64 `json:"accepted"`
	AcceptedWithWarning int64 `json:"accepted_with_warning"`
	Warnings            int64 `json:"warnings"`
	Rejected            int64 `json:"rejected"`
	Ambiguous           int64 `json:"ambiguous"`
	TerminalFailure     int64 `json:"terminal_failure"`
	Pending             int64 `json:"pending"`
}

type zatcaMonitorBranchResponse struct {
	BranchID         int64   `json:"branch_id"`
	BranchName       string  `json:"branch_name"`
	ZatcaStatus      int     `json:"zatca_status"`
	CertExpiry       *string `json:"cert_expiry"`
	TodayCount       int64   `json:"today_count"`
	SuccessRate      float64 `json:"success_rate"`
	LastSubmissionAt *string `json:"last_submission_at"`
}

type zatcaMonitorSubmissionResponse struct {
	InvoiceID           int64   `json:"invoice_id"`
	InvoiceNo           string  `json:"invoice_no"`
	InvoiceType         string  `json:"invoice_type"`
	BranchName          string  `json:"branch_name"`
	Status              string  `json:"status"`
	State               string  `json:"state"`
	StatusCode          int     `json:"status_code"`
	DocumentID          string  `json:"document_id"`
	ProviderCode        *string `json:"provider_code"`
	ProviderWarningCode *string `json:"provider_warning_code"`
	ProviderWarning     *string `json:"provider_warning"`
	ProviderReference   *string `json:"provider_reference"`
	ZatcaRef            *string `json:"zatca_ref"`
	WarningMsg          *string `json:"warning_msg"`
	LastAttemptAt       *string `json:"last_attempt_at"`
	SubmittedAt         *string `json:"submitted_at"`
}

func fetchZatcaMonitorEndpoint(sessionID, endpoint string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, config.BackendDomain+endpoint, nil)
	if err != nil {
		return fmt.Errorf("create monitor request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := helpers.DoAuthedRequestWithRetry(req, sessionID)
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s response: %w", endpoint, err)
	}
	return nil
}

func branchStatusPresentation(status int) (labelKey, dot, background string) {
	switch status {
	case 1:
		return "zatca_monitor.branch_status_active", "bg-green-500", "bg-green-100 text-green-700"
	case 2:
		return "zatca_monitor.branch_status_expired", "bg-red-500", "bg-red-100 text-red-600"
	case 4:
		return "zatca_monitor.branch_status_expiring_soon", "bg-orange-500", "bg-orange-100 text-orange-700"
	case 0, 3:
		return "zatca_monitor.branch_status_not_active", "bg-gray-500", "bg-gray-100 text-gray-700"
	default:
		return "zatca_monitor.branch_status_unknown", "bg-gray-500", "bg-gray-100 text-gray-700"
	}
}

func mapZatcaMonitorBranches(rows []zatcaMonitorBranchResponse) []ZatcaBranchMonitor {
	branches := make([]ZatcaBranchMonitor, 0, len(rows))
	for _, row := range rows {
		labelKey, dot, background := branchStatusPresentation(row.ZatcaStatus)
		branches = append(branches, ZatcaBranchMonitor{
			BranchID:       row.BranchID,
			BranchName:     monitorDisplayString(row.BranchName),
			StatusText:     resources.L(labelKey),
			StatusLabelKey: labelKey,
			StatusDot:      dot,
			StatusBg:       background,
			CertExpiry:     monitorDisplayPointer(row.CertExpiry),
			TodayCount:     row.TodayCount,
			SuccessRate:    row.SuccessRate,
			LastSubmission: monitorDisplayPointer(row.LastSubmissionAt),
		})
	}
	return branches
}

func mapZatcaMonitorSubmissions(rows []zatcaMonitorSubmissionResponse) []ZatcaSubmissionRow {
	submissions := make([]ZatcaSubmissionRow, 0, len(rows))
	for _, row := range rows {
		state := normalizeZatcaMonitorState(row.State, row.Status, row.StatusCode)
		labelKey, background, filterStatus := submissionStatusPresentation(state)
		submittedAt := monitorDisplayPointer(row.SubmittedAt)
		lastAttemptAt := monitorDisplayPointer(row.LastAttemptAt)
		pendingAge := zatcaMonitorUnavailable
		if state == "pending" {
			pendingAge = pendingAgeFrom(row.SubmittedAt, row.LastAttemptAt)
		}

		submissions = append(submissions, ZatcaSubmissionRow{
			InvoiceID:           int(row.InvoiceID),
			InvoiceNo:           monitorDisplayString(row.InvoiceNo),
			InvoiceType:         strings.TrimSpace(row.InvoiceType),
			BranchName:          monitorDisplayString(row.BranchName),
			Status:              state,
			State:               state,
			FilterStatus:        filterStatus,
			StatusLabelKey:      labelKey,
			StatusBg:            background,
			ZatcaRef:            monitorPointerValue(row.ZatcaRef),
			ProviderReference:   monitorPointerValue(row.ProviderReference),
			ProviderCode:        monitorPointerValue(row.ProviderCode),
			ProviderWarningCode: monitorPointerValue(row.ProviderWarningCode),
			WarningMsg:          firstMonitorPointerValue(row.ProviderWarning, row.WarningMsg),
			SubmittedAt:         submittedAt,
			LastAttemptAt:       lastAttemptAt,
			PendingAge:          pendingAge,
			DocumentID:          strings.TrimSpace(row.DocumentID),
		})
	}
	return submissions
}

func mapZatcaMonitorStats(response zatcaMonitorStatsResponse) ZatcaMonitorStats {
	warnings := response.Warnings
	if response.AcceptedWithWarning > 0 {
		warnings = response.AcceptedWithWarning
	}
	return ZatcaMonitorStats{
		TotalSubmitted:      response.TotalSubmitted,
		Accepted:            response.Accepted,
		AcceptedWithWarning: response.AcceptedWithWarning,
		Warnings:            warnings,
		Rejected:            response.Rejected,
		Ambiguous:           response.Ambiguous,
		TerminalFailure:     response.TerminalFailure,
		Pending:             response.Pending,
	}
}

func normalizeZatcaMonitorState(state, status string, statusCode int) string {
	for _, candidate := range []string{state, status} {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "pending":
			return "pending"
		case "accepted":
			return "accepted"
		case "accepted_with_warning", "warning":
			return "accepted_with_warning"
		case "rejected":
			return "rejected"
		case "ambiguous":
			return "ambiguous"
		case "terminal_failure", "terminal-failure":
			return "terminal_failure"
		}
	}

	switch statusCode {
	case 0, 1:
		return "pending"
	case 2:
		return "accepted"
	case 3:
		return "rejected"
	case 4:
		return "accepted_with_warning"
	default:
		return "unknown"
	}
}

func submissionStatusPresentation(state string) (labelKey, background, filterStatus string) {
	switch state {
	case "accepted":
		return "zatca_monitor.status_accepted", "bg-green-100 text-green-700", "accepted"
	case "accepted_with_warning":
		return "zatca_monitor.status_warning", "bg-amber-100 text-amber-700", "warning"
	case "rejected":
		return "zatca_monitor.status_rejected", "bg-red-100 text-red-700", "rejected"
	case "pending":
		return "zatca_monitor.status_pending", "bg-blue-100 text-blue-700", "pending"
	case "ambiguous":
		return "zatca_monitor.status_ambiguous", "bg-purple-100 text-purple-700", "ambiguous"
	case "terminal_failure":
		return "zatca_monitor.status_terminal_failure", "bg-red-100 text-red-700", "terminal_failure"
	default:
		return "zatca_monitor.status_unknown", "bg-gray-100 text-gray-700", "unknown"
	}
}

func monitorDisplayString(value string) string {
	if strings.TrimSpace(value) == "" {
		return zatcaMonitorUnavailable
	}
	return value
}

func monitorPointerValue(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return ""
	}
	return *value
}

func firstMonitorPointerValue(values ...*string) string {
	for _, value := range values {
		if result := monitorPointerValue(value); result != "" {
			return result
		}
	}
	return ""
}

func monitorDisplayPointer(value *string) string {
	if result := monitorPointerValue(value); result != "" {
		return result
	}
	return zatcaMonitorUnavailable
}

func parseZatcaMonitorTime(values ...*string) (time.Time, bool) {
	for _, value := range values {
		raw := monitorPointerValue(value)
		if raw == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, raw); err == nil {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}

func pendingAgeFrom(values ...*string) string {
	startedAt, ok := parseZatcaMonitorTime(values...)
	if !ok {
		return zatcaMonitorUnavailable
	}
	age := time.Since(startedAt)
	if age < 0 {
		return zatcaMonitorUnavailable
	}

	days := int(age / (24 * time.Hour))
	age -= time.Duration(days) * 24 * time.Hour
	hours := int(age / time.Hour)
	age -= time.Duration(hours) * time.Hour
	minutes := int(age / time.Minute)
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return "< 1m"
}

// HandleZatcaMonitor renders the ZATCA status monitoring page.
func HandleZatcaMonitor(w http.ResponseWriter, r *http.Request) {
	_, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	sessionID := helpers.GetSessionIDFromRequest(r)

	var (
		statsResponse  zatcaMonitorStatsResponse
		branchRows     []zatcaMonitorBranchResponse
		submissionRows []zatcaMonitorSubmissionResponse
		statsErr       error
		branchesErr    error
		submissionsErr error
	)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		statsErr = fetchZatcaMonitorEndpoint(sessionID, zatcaMonitorStatsEndpoint, &statsResponse)
	}()
	go func() {
		defer wg.Done()
		branchesErr = fetchZatcaMonitorEndpoint(sessionID, zatcaMonitorBranchesEndpoint, &branchRows)
	}()
	go func() {
		defer wg.Done()
		submissionsErr = fetchZatcaMonitorEndpoint(sessionID, zatcaMonitorSubmissionsEndpoint, &submissionRows)
	}()
	wg.Wait()

	for endpoint, err := range map[string]error{
		zatcaMonitorStatsEndpoint:       statsErr,
		zatcaMonitorBranchesEndpoint:    branchesErr,
		zatcaMonitorSubmissionsEndpoint: submissionsErr,
	} {
		if err != nil {
			log.Printf("[ZATCA MONITOR] %s failed: %v", endpoint, err)
		}
	}

	hasError := statsErr != nil || branchesErr != nil || submissionsErr != nil
	helpers.Render(w, r, "zatca-monitor", map[string]interface{}{
		"title":            "ZATCA Monitor",
		"active_page":      "zatca-monitor",
		"Stats":            mapZatcaMonitorStats(statsResponse),
		"StatsReady":       statsErr == nil,
		"StatsError":       statsErr != nil,
		"BranchStatuses":   mapZatcaMonitorBranches(branchRows),
		"BranchesReady":    branchesErr == nil,
		"BranchesError":    branchesErr != nil,
		"Submissions":      mapZatcaMonitorSubmissions(submissionRows),
		"SubmissionsReady": submissionsErr == nil,
		"SubmissionsError": submissionsErr != nil,
		"MonitorLoading":   false,
		"error": func() string {
			if hasError {
				return resources.L("zatca_monitor.load_error")
			}
			return ""
		}(),
	})
}
