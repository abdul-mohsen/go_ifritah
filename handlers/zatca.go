package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"

	"afrita/helpers"

	"github.com/gorilla/mux"
)

// ZatcaBranchStatus holds the ZATCA status for a branch.
// zatca_status: 0=deleted, 1=active, 2=expired, 3=not_active, 4=expiring_soon
type ZatcaBranchStatus struct {
	BranchID    int               `json:"branch_id"`
	BranchName  string            `json:"branch_name"`
	Config      map[string]string `json:"config"`
	ZatcaStatus int               `json:"zatca_status"`
	HasCSR      bool              `json:"has_csr"`
	HasProd     bool              `json:"has_production"`
}

// ── Mock ZATCA store (until backend is complete) ────────────────────
var (
	mockZatcaMu    sync.RWMutex
	mockZatcaStore = map[int]*ZatcaBranchStatus{} // branch_id → status
)

// FetchZatcaConfigForBranch loads ZATCA config for a branch.
// Currently mocked — will call GET /api/v2/branch/:id/zatca when backend is ready.
func FetchZatcaConfigForBranch(token string, branchID int) (*ZatcaBranchStatus, error) {
	// TODO: replace mock with real API call:
	// resp, err := helpers.DoAuthedRequest(req, token) where req = GET /api/v2/branch/:id/zatca

	mockZatcaMu.RLock()
	defer mockZatcaMu.RUnlock()

	if s, ok := mockZatcaStore[branchID]; ok {
		return s, nil
	}

	return &ZatcaBranchStatus{
		BranchID:    branchID,
		Config:      map[string]string{},
		ZatcaStatus: 3, // not_active
		HasCSR:      false,
		HasProd:     false,
	}, nil
}

// SaveZatcaConfigForBranch saves ZATCA config for a branch.
// Currently mocked — will call PUT /api/v2/branch/:id/zatca when backend is ready.
func SaveZatcaConfigForBranch(token string, branchID int, config map[string]string) error {
	// TODO: replace mock with real API call:
	// body, _ := json.Marshal(config)
	// req, _ := http.NewRequest("PUT", backendDomain+"/api/v2/branch/"+strconv.Itoa(branchID)+"/zatca", bytes.NewBuffer(body))

	mockZatcaMu.Lock()
	defer mockZatcaMu.Unlock()

	existing, ok := mockZatcaStore[branchID]
	if !ok {
		existing = &ZatcaBranchStatus{
			BranchID:    branchID,
			Config:      map[string]string{},
			ZatcaStatus: 3,
		}
	}
	for k, v := range config {
		existing.Config[k] = v
	}
	mockZatcaStore[branchID] = existing

	log.Printf("[ZATCA-MOCK] Saved config for branch %d (%d fields)", branchID, len(config))
	return nil
}

// OnboardZatcaBranch performs the ZATCA onboarding for a branch.
// Currently mocked — will call POST /api/v2/branch/:id/zatca/onboard when backend is ready.
func OnboardZatcaBranch(token string, branchID int, otp string) (int, string, error) {
	// TODO: replace mock with real API call:
	// body, _ := json.Marshal(map[string]string{"otp": otp})
	// req, _ := http.NewRequest("POST", backendDomain+"/api/v2/branch/"+strconv.Itoa(branchID)+"/zatca/onboard", bytes.NewBuffer(body))

	mockZatcaMu.Lock()
	defer mockZatcaMu.Unlock()

	existing, ok := mockZatcaStore[branchID]
	if !ok {
		existing = &ZatcaBranchStatus{
			BranchID:    branchID,
			Config:      map[string]string{},
			ZatcaStatus: 3,
		}
	}

	// Validate required fields
	required := []string{"csr_org_identifier", "csr_org_name", "seller_vat", "seller_crn"}
	for _, f := range required {
		if existing.Config[f] == "" {
			return 0, "يجب ملء جميع الحقول المطلوبة أولاً", nil
		}
	}

	// Mock success: set status to active
	existing.ZatcaStatus = 1 // active
	existing.HasCSR = true
	existing.HasProd = true
	mockZatcaStore[branchID] = existing

	log.Printf("[ZATCA-MOCK] Onboarded branch %d with OTP %s → status=active", branchID, otp)
	return 1, "تم ربط الفرع بنظام زاتكا بنجاح (وضع تجريبي)", nil
}

// ── HTTP Handlers ───────────────────────────────────────────────────

// HandleGetZatcaConfig returns ZATCA config for a branch (JSON API).
func HandleGetZatcaConfig(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}
	_ = token

	vars := mux.Vars(r)
	branchID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "invalid branch ID"})
		return
	}

	status, err := FetchZatcaConfigForBranch(token, branchID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"detail": "failed to load ZATCA config"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"detail": status,
	})
}

// HandleSaveZatcaConfig saves ZATCA config for a branch (JSON API).
func HandleSaveZatcaConfig(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	branchID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "invalid branch ID"})
		return
	}

	var config map[string]string
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "invalid request"})
		return
	}

	if err := SaveZatcaConfigForBranch(token, branchID, config); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"detail": "failed to save"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"detail": "success"})
}

// HandleZatcaOnboard handles the OTP submission for ZATCA onboarding.
func HandleZatcaOnboard(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	branchID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "invalid branch ID"})
		return
	}

	var req struct {
		OTP string `json:"otp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OTP == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "OTP مطلوب"})
		return
	}

	if len(req.OTP) != 6 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": "OTP يجب أن يكون 6 أرقام"})
		return
	}

	newStatus, msg, err := OnboardZatcaBranch(token, branchID, req.OTP)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"detail": "خطأ داخلي"})
		return
	}

	if newStatus == 0 {
		// Validation error from mock
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"detail": msg})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"detail":       msg,
		"zatca_status": newStatus,
	})
}
