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

	"github.com/gorilla/mux"
)

// ZatcaBranchStatus holds the ZATCA status for a branch.
// zatca_status: 0=deleted, 1=active, 2=expired, 3=not_active, 4=expiring_soon
// onboard_state (async worker progress): not_started|csr|compliance|invoices|done|failed
type ZatcaBranchStatus struct {
	BranchID     int               `json:"branch_id"`
	BranchName   string            `json:"branch_name"`
	Config       map[string]string `json:"config"`
	ZatcaStatus  int               `json:"zatca_status"`
	HasCSR       bool              `json:"has_csr"`
	HasProd      bool              `json:"has_production"`
	OnboardState string            `json:"onboard_state,omitempty"`
	LastError    string            `json:"last_error,omitempty"`
}

// backendZatcaGetResponse maps the aliased fields returned by
// GET /api/v2/branch/:id/zatca (sqlc snake_case JSON tags from SQL aliases).
type backendZatcaGetResponse struct {
	OrgID             string      `json:"orgid"`
	OrgUnit           string      `json:"orgunit"`
	OrgName           string      `json:"orgname"`
	CsrCountry        string      `json:"csrcountry"`
	CsrLoc            string      `json:"csrloc"`
	BizCat            string      `json:"bizcat"`
	Vat               string      `json:"vat"`
	Crn               string      `json:"crn"`
	Street            string      `json:"street"`
	Building          string      `json:"building"`
	District          string      `json:"district"`
	Postal            string      `json:"postal"`
	CsrLen            int         `json:"csrlen"`
	ProdLen           int         `json:"prodlen"`
	ZatcaRegisteredAt interface{} `json:"zatca_registered_at"`
	ZatcaStatus       int         `json:"zatca_status"`
	ZatcaOnboardedAt  interface{} `json:"zatca_onboarded_at"`
	Name              string      `json:"name"`
	// Async onboarding-worker progress (added in dev schema). Backend may
	// not yet expose these in older builds; in that case they decode as
	// zero-values and are dropped from the response by `omitempty`.
	OnboardState string `json:"onboard_state,omitempty"`
	LastError    string `json:"last_error,omitempty"`
}

// FetchZatcaConfigForBranch loads ZATCA config for a branch from the backend.
func FetchZatcaConfigForBranch(sessionID string, branchID int) (*ZatcaBranchStatus, error) {
	url := config.BackendDomain + "/api/v2/branch/" + strconv.Itoa(branchID) + "/zatca"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := helpers.DoAuthedRequestWithRetry(req, sessionID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 404 = no config row yet → return empty defaults
	if resp.StatusCode == http.StatusNotFound {
		return &ZatcaBranchStatus{
			BranchID:    branchID,
			Config:      map[string]string{},
			ZatcaStatus: 3, // not_active
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[ZATCA] GET config for branch %d: status %d, body: %s", branchID, resp.StatusCode, string(body))
		return nil, fmt.Errorf("backend status %d", resp.StatusCode)
	}

	var br backendZatcaGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&br); err != nil {
		return nil, fmt.Errorf("decode zatca config: %w", err)
	}

	// Map backend aliases → frontend field names
	cfg := map[string]string{
		"csr_org_identifier":    br.OrgID,
		"csr_org_unit":          br.OrgUnit,
		"csr_org_name":          br.OrgName,
		"csr_country":           br.CsrCountry,
		"csr_location":          br.CsrLoc,
		"csr_business_category": br.BizCat,
		"seller_vat":            br.Vat,
		"seller_crn":            br.Crn,
		"street":                br.Street,
		"building":              br.Building,
		"district":              br.District,
		"postal_code":           br.Postal,
	}

	return &ZatcaBranchStatus{
		BranchID:     branchID,
		BranchName:   br.Name,
		Config:       cfg,
		ZatcaStatus:  br.ZatcaStatus,
		HasCSR:       br.CsrLen > 0,
		HasProd:      br.ProdLen > 0,
		OnboardState: br.OnboardState,
		LastError:    br.LastError,
	}, nil
}

// ── HTTP Handlers ───────────────────────────────────────────────────

// HandleGetZatcaConfig returns ZATCA config for a branch (JSON API).
func HandleGetZatcaConfig(w http.ResponseWriter, r *http.Request) {
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"detail": "غير مصرح - يرجى تسجيل الدخول"})
		return
	}

	vars := mux.Vars(r)
	branchID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "invalid branch ID"})
		return
	}

	status, err := FetchZatcaConfigForBranch(sessionID, branchID)
	if err != nil {
		log.Printf("[ZATCA] fetch config error for branch %d: %v", branchID, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"detail": "failed to load ZATCA config"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"detail": status,
	})
}

