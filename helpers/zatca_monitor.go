package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"afrita/config"
)

// ZatcaMonitorAPIStats mirrors the clean backend monitor stats response.
type ZatcaMonitorAPIStats struct {
	TotalSubmitted int `json:"total_submitted"`
	Accepted       int `json:"accepted"`
	Warnings       int `json:"warnings"`
	Rejected       int `json:"rejected"`
	Pending        int `json:"pending"`
}

// ZatcaMonitorAPIBranch mirrors the clean backend branch monitor response.
type ZatcaMonitorAPIBranch struct {
	BranchID         int     `json:"branch_id"`
	BranchName       string  `json:"branch_name"`
	ZatcaStatus      int     `json:"zatca_status"`
	CertExpiry       *string `json:"cert_expiry"`
	TodayCount       int     `json:"today_count"`
	SuccessRate      float64 `json:"success_rate"`
	LastSubmissionAt *string `json:"last_submission_at"`
}

// ZatcaMonitorAPISubmission mirrors the clean backend submission monitor
// response. Status is authoritative when present; StatusCode is only a
// compatibility fallback for older monitor rows.
type ZatcaMonitorAPISubmission struct {
	TenantID             int64   `json:"tenant_id"`
	DocumentType         string  `json:"document_type"`
	SourceID             int     `json:"source_id"`
	PosID                string  `json:"pos_id"`
	RequestID            string  `json:"request_id"`
	InvoiceUUID          string  `json:"invoice_uuid"`
	OriginalDocumentType *string `json:"original_document_type"`
	OriginalSourceID     *int64  `json:"original_source_id"`
	OriginalInvoiceUUID  *string `json:"original_invoice_uuid"`
	DocumentNumber       *string `json:"document_number"`
	BranchID             *int64  `json:"branch_id"`
	BranchName           string  `json:"branch_name"`
	Status               string  `json:"status"`
	StatusCode           int     `json:"status_code"`
	ZatcaRef             *string `json:"zatca_ref"`
	WarningMsg           *string `json:"warning_msg"`
	SubmittedAt          *string `json:"submitted_at"`
}

func FetchZatcaMonitorStats(token string) (ZatcaMonitorAPIStats, error) {
	var out ZatcaMonitorAPIStats
	err := fetchZatcaMonitorJSON(token, "/api/v2/zatca/monitor/stats", &out)
	return out, err
}

func FetchZatcaMonitorBranches(token string) ([]ZatcaMonitorAPIBranch, error) {
	var out []ZatcaMonitorAPIBranch
	err := fetchZatcaMonitorJSON(token, "/api/v2/zatca/monitor/branches", &out)
	return out, err
}

func FetchZatcaMonitorSubmissions(token string) ([]ZatcaMonitorAPISubmission, error) {
	var out []ZatcaMonitorAPISubmission
	err := fetchZatcaMonitorJSON(token, "/api/v2/zatca/monitor/submissions?limit=50", &out)
	return out, err
}

func fetchZatcaMonitorJSON(token, path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, config.BackendDomain+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := DoAuthedRequest(req, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("backend status 401")
		}
		return fmt.Errorf("backend status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
