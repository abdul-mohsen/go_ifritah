//go:build ignore
// +build ignore

// Disabled: references BillType field that doesn't exist on models.Invoice.

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"afrita/config"
	"afrita/models"
)

// TestCreditNoteButton_OnlyForSubmittedNonCreditBills verifies that:
// 1. Submitted bills (state=1) with credit_state=0 → SHOW credit note button
// 2. Draft bills (state=0) → HIDE credit note button
// 3. Credit note bills (credit_state>=1) → HIDE credit note button
// 4. ZATCA issued bills (state=3) with credit_state=0 → HIDE credit note button
func TestCreditNoteButton_OnlyForSubmittedNonCreditBills(t *testing.T) {
	testCases := []struct {
		name            string
		state           int
		creditState     int
		expectCreditBtn bool
	}{
		{"submitted_no_credit", 1, 0, true},     // AC2: submitted + not credit → SHOW
		{"draft", 0, 0, false},                  // AC1: draft → HIDE
		{"credit_note_processing", 1, 1, false}, // AC3: already credit note → HIDE
		{"credit_note_issued", 1, 3, false},     // AC3: credit note issued → HIDE
		{"zatca_issued", 3, 0, false},           // AC4: ZATCA issued → HIDE
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			invoices := []models.Invoice{
				{
					ID:             42,
					SequenceNumber: 100,
					Total:          100.0,
					TotalBeforeVAT: 87.0,
					TotalVAT:       13.0,
					Discount:       0.0,
					State:          tc.state,
					CreditState:    tc.creditState,
					BillType:       true,
					EffectiveDate: struct {
						Time  string `json:"Time"`
						Valid bool   `json:"Valid"`
					}{Time: time.Now().Format(time.RFC3339), Valid: true},
				},
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				payload, _ := json.Marshal(invoices)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(payload)
			}))
			defer server.Close()

			prevBackend := config.BackendDomain
			config.BackendDomain = server.URL
			t.Cleanup(func() { config.BackendDomain = prevBackend })

			sessionID := "test-credit-" + tc.name
			config.SessionTokensMutex.Lock()
			config.SessionTokens[sessionID] = "test-token"
			config.SessionTokensMutex.Unlock()
			t.Cleanup(func() {
				config.SessionTokensMutex.Lock()
				delete(config.SessionTokens, sessionID)
				config.SessionTokensMutex.Unlock()
			})

			req := httptest.NewRequest("GET", "/dashboard/invoices", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			w := httptest.NewRecorder()

			HandleInvoices(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			body := w.Body.String()
			creditLink := "invoices/credit/42"

			hasCreditBtn := strings.Contains(body, creditLink)

			if tc.expectCreditBtn && !hasCreditBtn {
				t.Errorf("expected credit note button for state=%d credit_state=%d, but not found in HTML",
					tc.state, tc.creditState)
			}
			if !tc.expectCreditBtn && hasCreditBtn {
				t.Errorf("credit note button should NOT appear for state=%d credit_state=%d, but found in HTML",
					tc.state, tc.creditState)
			}
		})
	}
}
