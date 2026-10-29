package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"

	"github.com/gorilla/mux"
)

func TestHandleGetZatcaConfigSurfacesDurableOnboardingMetadata(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		state        string
		step         string
		attempt      int
		retryCount   int
		errorCode    string
		errorMessage string
		completedAt  string
	}{
		{
			name:         "invalid OTP failure",
			status:       3,
			state:        "failed",
			step:         "failed",
			attempt:      2,
			retryCount:   1,
			errorCode:    "invalid_otp",
			errorMessage: "OTP rejected",
			completedAt:  "2026-10-29T17:00:00Z",
		},
		{
			name:         "provider unavailable failure",
			status:       3,
			state:        "failed",
			step:         "failed",
			attempt:      3,
			retryCount:   2,
			errorCode:    "provider_unavailable",
			errorMessage: "ZATCA provider unavailable",
			completedAt:  "2026-10-29T17:05:00Z",
		},
		{
			name:        "durable success",
			status:      1,
			state:       "done",
			step:        "done",
			attempt:     4,
			retryCount:  0,
			completedAt: "2026-10-29T17:10:00Z",
		},
	}

	const sessionID = "test-session-t012-onboarding-ui"
	config.SessionTokens[sessionID] = "fake-jwt"
	t.Cleanup(func() {
		delete(config.SessionTokens, sessionID)
	})

	previousBackend := config.BackendDomain
	t.Cleanup(func() {
		config.BackendDomain = previousBackend
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"orgid":                "311111",
					"orgunit":              "IT",
					"orgname":              "ACME",
					"csrcountry":           "SA",
					"csrloc":               "RRRD2929",
					"bizcat":               "Retail",
					"vat":                  "300000000000003",
					"crn":                  "7000000000",
					"street":               "King Fahd Rd",
					"building":             "1234",
					"district":             "Olaya",
					"postal":               "12345",
					"csrlen":               128,
					"prodlen":              256,
					"zatca_status":         tt.status,
					"name":                 "Main Branch",
					"onboard_state":        tt.state,
					"onboard_step":         tt.step,
					"onboard_attempt":      tt.attempt,
					"onboard_retry_count":  tt.retryCount,
					"onboard_error_code":   tt.errorCode,
					"onboard_error":        tt.errorMessage,
					"last_error":           tt.errorMessage,
					"onboard_completed_at": tt.completedAt,
					"last_attempt_at":      "2026-10-29T17:10:00Z",
				})
			}))
			defer backend.Close()
			config.BackendDomain = backend.URL

			req := httptest.NewRequest(http.MethodGet, "/api/zatca/branch/7", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "7"})
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			rr := httptest.NewRecorder()

			HandleGetZatcaConfig(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
			}

			var envelope struct {
				Detail map[string]interface{} `json:"detail"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, rr.Body.String())
			}

			detail := envelope.Detail
			if got, _ := detail["onboard_state"].(string); got != tt.state {
				t.Errorf("onboard_state = %q, want %q", got, tt.state)
			}
			if got, _ := detail["onboard_step"].(string); got != tt.step {
				t.Errorf("onboard_step = %q, want %q", got, tt.step)
			}
			if got, _ := detail["onboard_error_code"].(string); got != tt.errorCode {
				t.Errorf("onboard_error_code = %q, want %q", got, tt.errorCode)
			}
			if got, _ := detail["onboard_error"].(string); got != tt.errorMessage {
				t.Errorf("onboard_error = %q, want %q", got, tt.errorMessage)
			}
			if got, _ := detail["onboard_completed_at"].(string); got != tt.completedAt {
				t.Errorf("onboard_completed_at = %q, want %q", got, tt.completedAt)
			}
			if got, ok := detail["onboard_retry_count"].(float64); !ok || int(got) != tt.retryCount {
				t.Errorf("onboard_retry_count = %#v, want %d", detail["onboard_retry_count"], tt.retryCount)
			}
		})
	}
}

func TestHandleGetZatcaConfigDoesNotClaimActiveForPendingState(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"zatca_status":        1,
			"onboard_state":       "compliance",
			"onboard_step":        "compliance",
			"onboard_attempt":     5,
			"onboard_retry_count": 0,
			"name":                "Main Branch",
		})
	}))
	defer backend.Close()

	const sessionID = "test-session-t012-pending-ui"
	config.SessionTokens[sessionID] = "fake-jwt"
	t.Cleanup(func() {
		delete(config.SessionTokens, sessionID)
	})

	previousBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() {
		config.BackendDomain = previousBackend
	})

	req := httptest.NewRequest(http.MethodGet, "/api/zatca/branch/7", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "7"})
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	rr := httptest.NewRecorder()

	HandleGetZatcaConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"onboard_state":""`) {
		t.Fatal("pending onboarding state must not be dropped from the frontend response")
	}
}
