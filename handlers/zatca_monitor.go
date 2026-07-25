package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"afrita/config"
	"afrita/helpers"
	"afrita/resources"
)

// ZatcaMonitorStats holds aggregate submission statistics.
type ZatcaMonitorStats struct {
	TotalSubmitted int
	Accepted       int
	Warnings       int
	Rejected       int
	Pending        int
}

// ZatcaBranchMonitor holds per-branch ZATCA connection + submission info.
type ZatcaBranchMonitor struct {
	BranchName     string
	StatusText     string
	StatusDot      string
	StatusBg       string
	CertExpiry     string
	TodayCount     int
	SuccessRate    float64
	LastSubmission string
}

// ZatcaSubmissionRow holds one row in the recent submissions log.
type ZatcaSubmissionRow struct {
	InvoiceID   int
	InvoiceNo   string
	InvoiceType string // "standard" or "simplified"
	BranchName  string
	Status      string // "accepted", "warning", "rejected", "pending"
	ZatcaRef    string
	SubmittedAt string
	WarningMsg  string
}

type zatcaMonitorStatsResponse struct {
	TotalSubmitted int `json:"total_submitted"`
	Accepted       int `json:"accepted"`
	Warnings       int `json:"warnings"`
	Rejected       int `json:"rejected"`
	Pending        int `json:"pending"`
}

type zatcaMonitorBranchResponse struct {
	BranchID         int     `json:"branch_id"`
	BranchName       string  `json:"branch_name"`
	ZatcaStatus      int     `json:"zatca_status"`
	CertExpiry       *string `json:"cert_expiry"`
	TodayCount       int     `json:"today_count"`
	SuccessRate      float64 `json:"success_rate"`
	LastSubmissionAt *string `json:"last_submission_at"`
}

type zatcaMonitorSubmissionResponse struct {
	InvoiceID   int     `json:"invoice_id"`
	InvoiceNo   string  `json:"invoice_no"`
	BranchName  string  `json:"branch_name"`
	Status      string  `json:"status"`
	ZatcaRef    *string `json:"zatca_ref"`
	WarningMsg  *string `json:"warning_msg"`
	SubmittedAt *string `json:"submitted_at"`
}

const (
	zatcaMonitorStatsEndpoint       = "/api/v2/zatca/monitor/stats"
	zatcaMonitorBranchesEndpoint    = "/api/v2/zatca/monitor/branches"
	zatcaMonitorSubmissionsPath     = "/api/v2/zatca/monitor/submissions"
	zatcaMonitorSubmissionsEndpoint = zatcaMonitorSubmissionsPath + "?limit=50"
)

func fetchZatcaMonitorEndpoint(token, endpoint string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, config.BackendDomain+endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request for %s: %w", endpoint, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := helpers.DoAuthedRequest(req, token)
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

func branchStatusPresentation(status int) (text, dot, background string) {
	switch status {
	case 1:
		return "متصل ✓", "bg-green-500", "bg-green-100 text-green-700"
	case 2:
		return "منتهية", "bg-red-500", "bg-red-100 text-red-600"
	default:
		return "غير متصل", "bg-red-500", "bg-red-100 text-red-600"
	}
}

func mapZatcaMonitorBranches(rows []zatcaMonitorBranchResponse) []ZatcaBranchMonitor {
	branches := make([]ZatcaBranchMonitor, 0, len(rows))
	for _, row := range rows {
		statusText, statusDot, statusBackground := branchStatusPresentation(row.ZatcaStatus)
		certExpiry := "—"
		if row.CertExpiry != nil && *row.CertExpiry != "" {
			certExpiry = *row.CertExpiry
		}
		lastSubmission := "—"
		if row.LastSubmissionAt != nil && *row.LastSubmissionAt != "" {
			lastSubmission = *row.LastSubmissionAt
		}
		branches = append(branches, ZatcaBranchMonitor{
			BranchName:     row.BranchName,
			StatusText:     statusText,
			StatusDot:      statusDot,
			StatusBg:       statusBackground,
			CertExpiry:     certExpiry,
			TodayCount:     row.TodayCount,
			SuccessRate:    row.SuccessRate,
			LastSubmission: lastSubmission,
		})
	}
	return branches
}

func mapZatcaMonitorSubmissions(rows []zatcaMonitorSubmissionResponse) []ZatcaSubmissionRow {
	submissions := make([]ZatcaSubmissionRow, 0, len(rows))
	for _, row := range rows {
		zatcaRef, warningMsg, submittedAt := "", "", ""
		if row.ZatcaRef != nil {
			zatcaRef = *row.ZatcaRef
		}
		if row.WarningMsg != nil {
			warningMsg = *row.WarningMsg
		}
		if row.SubmittedAt != nil {
			submittedAt = *row.SubmittedAt
		}
		submissions = append(submissions, ZatcaSubmissionRow{
			InvoiceID:   row.InvoiceID,
			InvoiceNo:   row.InvoiceNo,
			BranchName:  row.BranchName,
			Status:      row.Status,
			ZatcaRef:    zatcaRef,
			SubmittedAt: submittedAt,
			WarningMsg:  warningMsg,
		})
	}
	return submissions
}

// HandleZatcaMonitor renders the ZATCA status monitoring page.
func HandleZatcaMonitor(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	var (
		stats          zatcaMonitorStatsResponse
		branchRows     []zatcaMonitorBranchResponse
		submissionRows []zatcaMonitorSubmissionResponse
	)
	errors := make(chan error, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		if err := fetchZatcaMonitorEndpoint(token, zatcaMonitorStatsEndpoint, &stats); err != nil {
			errors <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := fetchZatcaMonitorEndpoint(token, zatcaMonitorBranchesEndpoint, &branchRows); err != nil {
			errors <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := fetchZatcaMonitorEndpoint(token, zatcaMonitorSubmissionsEndpoint, &submissionRows); err != nil {
			errors <- err
		}
	}()

	wg.Wait()
	close(errors)

	var backendErr string
	for err := range errors {
		if backendErr == "" {
			backendErr = resources.L("zatca_monitor.load_error")
		}
		log.Printf("[ZATCA MONITOR] backend fetch failed: %v", err)
	}

	helpers.Render(w, r, "zatca-monitor", map[string]interface{}{
		"title":       "ZATCA Monitor",
		"active_page": "zatca-monitor",
		"Stats": ZatcaMonitorStats{
			TotalSubmitted: stats.TotalSubmitted,
			Accepted:       stats.Accepted,
			Warnings:       stats.Warnings,
			Rejected:       stats.Rejected,
			Pending:        stats.Pending,
		},
		"BranchStatuses": mapZatcaMonitorBranches(branchRows),
		"Submissions":    mapZatcaMonitorSubmissions(submissionRows),
		"error":          backendErr,
	})
}
