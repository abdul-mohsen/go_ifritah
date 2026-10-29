package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
	"afrita/resources"
)

const (
	zatcaSubmissionStatePending             = "pending"
	zatcaSubmissionStateAccepted            = "accepted"
	zatcaSubmissionStateAcceptedWithWarning = "accepted_with_warning"
	zatcaSubmissionStateRejected            = "rejected"
	zatcaSubmissionStateAmbiguous           = "ambiguous"
	zatcaSubmissionStateTerminalFailure     = "terminal_failure"
)

// ZatcaSubmissionOutcome is the frontend view of a durable backend submission
// row. Empty strings represent nullable provider/attempt/artifact fields.
type ZatcaSubmissionOutcome struct {
	InvoiceID           int
	State               string
	DocumentID          string
	ProviderCode        string
	ProviderWarningCode string
	ProviderWarning     string
	ProviderReference   string
	Retryable           bool
	RetryabilityKnown   bool
	RetryCount          int
	LastAttemptAt       string
	NextRetryAt         string
	RequestXMLPath      string
	ClearedXMLPath      string
	ArtifactReference   string
	OutcomeAt           string
}

// zatcaSubmissionWire accepts the T003/T004 contract plus compatibility aliases
// used by older monitor responses.
type zatcaSubmissionWire struct {
	InvoiceID           int    `json:"invoice_id"`
	State               string `json:"state"`
	Status              string `json:"status"`
	DocumentID          string `json:"document_id"`
	ProviderCode        string `json:"provider_code"`
	ProviderWarningCode string `json:"provider_warning_code"`
	ProviderWarning     string `json:"provider_warning"`
	ProviderWarningMsg  string `json:"provider_warning_message"`
	WarningMsg          string `json:"warning_msg"`
	ProviderReference   string `json:"provider_reference"`
	ZatcaRef            string `json:"zatca_ref"`
	Retryable           *bool  `json:"retryable"`
	CanRetry            *bool  `json:"can_retry"`
	RetryCount          int    `json:"retry_count"`
	LastAttemptAt       string `json:"last_attempt_at"`
	NextRetryAt         string `json:"next_retry_at"`
	RequestXMLPath      string `json:"request_xml_path"`
	ClearedXMLPath      string `json:"cleared_xml_path"`
	ArtifactReference   string `json:"artifact_reference"`
	ArtifactRef         string `json:"artifact_ref"`
	OutcomeAt           string `json:"outcome_at"`
}

// FetchZatcaSubmissionOutcomes reads the bounded monitor response and indexes
// valid durable rows by invoice ID. The monitor route is optional for older
// deployments, so callers can fall back to legacy bill state on errors.
func FetchZatcaSubmissionOutcomes(token string) (map[int]ZatcaSubmissionOutcome, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		config.BackendDomain+"/api/v2/zatca/monitor/submissions?limit=200",
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := helpers.DoAuthedRequest(req, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend ZATCA monitor status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ZATCA monitor response: %w", err)
	}

	rows, err := decodeZatcaSubmissionRows(body)
	if err != nil {
		return nil, err
	}

	outcomes := make(map[int]ZatcaSubmissionOutcome, len(rows))
	for _, row := range rows {
		state := normalizeZatcaSubmissionState(row.State, row.Status)
		if row.InvoiceID <= 0 || state == "" {
			continue
		}

		warning := firstNonEmpty(row.ProviderWarning, row.ProviderWarningMsg, row.WarningMsg)
		providerReference := firstNonEmpty(row.ProviderReference, row.ZatcaRef)
		artifactReference := firstNonEmpty(row.ArtifactReference, row.ArtifactRef)
		retryable := false
		retryabilityKnown := false
		if row.Retryable != nil {
			retryable = *row.Retryable
			retryabilityKnown = true
		} else if row.CanRetry != nil {
			retryable = *row.CanRetry
			retryabilityKnown = true
		}

		outcomes[row.InvoiceID] = ZatcaSubmissionOutcome{
			InvoiceID:           row.InvoiceID,
			State:               state,
			DocumentID:          row.DocumentID,
			ProviderCode:        row.ProviderCode,
			ProviderWarningCode: row.ProviderWarningCode,
			ProviderWarning:     warning,
			ProviderReference:   providerReference,
			Retryable:           retryable,
			RetryabilityKnown:   retryabilityKnown,
			RetryCount:          row.RetryCount,
			LastAttemptAt:       row.LastAttemptAt,
			NextRetryAt:         row.NextRetryAt,
			RequestXMLPath:      row.RequestXMLPath,
			ClearedXMLPath:      row.ClearedXMLPath,
			ArtifactReference:   artifactReference,
			OutcomeAt:           row.OutcomeAt,
		}
	}

	return outcomes, nil
}

