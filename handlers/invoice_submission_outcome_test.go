package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afrita/config"
	"afrita/helpers"

	"github.com/gorilla/mux"
)

func TestHandleInvoicesUsesDurableSubmissionOutcomeForStatusAndActions(t *testing.T) {
	helpers.APICache.Delete("clients")
	helpers.APICache.Delete("stores")
	t.Cleanup(func() {
		helpers.APICache.Delete("clients")
		helpers.APICache.Delete("stores")
	})

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/bill/all":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id": 901, "sequence_number": 901,
					"total": 100.0, "total_before_vat": 86.96, "total_vat": 13.04,
					"discount": 0.0, "state": 3, "credit_state": 0, "type": false,
					"effective_date": map[string]interface{}{"Time": "2026-10-29T00:00:00Z", "Valid": true},
				},
				{
					"id": 902, "sequence_number": 902,
					"total": 200.0, "total_before_vat": 173.91, "total_vat": 26.09,
					"discount": 0.0, "state": 3, "credit_state": 0, "type": false,
					"effective_date": map[string]interface{}{"Time": "2026-10-28T00:00:00Z", "Valid": true},
				},
				{
					"id": 903, "sequence_number": 903,
					"total": 300.0, "total_before_vat": 260.87, "total_vat": 39.13,
					"discount": 0.0, "state": 3, "credit_state": 0, "type": false,
					"effective_date": map[string]interface{}{"Time": "2026-10-27T00:00:00Z", "Valid": true},
				},
			})
		case "/api/v2/zatca/monitor/submissions":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"invoice_id": 901, "state": "rejected", "provider_code": "INV_REJECTED"},
				{"invoice_id": 902, "state": "ambiguous", "retryable": true, "next_retry_at": "2026-10-29T20:00:00Z"},
				{"invoice_id": 903, "state": "pending", "retryable": true},
			})
		case "/api/v2/client/all", "/api/v2/store/all", "/api/v2/stores/all":
			_ = json.NewEncoder(w).Encode([]interface{}{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	prevBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = prevBackend })
	seedTestSession()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/invoices", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleInvoices(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, want := range []string{"مرفوضة", "نتيجة غير مؤكدة", "قيد الإرسال"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected durable submission label %q in invoice list, body=%q", want, body)
		}
	}
	for _, id := range []string{"901", "902", "903"} {
		if strings.Contains(body, "/dashboard/invoices/credit/"+id) {
			t.Fatalf("invoice %s must not expose issued-only credit action for non-success outcome", id)
		}
	}
}

func TestHandleGetInvoiceUsesDurableSubmissionOutcomeDetails(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/settings":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		case "/api/v2/bill/904":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 904, "sequence_number": 904, "state": 3,
				"total": 400.0, "total_before_vat": 347.83, "total_vat": 52.17,
				"discount": 0.0, "type": false,
				"effective_date": "2026-10-29T00:00:00Z",
				"products":       []interface{}{}, "manual_products": []interface{}{},
			})
		case "/api/v2/zatca/monitor/submissions":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"invoice_id": 904, "state": "accepted_with_warning",
					"provider_code": "WARN-904", "provider_warning_code": "VAT_ROUNDING",
					"provider_warning": "rounding adjusted", "provider_reference": "ZATCA-904",
					"retryable": true, "retry_count": 1, "artifact_reference": "artifacts/904/response.xml",
					"cleared_xml_path": "artifacts/904/cleared.xml",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	prevBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = prevBackend })
	seedTestSession()

	req := httptest.NewRequest(http.MethodGet, "/bill/904", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "904"})
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	w := httptest.NewRecorder()

	HandleGetInvoice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, want := range []string{"صادرة مع تحذير", "WARN-904", "VAT_ROUNDING", "قابلة لإعادة المحاولة", "artifacts/904/response.xml"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected durable detail value %q in invoice page, body=%q", want, body)
		}
	}
	if !strings.Contains(body, `data-testid="zatca-submission-outcome"`) {
		t.Fatal("expected durable submission details panel in invoice detail")
	}
}
