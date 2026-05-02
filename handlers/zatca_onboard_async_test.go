package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"

	"github.com/gorilla/mux"
)

// TestZatcaOnboard_BackendEmpty200_DoesNotClaimActive locks down the contract
// that a 200 response from the backend with an empty body means "OTP accepted,
// onboarding queued for async processing", NOT "branch is now active".
//
// The backend handler `OnboardBranchZatca` enqueues the job via
// `h.pub.OnboadBranch(branchID)` and returns 200 immediately. The async
// worker later updates `onboard_state` (csr → compliance → invoices →
// done|failed) and eventually flips `zatca_status` to 1 (active).
//
// If our frontend echoes back `zatca_status: 1` here it will lie to the user
// and corrupt the in-memory `zatcaStatusByBranch` map.
func TestZatcaOnboard_BackendEmpty200_DoesNotClaimActive(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mirror the real backend: 200 with empty body.
		w.WriteHeader(http.StatusOK)
	}))
	defer fake.Close()

	prevDomain := config.BackendDomain
	config.BackendDomain = strings.TrimRight(fake.URL, "/")
	defer func() { config.BackendDomain = prevDomain }()

	const sessionID = "test-session-onboard-async"
	config.SessionTokens[sessionID] = "fake-jwt"
	defer delete(config.SessionTokens, sessionID)

	body, _ := json.Marshal(map[string]string{"otp": "123456"})
	req := httptest.NewRequest(http.MethodPost, "/api/zatca/branch/42/onboard", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "42"})
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	HandleZatcaOnboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}

	// Must NOT claim active.
	if status, _ := resp["zatca_status"].(float64); int(status) == 1 {
		t.Errorf("zatca_status must not be 1 (active) on async-queued response; got %v", resp["zatca_status"])
	}

	// Should mark processing so the client polls.
	if processing, _ := resp["processing"].(bool); !processing {
		t.Errorf("expected processing=true on async-queued response; got %v", resp["processing"])
	}

	// Should expose an onboard_state value (csr is the first worker step).
	if state, _ := resp["onboard_state"].(string); state == "" {
		t.Errorf("expected onboard_state to be set; got empty")
	}
}

// TestFetchZatcaConfigForBranch_SurfacesOnboardState verifies the GET helper
// reads `onboard_state` and `last_error` from the backend (added in the dev
// schema for the async-onboarding worker) and exposes them on
// ZatcaBranchStatus so the polling UI can show progress.
func TestFetchZatcaConfigForBranch_SurfacesOnboardState(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"orgid": "", "orgunit": "", "orgname": "",
			"csrcountry": "SA", "csrloc": "", "bizcat": "",
			"vat": "", "crn": "",
			"street": "", "building": "", "district": "", "postal": "",
			"csrlen": 0, "prodlen": 0,
			"zatca_status":  3,
			"name":          "Test",
			"onboard_state": "compliance",
			"last_error":    "",
		})
	}))
	defer fake.Close()

	prevDomain := config.BackendDomain
	config.BackendDomain = strings.TrimRight(fake.URL, "/")
	defer func() { config.BackendDomain = prevDomain }()

	const sessionID = "test-session-onboard-state"
	config.SessionTokens[sessionID] = "fake-jwt"
	defer delete(config.SessionTokens, sessionID)

	got, err := FetchZatcaConfigForBranch(sessionID, 7)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.OnboardState != "compliance" {
		t.Errorf("OnboardState: want %q, got %q", "compliance", got.OnboardState)
	}
}
