package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"
	"afrita/helpers"
	"afrita/models"
)

func TestInvoiceListProjectsEveryDurableBillState(t *testing.T) {
	tests := []struct {
		name        string
		zatca       string
		wantOverall string
		wantIssued  bool
		wantReplace bool
		wantStatus  string
	}{
		{name: "pending", zatca: "pending", wantOverall: "pending", wantStatus: "قيد المعالجة"},
		{name: "retrying", zatca: "retrying", wantOverall: "retrying", wantStatus: "إعادة المحاولة"},
		{name: "result_unknown", zatca: "result_unknown", wantOverall: "result_unknown", wantStatus: "نتيجة غير معروفة"},
		{name: "action_required", zatca: "action_required", wantOverall: "action_required", wantStatus: "يتطلب إجراء"},
		{name: "rejected", zatca: "rejected", wantOverall: "rejected", wantStatus: "مرفوضة"},
		{name: "accepted", zatca: "accepted", wantOverall: "issued", wantIssued: true, wantReplace: true, wantStatus: "صادرة"},
		{name: "accepted_with_warning", zatca: "accepted_with_warning", wantOverall: "issued_with_warning", wantIssued: true, wantReplace: true, wantStatus: "صادرة مع تحذير"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v2/bill/all" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{{
					"id":              42,
					"sequence_number": 100,
					"state":           3,
					"credit_state":    0,
					"total":           115.0,
					"sale_status":     "completed",
					"payment_status":  "paid",
					"invoice_status":  "issued",
					"zatca_status":    tc.zatca,
				}})
			}))
			defer backend.Close()

			previousBackend := config.BackendDomain
			config.BackendDomain = backend.URL
			t.Cleanup(func() { config.BackendDomain = previousBackend })

			sessionID := "bill-state-" + tc.name
			config.SessionTokensMutex.Lock()
			config.SessionTokens[sessionID] = "test-token"
			config.SessionTokensMutex.Unlock()
			t.Cleanup(func() {
				config.SessionTokensMutex.Lock()
				delete(config.SessionTokens, sessionID)
				config.SessionTokensMutex.Unlock()
			})

			req := httptest.NewRequest(http.MethodGet, "/dashboard/invoices", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			w := httptest.NewRecorder()
			HandleInvoices(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%q", w.Code, w.Body.String())
			}
			body := w.Body.String()
			if !strings.Contains(body, `data-bill-state="`+tc.wantOverall+`"`) {
				t.Fatalf("missing projected overall state %q in %s", tc.wantOverall, body)
			}
			if !strings.Contains(body, tc.wantStatus) {
				t.Fatalf("missing projected status %q in %s", tc.wantStatus, body)
			}
			if tc.wantIssued && !strings.Contains(body, `data-bill-state="`+tc.wantOverall+`"`) {
				t.Errorf("expected issued projection for %s", tc.name)
			}
			creditLink := `href="/dashboard/invoices/credit/42"`
			if tc.wantReplace && !strings.Contains(body, creditLink) {
				t.Errorf("expected replacement action for terminal state %s", tc.name)
			}
			if !tc.wantReplace && strings.Contains(body, creditLink) {
				t.Errorf("replacement action rendered for non-terminal state %s", tc.name)
			}
			if !tc.wantIssued && strings.Contains(body, `data-bill-state="issued"`) {
				t.Errorf("false issued projection rendered for %s", tc.name)
			}
		})
	}
}

func TestProjectBillStateDoesNotUseLegacyIssuedOrReplacementWhenDurableUnknown(t *testing.T) {
	invoice := models.Invoice{}
	invoice.State = 3
	invoice.StateProjection = helpers.BillStateProjectionFromRaw(map[string]interface{}{
		"sale_status":    "completed",
		"payment_status": "paid",
		"invoice_status": "issued",
		"zatca_status":   "result_unknown",
		"state":          3,
	})

	state := helpers.ProjectBillState(invoice)
	if state.OverallState != "result_unknown" {
		t.Fatalf("overall state = %q, want result_unknown", state.OverallState)
	}
	if state.Issued {
		t.Fatal("durable unknown state must not be issued")
	}
	if state.CanDuplicate || state.CanReplace {
		t.Fatal("duplicate/replacement actions must be disabled while result is unknown")
	}
}
