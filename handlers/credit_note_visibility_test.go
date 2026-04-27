package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"afrita/config"
	"afrita/models"
)

// TestCreditNoteButton_Visibility verifies which combinations of (state,
// credit_state) render the "Credit Note" action link in the invoices list.
//
// Per ZATCA regulations, a credit note IS the official mechanism to reverse
// a cleared invoice, so the button must remain visible for state=3
// (ZATCA-issued) too — as long as credit_state == 0.
//
// Matrix:
//   state=1, credit_state=0  -> SHOW   (submitted, not credited)
//   state=3, credit_state=0  -> SHOW   (ZATCA-issued, not credited) <- bug fix
//   state=0, credit_state=0  -> HIDE   (draft)
//   state=1, credit_state=1  -> HIDE   (already credited)
//   state=3, credit_state=3  -> HIDE   (already credited)
func TestCreditNoteButton_Visibility(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Dir(cwd)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	cases := []struct {
		name        string
		state       int
		creditState int
		expectShow  bool
	}{
		{"submitted_not_credited", 1, 0, true},
		{"zatca_issued_not_credited", 3, 0, true},
		{"draft", 0, 0, false},
		{"submitted_already_credited", 1, 1, false},
		{"zatca_issued_already_credited", 3, 3, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			invoices := []models.Invoice{
				{
					ID:             42,
					SequenceNumber: 100,
					Total:          115.0,
					TotalBeforeVAT: 100.0,
					TotalVAT:       15.0,
					Discount:       0.0,
					State:          tc.state,
					CreditState:    tc.creditState,
					Type:           true,
					EffectiveDate: struct {
						Time  string `json:"Time"`
						Valid bool   `json:"Valid"`
					}{Time: time.Now().Format(time.RFC3339), Valid: true},
				},
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v2/bill/all" {
					http.NotFound(w, r)
					return
				}
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
			creditLink := `href="/dashboard/invoices/credit/42"`
			has := strings.Contains(body, creditLink)

			if tc.expectShow && !has {
				t.Errorf("expected credit-note link for state=%d credit_state=%d, but it was hidden", tc.state, tc.creditState)
			}
			if !tc.expectShow && has {
				t.Errorf("expected credit-note link to be hidden for state=%d credit_state=%d, but it was rendered", tc.state, tc.creditState)
			}
		})
	}
}
