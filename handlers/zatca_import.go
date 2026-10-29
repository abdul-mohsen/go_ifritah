package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"afrita/config"
	"afrita/helpers"
	"afrita/resources"

	"github.com/gorilla/mux"
)

const maxZatcaImportProxyRequestBytes = 8 << 20

// ZatcaImportView is the frontend's read-only representation of the T031
// import lane. It deliberately keeps import status separate from invoice
// status and never maps an import to an outbound issued state.
type ZatcaImportView struct {
	ID                uint64                           `json:"id"`
	BranchID          *uint32                          `json:"branch_id,omitempty"`
	Status            string                           `json:"status"`
	DisplayStatus     string                           `json:"-"`
	StatusClass       string                           `json:"-"`
	WorkflowVersion   uint64                           `json:"workflow_version"`
	SourceContentHash string                           `json:"source_content_hash"`
	CreatedAt         string                           `json:"created_at"`
	UpdatedAt         string                           `json:"updated_at"`
	Evidence          *ZatcaImportEvidenceView         `json:"evidence,omitempty"`
	ValidationErrors  []ZatcaImportValidationErrorView `json:"validation_errors,omitempty"`
	Reviews           []ZatcaImportReviewView          `json:"reviews,omitempty"`
	StatusHistory     []ZatcaImportHistoryView         `json:"status_history,omitempty"`
	CanApprove        bool                             `json:"-"`
	CanRequestChanges bool                             `json:"-"`
	CanPost           bool                             `json:"-"`
	CanReject         bool                             `json:"-"`
}

type ZatcaImportEvidenceView struct {
	SourceReference    string          `json:"source_reference"`
	SourceOriginalName *string         `json:"source_original_name,omitempty"`
	SourceMIMEType     string          `json:"source_mime_type"`
	SourceProvenance   string          `json:"source_provenance"`
	SourceContentHash  string          `json:"source_content_hash"`
	SourceSize         uint64          `json:"source_size"`
	CanonicalData      json.RawMessage `json:"canonical_data,omitempty"`
	ParsedData         json.RawMessage `json:"parsed_data,omitempty"`
	SignatureData      json.RawMessage `json:"signature_data,omitempty"`
	ValidationData     json.RawMessage `json:"validation_data,omitempty"`
	FailureClass       *string         `json:"failure_class,omitempty"`
	FailureMessage     *string         `json:"failure_message,omitempty"`
}

type ZatcaImportValidationErrorView struct {
	Severity  string          `json:"severity"`
	ErrorCode string          `json:"error_code"`
	FieldKey  *string         `json:"field_key,omitempty"`
	Message   string          `json:"message"`
	Details   json.RawMessage `json:"details,omitempty"`
	CreatedAt string          `json:"created_at"`
}

type ZatcaImportReviewView struct {
	Decision                string  `json:"decision"`
	Reason                  *string `json:"reason,omitempty"`
	ExpectedWorkflowVersion *uint64 `json:"expected_workflow_version,omitempty"`
	ObservedWorkflowVersion uint64  `json:"observed_workflow_version"`
	CreatedAt               string  `json:"created_at"`
}

