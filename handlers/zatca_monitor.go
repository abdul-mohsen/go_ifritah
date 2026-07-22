package handlers

import (
	"net/http"

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

// HandleZatcaMonitor renders the ZATCA status monitoring page.
// Currently returns mock data — will call backend APIs when ready.
func HandleZatcaMonitor(w http.ResponseWriter, r *http.Request) {
	_, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	lang := helpers.GetLang(r)

	// TODO: Replace mock data with real API calls:
	//   GET /api/v2/zatca/monitor/stats
	//   GET /api/v2/zatca/monitor/branches
	//   GET /api/v2/zatca/monitor/submissions?limit=50

	stats := ZatcaMonitorStats{
		TotalSubmitted: 1247,
		Accepted:       1189,
		Warnings:       38,
		Rejected:       12,
		Pending:        8,
	}

	branchStatuses := []ZatcaBranchMonitor{
		{
			BranchName:     resources.T(lang, "branch.main_branch"),
			StatusText:     resources.T(lang, "tpl.settings.zatca_status_active") + " ✓",
			StatusDot:      "bg-green-500",
			StatusBg:       "bg-green-100 text-green-700",
			CertExpiry:     "2026-09-15",
			TodayCount:     23,
			SuccessRate:    97.8,
			LastSubmission: resources.T(lang, "zatca_monitor.five_minutes_ago"),
		},
		{
			BranchName:     resources.T(lang, "branch.jeddah_branch"),
			StatusText:     resources.T(lang, "tpl.settings.zatca_status_active") + " ✓",
			StatusDot:      "bg-green-500",
			StatusBg:       "bg-green-100 text-green-700",
			CertExpiry:     "2026-08-22",
			TodayCount:     15,
			SuccessRate:    100.0,
			LastSubmission: resources.T(lang, "zatca_monitor.twelve_minutes_ago"),
		},
		{
			BranchName:     resources.T(lang, "branch.dammam_branch"),
			StatusText:     resources.T(lang, "tpl.settings.zatca_status_expiring_soon") + " ⏰",
			StatusDot:      "bg-orange-500",
			StatusBg:       "bg-orange-100 text-orange-700",
			CertExpiry:     "2026-05-01",
			TodayCount:     8,
			SuccessRate:    87.5,
			LastSubmission: resources.T(lang, "zatca_monitor.thirty_minutes_ago"),
		},
		{
			BranchName:     resources.T(lang, "branch.madinah_branch"),
			StatusText:     resources.T(lang, "tpl.settings.zatca_status_inactive"),
			StatusDot:      "bg-red-500",
			StatusBg:       "bg-red-100 text-red-600",
			CertExpiry:     "—",
			TodayCount:     0,
			SuccessRate:    0,
			LastSubmission: "—",
		},
	}

	submissions := []ZatcaSubmissionRow{
		{InvoiceID: 4521, InvoiceNo: "INV-4521", InvoiceType: "simplified", BranchName: resources.T(lang, "branch.main_branch"), Status: "accepted", ZatcaRef: "ZAT-2026-04150001", SubmittedAt: "2026-04-15 09:32:14"},
		{InvoiceID: 4520, InvoiceNo: "INV-4520", InvoiceType: "standard", BranchName: resources.T(lang, "branch.jeddah_branch"), Status: "accepted", ZatcaRef: "ZAT-2026-04150002", SubmittedAt: "2026-04-15 09:28:41"},
		{InvoiceID: 4519, InvoiceNo: "INV-4519", InvoiceType: "simplified", BranchName: resources.T(lang, "branch.main_branch"), Status: "warning", ZatcaRef: "ZAT-2026-04150003", SubmittedAt: "2026-04-15 09:15:07", WarningMsg: resources.T(lang, "zatca_monitor.warning_incomplete_address")},
		{InvoiceID: 4518, InvoiceNo: "INV-4518", InvoiceType: "simplified", BranchName: resources.T(lang, "branch.dammam_branch"), Status: "rejected", ZatcaRef: "", SubmittedAt: "2026-04-15 09:10:33"},
		{InvoiceID: 4517, InvoiceNo: "INV-4517", InvoiceType: "standard", BranchName: resources.T(lang, "branch.main_branch"), Status: "accepted", ZatcaRef: "ZAT-2026-04150004", SubmittedAt: "2026-04-15 08:55:19"},
		{InvoiceID: 4516, InvoiceNo: "INV-4516", InvoiceType: "simplified", BranchName: resources.T(lang, "branch.jeddah_branch"), Status: "accepted", ZatcaRef: "ZAT-2026-04150005", SubmittedAt: "2026-04-15 08:42:08"},
		{InvoiceID: 4515, InvoiceNo: "INV-4515", InvoiceType: "simplified", BranchName: resources.T(lang, "branch.main_branch"), Status: "pending", ZatcaRef: "", SubmittedAt: "2026-04-15 08:38:55"},
		{InvoiceID: 4514, InvoiceNo: "INV-4514", InvoiceType: "standard", BranchName: resources.T(lang, "branch.dammam_branch"), Status: "warning", ZatcaRef: "ZAT-2026-04150006", SubmittedAt: "2026-04-15 08:30:11", WarningMsg: resources.T(lang, "zatca_monitor.warning_duplicate_reference")},
		{InvoiceID: 4513, InvoiceNo: "INV-4513", InvoiceType: "simplified", BranchName: resources.T(lang, "branch.main_branch"), Status: "accepted", ZatcaRef: "ZAT-2026-04150007", SubmittedAt: "2026-04-15 08:22:44"},
		{InvoiceID: 4512, InvoiceNo: "INV-4512", InvoiceType: "simplified", BranchName: resources.T(lang, "branch.jeddah_branch"), Status: "accepted", ZatcaRef: "ZAT-2026-04150008", SubmittedAt: "2026-04-15 08:15:02"},
	}

	helpers.Render(w, r, "zatca-monitor", map[string]interface{}{
		"title":          resources.T(lang, "zatca_monitor.title"),
		"active_page":    "zatca-monitor",
		"Stats":          stats,
		"BranchStatuses": branchStatuses,
		"Submissions":    submissions,
	})
}
