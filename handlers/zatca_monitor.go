package handlers

import (
	"log"
	"net/http"
	"strconv"

	"afrita/helpers"
	"afrita/models"
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
	InvoiceType string
	BranchName  string
	Status      string
	StatusClass string
	ZatcaRef    string
	SubmittedAt string
	WarningMsg  string
	BillState   models.BillStateProjection
}

// HandleZatcaMonitor renders the durable ZATCA status monitoring page.
func HandleZatcaMonitor(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	stats, statsErr := helpers.FetchZatcaMonitorStats(token)
	branches, branchesErr := helpers.FetchZatcaMonitorBranches(token)
	submissions, submissionsErr := helpers.FetchZatcaMonitorSubmissions(token)
	for _, err := range []error{statsErr, branchesErr, submissionsErr} {
		if err != nil && helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
	}

	var monitorErrors []string
	if statsErr != nil {
		log.Printf("[zatca-monitor] stats fetch failed: %v", statsErr)
		monitorErrors = append(monitorErrors, "إحصاءات الإرسال")
	}
	if branchesErr != nil {
		log.Printf("[zatca-monitor] branches fetch failed: %v", branchesErr)
		monitorErrors = append(monitorErrors, "حالة الفروع")
	}
	if submissionsErr != nil {
		log.Printf("[zatca-monitor] submissions fetch failed: %v", submissionsErr)
		monitorErrors = append(monitorErrors, "سجل الإرسال")
	}

	branchStatuses := make([]ZatcaBranchMonitor, 0, len(branches))
	if branchesErr == nil {
		for _, branch := range branches {
			statusText, statusDot, statusBg := branchStatusPresentation(branch.ZatcaStatus)
			certExpiry := "—"
			if branch.CertExpiry != nil && *branch.CertExpiry != "" {
				certExpiry = *branch.CertExpiry
			}
			lastSubmission := "—"
			if branch.LastSubmissionAt != nil && *branch.LastSubmissionAt != "" {
				lastSubmission = *branch.LastSubmissionAt
			}
			branchStatuses = append(branchStatuses, ZatcaBranchMonitor{
				BranchName:     branch.BranchName,
				StatusText:     statusText,
				StatusDot:      statusDot,
				StatusBg:       statusBg,
				CertExpiry:     certExpiry,
				TodayCount:     branch.TodayCount,
				SuccessRate:    branch.SuccessRate,
				LastSubmission: lastSubmission,
			})
		}
	}

	submissionRows := make([]ZatcaSubmissionRow, 0, len(submissions))
	if submissionsErr == nil {
		for _, submission := range submissions {
			billState := helpers.ProjectZatcaMonitorState(submission.Status, submission.StatusCode)
			status := monitorStatusKey(billState.ZatcaState)
			invoiceNo := strconv.Itoa(submission.SourceID)
			if submission.DocumentNumber != nil && *submission.DocumentNumber != "" {
				invoiceNo = *submission.DocumentNumber
			}
			zatcaRef := ""
			if submission.ZatcaRef != nil {
				zatcaRef = *submission.ZatcaRef
			}
			warningMsg := ""
			if submission.WarningMsg != nil {
				warningMsg = *submission.WarningMsg
			}
			submittedAt := ""
			if submission.SubmittedAt != nil {
				submittedAt = *submission.SubmittedAt
			}
			submissionRows = append(submissionRows, ZatcaSubmissionRow{
				InvoiceID:   submission.SourceID,
				InvoiceNo:   invoiceNo,
				InvoiceType: monitorDocumentType(submission.DocumentType),
				BranchName:  submission.BranchName,
				Status:      status,
				StatusClass: billState.ZatcaClass,
				ZatcaRef:    zatcaRef,
				SubmittedAt: submittedAt,
				WarningMsg:  warningMsg,
				BillState:   billState,
			})
		}
	}

	helpers.Render(w, r, "zatca-monitor", map[string]interface{}{
		"title":          "ZATCA Monitor",
		"active_page":    "zatca-monitor",
		"Stats":          ZatcaMonitorStats{TotalSubmitted: stats.TotalSubmitted, Accepted: stats.Accepted, Warnings: stats.Warnings, Rejected: stats.Rejected, Pending: stats.Pending},
		"BranchStatuses": branchStatuses,
		"Submissions":    submissionRows,
		"monitor_errors": monitorErrors,
	})
}

func branchStatusPresentation(status int) (string, string, string) {
	switch status {
	case 1:
		return "متصل ✓", "bg-green-500", "bg-green-100 text-green-700"
	case 2:
		return "منتهي", "bg-red-500", "bg-red-100 text-red-600"
	case 4:
		return "ينتهي قريباً ⏰", "bg-orange-500", "bg-orange-100 text-orange-700"
	default:
		return "غير متصل", "bg-gray-500", "bg-gray-100 text-gray-700"
	}
}

func monitorStatusKey(state string) string {
	switch state {
	case "accepted":
		return "accepted"
	case "accepted_with_warning":
		return "warning"
	case "rejected":
		return "rejected"
	case "pending", "retrying":
		return "pending"
	case "result_unknown":
		return "result_unknown"
	case "action_required":
		return "action_required"
	default:
		return "unknown"
	}
}

func monitorDocumentType(documentType string) string {
	switch documentType {
	case "bill", "invoice":
		return "standard"
	case "credit":
		return "credit"
	case "debit":
		return "debit"
	default:
		return documentType
	}
}
