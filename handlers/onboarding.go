package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"afrita/helpers"
	"afrita/models"
)

type onboardingZatcaStatus struct {
	BranchID      int    `json:"branch_id"`
	BranchName    string `json:"branch_name"`
	Status        int    `json:"status"`
	HasCSR        bool   `json:"has_csr"`
	HasProduction bool   `json:"has_production"`
	Configured    bool   `json:"configured"`
}

type onboardingState struct {
	Branches []models.Branch         `json:"branches"`
	Stores   []models.Store          `json:"stores"`
	ZATCA    []onboardingZatcaStatus `json:"zatca"`
}

func loadOnboardingState(token, sessionID string) (onboardingState, error) {
	state := onboardingState{
		Branches: []models.Branch{},
		Stores:   []models.Store{},
		ZATCA:    []onboardingZatcaStatus{},
	}

	branches, err := helpers.FetchBranches(token)
	if err != nil {
		return state, err
	}
	state.Branches = branches

	stores, err := helpers.FetchStores(token)
	if err != nil {
		return state, err
	}
	state.Stores = stores

	for _, branch := range branches {
		status, err := FetchZatcaConfigForBranch(sessionID, branch.ID)
		if err != nil {
			log.Printf("[ONBOARDING] ZATCA status unavailable for branch %d: %v", branch.ID, err)
			state.ZATCA = append(state.ZATCA, onboardingZatcaStatus{
				BranchID:   branch.ID,
				BranchName: branch.Name,
			})
			continue
		}

		configured := status.ZatcaStatus == 1 ||
			status.ZatcaStatus == 2 ||
			status.ZatcaStatus == 4 ||
			status.HasCSR ||
			status.HasProd
		if !configured {
			for _, value := range status.Config {
				if strings.TrimSpace(value) != "" {
					configured = true
					break
				}
			}
		}
		state.ZATCA = append(state.ZATCA, onboardingZatcaStatus{
			BranchID:      branch.ID,
			BranchName:    branch.Name,
			Status:        status.ZatcaStatus,
			HasCSR:        status.HasCSR,
			HasProduction: status.HasProd,
			Configured:    configured,
		})
	}

	return state, nil
}

// HandleOnboarding renders the first-run onboarding wizard.
func HandleOnboarding(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	pageError := ""
	if err := loadSettingsFromBackend(token); err != nil {
		pageError = "تعذر تحميل إعدادات الشركة حالياً. يمكنك المحاولة مرة أخرى."
	} else if GetSettingValue(token, "onboarding_completed") == "true" {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	state, err := loadOnboardingState(token, helpers.GetSessionIDFromRequest(r))
	if err != nil {
		log.Printf("[ONBOARDING] state load failed: %v", err)
		pageError = "تعذر تحميل بيانات الإعداد الأولي حالياً. يمكنك المحاولة مرة أخرى."
	}

	helpers.Render(w, r, "onboarding", map[string]interface{}{
		"title":    "إعداد حسابك",
		"settings": getSettings(token),
		"branches": state.Branches,
		"stores":   state.Stores,
		"zatca":    state.ZATCA,
		"error":    pageError,
	})
}

// HandleOnboardingState returns fresh branch, store, and ZATCA state for the
// wizard after a branch or store is created without reloading the page.
func HandleOnboardingState(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	state, err := loadOnboardingState(token, helpers.GetSessionIDFromRequest(r))
	if err != nil {
		http.Error(w, `{"error":"تعذر تحميل بيانات الإعداد الأولي"}`, http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(state)
}

// HandleCompleteOnboarding persists the completion marker and returns JSON so
// the wizard can redirect only after the setting has been accepted.
func HandleCompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	token, ok := helpers.GetTokenOrRedirect(w, r)
	if !ok {
		return
	}

	if err := saveSettingsToBackend(token, map[string]string{
		"onboarding_completed": "true",
	}); err != nil {
		http.Error(w, `{"error":"تعذر حفظ اكتمال الإعداد الأولي"}`, http.StatusBadGateway)
		return
	}

	ts := storeFor(token)
	ts.mu.Lock()
	ts.values["onboarding_completed"] = "true"
	ts.mu.Unlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]bool{"completed": true})
}