// HandleSaveZatcaConfig saves ZATCA config for a branch via the backend API.
func HandleSaveZatcaConfig(w http.ResponseWriter, r *http.Request) {
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"detail": "غير مصرح - يرجى تسجيل الدخول"})
		return
	}

	vars := mux.Vars(r)
	branchID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "invalid branch ID"})
		return
	}

	var frontendCfg map[string]string
	if err := json.NewDecoder(r.Body).Decode(&frontendCfg); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "invalid request body"})
		return
	}

	// Preserve existing zatca_status. The backend UPSERT in branch_zatca_config
	// runs `zatca_status = VALUES(zatca_status)` on duplicate key, so whatever
	// value we send here is persisted. If we always sent 3 (not_active), saving
	// any field on an already-onboarded branch (status=1 active) would silently
	// downgrade it. So fetch the current status first and echo it back.
	preservedStatus := 3 // default for never-onboarded branches
	if existing, ferr := FetchZatcaConfigForBranch(sessionID, branchID); ferr == nil && existing != nil && existing.ZatcaStatus != 0 {
		preservedStatus = existing.ZatcaStatus
	}

	// Map frontend field names → backend field names.
	backendBody := map[string]interface{}{
		"branch_id":          branchID,
		"csr_org_identifier": frontendCfg["csr_org_identifier"],
		"csr_org_unit":       frontendCfg["csr_org_unit"],
		"csr_org_name":       frontendCfg["csr_org_name"],
		"csr_country":        frontendCfg["csr_country"],
		"csr_location":       frontendCfg["csr_location"],
		"business_category":  frontendCfg["csr_business_category"], // frontend → backend name
		"seller_vat":         frontendCfg["seller_vat"],
		"seller_crn":         frontendCfg["seller_crn"],
		"street":             frontendCfg["street"],
		"building":           frontendCfg["building"],
		"district":           frontendCfg["district"],
		"postal_code":        frontendCfg["postal_code"],
		"zatca_status":       preservedStatus,
	}

	bodyBytes, _ := json.Marshal(backendBody)
	log.Printf("[ZATCA] PUT branch %d body: %s", branchID, helpers.SanitizeForLog(string(bodyBytes)))

	apiURL := config.BackendDomain + "/api/v2/branch/" + strconv.Itoa(branchID) + "/zatca"
	backendReq, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"detail": "internal error"})
		return
	}
	backendReq.Header.Set("Content-Type", "application/json")

	resp, err := helpers.DoAuthedRequestWithRetry(backendReq, sessionID)
	if err != nil {
		log.Printf("[ZATCA] save config error for branch %d: %v", branchID, err)
		w.Header().Set("Content-Type", "application/json")
		if helpers.IsUnauthorizedError(err) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"detail": "انتهت الجلسة - يرجى تسجيل الدخول مرة أخرى"})
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"detail": "فشل الاتصال بالخادم: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[ZATCA] PUT branch %d response: status=%d body=%s", branchID, resp.StatusCode, string(respBody))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// HandleZatcaOnboard handles the OTP submission for ZATCA onboarding.
// Frontend sends POST → backend POST /api/v2/branch/:id/zatca/onboard
// Backend performs: validate config → generate CSR → compliance CSID → production CSID → save.
// Returns: {"detail":"success","zatca_status":1} on success,
//          {"detail":"error reason"} on failure (400/500/502).
func HandleZatcaOnboard(w http.ResponseWriter, r *http.Request) {
	sessionID := helpers.GetSessionIDFromRequest(r)
	if sessionID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"detail": "غير مصرح - يرجى تسجيل الدخول"})
		return
	}

	vars := mux.Vars(r)
	branchID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "invalid branch ID"})
		return
	}

	var otpReq struct {
		OTP string `json:"otp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&otpReq); err != nil || otpReq.OTP == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "OTP مطلوب"})
		return
	}

	otp := strings.TrimSpace(otpReq.OTP)
	if len(otp) != 6 || !isDigits(otp) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "OTP يجب أن يكون 6 أرقام"})
		return
	}

	bodyBytes, _ := json.Marshal(map[string]string{"otp": otp})
	apiURL := config.BackendDomain + "/api/v2/branch/" + strconv.Itoa(branchID) + "/zatca/onboard"
	log.Printf("[ZATCA] POST onboard branch %d", branchID)

	backendReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"detail": "internal error"})
		return
	}
	backendReq.Header.Set("Content-Type", "application/json")

	resp, err := helpers.DoAuthedRequestWithRetry(backendReq, sessionID)
	if err != nil {
		log.Printf("[ZATCA] onboard error for branch %d: %v", branchID, err)
		w.Header().Set("Content-Type", "application/json")
		if helpers.IsUnauthorizedError(err) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"detail": "انتهت الجلسة - يرجى تسجيل الدخول مرة أخرى"})
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"detail": "فشل الاتصال بالخادم: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[ZATCA] onboard branch %d response: status=%d body=%s", branchID, resp.StatusCode, string(respBody))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	// Backend 200 means: OTP accepted, onboarding queued for async processing.
	// The DB row is updated later by the worker — zatca_status stays 3 until
	// onboard_state transitions through csr → compliance → invoices → done.
	// Only when the worker finishes does zatca_status flip to 1 (active).
	// So we MUST NOT claim "active" here. We surface a "processing" flag and
	// the client polls GET /api/zatca/branch/:id for the real status.
	if len(respBody) > 0 {
		w.Write(respBody)
		return
	}
	if resp.StatusCode == http.StatusOK {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"detail":        "تم إرسال الطلب — جاري ربط الفرع بنظام زاتكا، قد يستغرق ذلك دقيقة",
			"processing":    true,
			"zatca_status":  3, // remains not_active until worker confirms
			"onboard_state": "csr",
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{
		"detail": fmt.Sprintf("خطأ من الخادم (HTTP %d)", resp.StatusCode),
	})
}

// isDigits returns true if s contains only ASCII digits.
func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