func decodeZatcaSubmissionRows(body []byte) ([]zatcaSubmissionWire, error) {
	var rows []zatcaSubmissionWire
	if err := json.Unmarshal(body, &rows); err == nil {
		return rows, nil
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode ZATCA monitor response: %w", err)
	}

	for _, key := range []string{"data", "items", "results", "submissions", "detail"} {
		raw, ok := envelope[key]
		if !ok || string(raw) == "null" {
			continue
		}
		if err := json.Unmarshal(raw, &rows); err == nil {
			return rows, nil
		}
	}

	return nil, fmt.Errorf("unsupported ZATCA monitor response shape")
}

func normalizeZatcaSubmissionState(state, status string) string {
	for _, candidate := range []string{state, status} {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case zatcaSubmissionStatePending:
			return zatcaSubmissionStatePending
		case zatcaSubmissionStateAccepted:
			return zatcaSubmissionStateAccepted
		case zatcaSubmissionStateAcceptedWithWarning, "warning":
			return zatcaSubmissionStateAcceptedWithWarning
		case zatcaSubmissionStateRejected:
			return zatcaSubmissionStateRejected
		case zatcaSubmissionStateAmbiguous:
			return zatcaSubmissionStateAmbiguous
		case zatcaSubmissionStateTerminalFailure:
			return zatcaSubmissionStateTerminalFailure
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type invoiceSubmissionView struct {
	Status            string
	StatusClass       string
	State             string
	IsIssued          bool
	HasOutcome        bool
	ProviderCode      string
	WarningCode       string
	Warning           string
	ProviderReference string
	Retryable         bool
	RetryabilityKnown bool
	RetryCount        int
	NextRetryAt       string
	RequestXMLPath    string
	ClearedXMLPath    string
	ArtifactReference string
	OutcomeAt         string
}

func resolveInvoiceSubmissionView(inv models.Invoice, outcome *ZatcaSubmissionOutcome) invoiceSubmissionView {
	status, statusClass := helpers.InvoiceStatus(inv)
	view := invoiceSubmissionView{
		Status:      helpers.TranslateInvoiceStatus(status),
		StatusClass: statusClass,
		IsIssued:    inv.State == 2 || inv.State == 3,
	}

	// T003 currently keys durable submissions to bill_id, so a credit-note row
	// must retain its established credit-state label until a document-kind
	// extension exists.
	if inv.CreditState >= 1 || outcome == nil || normalizeZatcaSubmissionState(outcome.State, "") == "" {
		return view
	}

	view.State = outcome.State
	view.HasOutcome = true
	view.IsIssued = outcome.State == zatcaSubmissionStateAccepted ||
		outcome.State == zatcaSubmissionStateAcceptedWithWarning
	view.Status, view.StatusClass = zatcaSubmissionStatusLabel(outcome.State)
	view.ProviderCode = outcome.ProviderCode
	view.WarningCode = outcome.ProviderWarningCode
	view.Warning = outcome.ProviderWarning
	view.ProviderReference = outcome.ProviderReference
	view.Retryable = outcome.Retryable
	view.RetryabilityKnown = outcome.RetryabilityKnown
	view.RetryCount = outcome.RetryCount
	view.NextRetryAt = outcome.NextRetryAt
	view.RequestXMLPath = outcome.RequestXMLPath
	view.ClearedXMLPath = outcome.ClearedXMLPath
	view.ArtifactReference = outcome.ArtifactReference
	view.OutcomeAt = outcome.OutcomeAt
	return view
}

func zatcaSubmissionStatusLabel(state string) (string, string) {
	switch state {
	case zatcaSubmissionStatePending:
		return resources.L("invoice_status.zatca_pending"), "badge-pending"
	case zatcaSubmissionStateAccepted:
		return resources.L("tpl.invoice.status_issued"), "badge-issued"
	case zatcaSubmissionStateAcceptedWithWarning:
		return resources.L("invoice_status.zatca_accepted_with_warning"), "badge-warning"
	case zatcaSubmissionStateRejected:
		return resources.L("invoice_status.zatca_rejected"), "badge-danger"
	case zatcaSubmissionStateAmbiguous:
		return resources.L("invoice_status.zatca_ambiguous"), "badge-warning"
	case zatcaSubmissionStateTerminalFailure:
		return resources.L("invoice_status.zatca_terminal_failure"), "badge-danger"
	default:
		return "", ""
	}
}

func logZatcaSubmissionFallback(err error) {
	if err != nil {
		log.Printf("[invoices] durable ZATCA outcome unavailable: %v", err)
	}
}