type ZatcaImportHistoryView struct {
	FromStatus      *string `json:"from_status,omitempty"`
	ToStatus        string  `json:"to_status"`
	TransitionKey   string  `json:"transition_key"`
	WorkflowVersion uint64  `json:"workflow_version"`
	Reason          *string `json:"reason,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

// HandleZatcaImports renders the authenticated signed-UBL import list and
// upload page.
func HandleZatcaImports(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	imports, err := fetchZatcaImportList(r)
	backendError := ""
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		log.Printf("[zatca-import] list fetch failed: %v", err)
		backendError = resources.L("zatca_import.load_failed")
		imports = []ZatcaImportView{}
	}

	helpers.Render(w, r, "zatca-imports", map[string]interface{}{
		"title":         resources.L("zatca_import.title"),
		"active_page":   "zatca-imports",
		"Imports":       imports,
		"status_filter": strings.TrimSpace(r.URL.Query().Get("status")),
		"error":         backendError,
	})
}

// HandleZatcaImportDetail renders the authenticated evidence/review page.
func HandleZatcaImportDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := helpers.GetTokenOrRedirect(w, r); !ok {
		return
	}

	id, err := parseZatcaImportRouteID(r)
	if err != nil {
		RenderErrorPage(w, "400", http.StatusBadRequest, resources.L("zatca_import.invalid_id"), resources.L("zatca_import.invalid_id"))
		return
	}

	importView, err := fetchZatcaImportDetail(r, id)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			helpers.HandleUnauthorized(w, r)
			return
		}
		log.Printf("[zatca-import] detail fetch failed for %d: %v", id, err)
		RenderErrorPage(w, "502", http.StatusBadGateway, resources.L("zatca_import.load_failed"), resources.L("zatca_import.load_failed"))
		return
	}

	helpers.Render(w, r, "zatca-import-detail", map[string]interface{}{
		"title":       fmt.Sprintf("%s #%d", resources.L("zatca_import.detail_title"), importView.ID),
		"active_page": "zatca-imports",
		"Import":      importView,
	})
}

// HandleZatcaImportCreate proxies a signed-UBL import upload to T031.
func HandleZatcaImportCreate(w http.ResponseWriter, r *http.Request) {
	proxyZatcaImportRequest(w, r, "")
}

// HandleZatcaImportList proxies a tenant-scoped import list to T031.
func HandleZatcaImportList(w http.ResponseWriter, r *http.Request) {
	proxyZatcaImportRequest(w, r, "")
}

// HandleZatcaImportDetailAPI proxies a tenant-scoped import detail to T031.
func HandleZatcaImportDetailAPI(w http.ResponseWriter, r *http.Request) {
	id, err := parseZatcaImportRouteID(r)
	if err != nil {
		writeZatcaImportJSONError(w, http.StatusBadRequest, resources.L("zatca_import.invalid_id"))
		return
	}
	proxyZatcaImportRequest(w, r, fmt.Sprintf("/%d", id))
}

// HandleZatcaImportReview proxies an append-only review decision to T031.
func HandleZatcaImportReview(w http.ResponseWriter, r *http.Request) {
	id, err := parseZatcaImportRouteID(r)
	if err != nil {
		writeZatcaImportJSONError(w, http.StatusBadRequest, resources.L("zatca_import.invalid_id"))
		return
	}
	proxyZatcaImportRequest(w, r, fmt.Sprintf("/%d/review", id))
}

// HandleZatcaImportPost proxies import-lane posting to T031.
func HandleZatcaImportPost(w http.ResponseWriter, r *http.Request) {
	id, err := parseZatcaImportRouteID(r)
	if err != nil {
		writeZatcaImportJSONError(w, http.StatusBadRequest, resources.L("zatca_import.invalid_id"))
		return
	}
	proxyZatcaImportRequest(w, r, fmt.Sprintf("/%d/post", id))
}

// HandleZatcaImportReject proxies a required-reason rejection to T031.
func HandleZatcaImportReject(w http.ResponseWriter, r *http.Request) {
	id, err := parseZatcaImportRouteID(r)
	if err != nil {
		writeZatcaImportJSONError(w, http.StatusBadRequest, resources.L("zatca_import.invalid_id"))
		return
	}
	proxyZatcaImportRequest(w, r, fmt.Sprintf("/%d/reject", id))
}

// Compatibility aliases keep handler names discoverable for callers that use
// singular page/action terminology.
func HandleZatcaImportPage(w http.ResponseWriter, r *http.Request) {
	HandleZatcaImports(w, r)
}

func HandleZatcaImportDetailPage(w http.ResponseWriter, r *http.Request) {
	HandleZatcaImportDetail(w, r)
}

func HandleZatcaImportUpload(w http.ResponseWriter, r *http.Request) {
	HandleZatcaImportCreate(w, r)
}

func fetchZatcaImportList(r *http.Request) ([]ZatcaImportView, error) {
	path := "/api/v2/zatca/import"
	if query := r.URL.RawQuery; query != "" {
		path += "?" + query
	}
	status, _, body, err := doZatcaImportBackendRequest(r, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("backend status %d: %s", status, strings.TrimSpace(string(body)))
	}

	var imports []ZatcaImportView
	if err := json.Unmarshal(body, &imports); err != nil {
		return nil, fmt.Errorf("decode signed-UBL import list: %w", err)
	}
	for i := range imports {
		prepareZatcaImportView(&imports[i])
	}
	return imports, nil
}

func fetchZatcaImportDetail(r *http.Request, id uint64) (ZatcaImportView, error) {
	status, _, body, err := doZatcaImportBackendRequest(
		r,
		http.MethodGet,
		fmt.Sprintf("/api/v2/zatca/import/%d", id),
		nil,
	)
	if err != nil {
		return ZatcaImportView{}, err
	}
	if status != http.StatusOK {
		return ZatcaImportView{}, fmt.Errorf("backend status %d: %s", status, strings.TrimSpace(string(body)))
	}

	var importView ZatcaImportView
	if err := json.Unmarshal(body, &importView); err != nil {
		return ZatcaImportView{}, fmt.Errorf("decode signed-UBL import detail: %w", err)
	}
	prepareZatcaImportView(&importView)
	return importView, nil
}

func prepareZatcaImportView(importView *ZatcaImportView) {
	importView.DisplayStatus = importView.Status
	importView.StatusClass = "bg-gray-100 text-gray-700"

	hasFailure := false
	if importView.Evidence != nil && importView.Evidence.FailureClass != nil &&
		strings.TrimSpace(*importView.Evidence.FailureClass) != "" {
		hasFailure = true
	}
	for _, issue := range importView.ValidationErrors {
		if strings.EqualFold(strings.TrimSpace(issue.Severity), "error") {
			hasFailure = true
			break
		}
	}

	if importView.Status == "review" {
		importView.DisplayStatus = "pending"
	}
	if hasFailure && (importView.Status == "review" || importView.Status == "approved" || importView.Status == "posted") {
		importView.DisplayStatus = "failed"
	}

	switch importView.DisplayStatus {
	case "pending":
		importView.StatusClass = "bg-blue-100 text-blue-700"
	case "failed":
		importView.StatusClass = "bg-red-100 text-red-700"
	case "approved":
		importView.StatusClass = "bg-emerald-100 text-emerald-700"
	case "posted":
		importView.StatusClass = "bg-purple-100 text-purple-700"
	case "rejected":
		importView.StatusClass = "bg-red-100 text-red-700"
	}

	importView.CanApprove = importView.Status == "review" && !hasFailure
	importView.CanRequestChanges = importView.Status == "review" || importView.Status == "approved"
	importView.CanPost = importView.Status == "approved" && !hasFailure
	importView.CanReject = importView.Status == "review" || importView.Status == "approved"
}

func parseZatcaImportRouteID(r *http.Request) (uint64, error) {
	raw := strings.TrimSpace(mux.Vars(r)["id"])
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid signed-UBL import id")
	}
	return id, nil
}

func proxyZatcaImportRequest(w http.ResponseWriter, r *http.Request, suffix string) {
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		writeZatcaImportJSONError(w, http.StatusUnauthorized, resources.L("auth.unauthorized"))
		return
	}

	var body []byte
	if r.Body != nil && r.Method != http.MethodGet {
		var err error
		body, err = io.ReadAll(io.LimitReader(r.Body, maxZatcaImportProxyRequestBytes+1))
		if err != nil {
			writeZatcaImportJSONError(w, http.StatusBadRequest, resources.L("zatca_import.invalid_request"))
			return
		}
		if len(body) > maxZatcaImportProxyRequestBytes {
			writeZatcaImportJSONError(w, http.StatusRequestEntityTooLarge, resources.L("zatca_import.file_too_large"))
			return
		}
	}

	status, contentType, responseBody, err := doZatcaImportBackendRequest(
		r,
		r.Method,
		"/api/v2/zatca/import"+suffix,
		body,
	)
	if err != nil {
		if helpers.IsUnauthorizedError(err) {
			writeZatcaImportJSONError(w, http.StatusUnauthorized, resources.L("auth.session_expired"))
			return
		}
		log.Printf("[zatca-import] backend proxy failed: %v", err)
		writeZatcaImportJSONError(w, http.StatusBadGateway, resources.L("zatca_import.backend_unavailable"))
		return
	}

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	_, _ = w.Write(responseBody)
}

func doZatcaImportBackendRequest(r *http.Request, method, path string, body []byte) (int, string, []byte, error) {
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		return http.StatusUnauthorized, "", nil, fmt.Errorf("no session token")
	}

	var requestBody io.Reader
	if body != nil {
		requestBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, config.BackendDomain+path, requestBody)
	if err != nil {
		return 0, "", nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := helpers.DoAuthedRequestWithRetry(req, sessionID)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, resp.Header.Get("Content-Type"), nil, readErr
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), responseBody, nil
}

func writeZatcaImportJSONError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": detail})
}
