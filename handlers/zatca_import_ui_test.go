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

func TestPrepareZatcaImportViewPostedFailureEvidence(t *testing.T) {
	failureClass := "trust_unavailable"
	tests := []struct {
		name       string
		importView ZatcaImportView
		wantStatus string
	}{
		{
			name: "failure class overrides posted",
			importView: ZatcaImportView{
				Status: "posted",
				Evidence: &ZatcaImportEvidenceView{
					FailureClass: &failureClass,
				},
			},
			wantStatus: "failed",
		},
		{
			name: "error validation overrides posted",
			importView: ZatcaImportView{
				Status: "posted",
				ValidationErrors: []ZatcaImportValidationErrorView{
					{Severity: "ERROR"},
				},
			},
			wantStatus: "failed",
		},
		{
			name: "successful posted import remains posted",
			importView: ZatcaImportView{
				Status: "posted",
			},
			wantStatus: "posted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepareZatcaImportView(&tt.importView)

			if tt.importView.DisplayStatus != tt.wantStatus {
				t.Fatalf("display status = %q, want %q", tt.importView.DisplayStatus, tt.wantStatus)
			}
		})
	}
}

func TestZatcaImportPagesRenderDurableStates(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/zatca/import":
			_, _ = w.Write([]byte(`[{"id":42,"status":"review","workflow_version":1,"source_content_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","created_at":"2026-10-29T18:00:00Z","updated_at":"2026-10-29T18:00:00Z"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/zatca/import/42":
			_, _ = w.Write([]byte(`{
				"id":42,
				"status":"review",
				"workflow_version":1,
				"source_content_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"created_at":"2026-10-29T18:00:00Z",
				"updated_at":"2026-10-29T18:00:00Z",
				"evidence":{
					"source_reference":"upload:42",
					"source_original_name":"supplier.xml",
					"source_mime_type":"application/xml",
					"source_provenance":"supplier_xml",
					"source_content_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"source_size":128,
					"canonical_data":{"invoice_number":"42"},
					"parsed_data":{"invoice_number":"42"},
					"signature_data":{"present":true,"status":"unverified"},
					"validation_data":{"review_required":true},
					"failure_class":"trust_unavailable",
					"failure_message":"trust unavailable"
				},
				"validation_errors":[{
					"severity":"error",
					"error_code":"trust_unavailable",
					"field_key":"signature",
					"message":"trust unavailable",
					"details":{"provider":"not configured"}
				}],
				"reviews":[],
				"status_history":[{
					"from_status":null,
					"to_status":"review",
					"transition_key":"create:42",
					"workflow_version":1,
					"reason":"review required",
					"created_at":"2026-10-29T18:00:00Z"
				}]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	previousBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = previousBackend })

	const sessionID = "test-session-zatca-import-ui"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "fake-jwt"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})

	t.Run("list shows pending import and detail route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/zatca/imports", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
		rr := httptest.NewRecorder()

		HandleZatcaImports(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		for _, want := range []string{"#42", "pending", "/dashboard/zatca/imports/42", "/api/zatca/import"} {
			if !strings.Contains(body, want) {
				t.Errorf("list body missing %q", want)
			}
		}
		if strings.Contains(strings.ToLower(body), "issued") || strings.Contains(body, "صادرة") {
			t.Fatal("import list must not claim an issued invoice state")
		}
	})

	t.Run("detail shows failure evidence and review actions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/zatca/imports/42", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "42"})
		req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
		rr := httptest.NewRecorder()

		HandleZatcaImportDetail(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		for _, want := range []string{"upload:42", "supplier.xml", "trust unavailable", "failed", "review", "reject"} {
			if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
				t.Errorf("detail body missing %q", want)
			}
		}
		if strings.Contains(body, "/api/zatca/import/42/post") {
			t.Fatal("failed review evidence must not expose the post action")
		}
		if strings.Contains(strings.ToLower(body), "issued") || strings.Contains(body, "صادرة") {
			t.Fatal("import detail must not claim an issued invoice state")
		}
	})
}

func TestZatcaImportReviewProxyForwardsWorkflowVersion(t *testing.T) {
	var captured struct {
		Method string
		Path   string
		Body   map[string]interface{}
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.Path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&captured.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"status":"approved","workflow_version":2}`))
	}))
	defer backend.Close()

	previousBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = previousBackend })

	const sessionID = "test-session-zatca-import-review"
	config.SessionTokensMutex.Lock()
	config.SessionTokens[sessionID] = "fake-jwt"
	config.SessionTokensMutex.Unlock()
	t.Cleanup(func() {
		config.SessionTokensMutex.Lock()
		delete(config.SessionTokens, sessionID)
		config.SessionTokensMutex.Unlock()
	})

	req := httptest.NewRequest(http.MethodPost, "/api/zatca/import/42/review",
		strings.NewReader(`{"decision":"approve","reason":"evidence checked","expected_workflow_version":1}`))
	req = mux.SetURLVars(req, map[string]string{"id": "42"})
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	HandleZatcaImportReview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if captured.Method != http.MethodPost || captured.Path != "/api/v2/zatca/import/42/review" {
		t.Fatalf("backend request = %s %s, want POST /api/v2/zatca/import/42/review", captured.Method, captured.Path)
	}
	if got, _ := captured.Body["expected_workflow_version"].(float64); got != 1 {
		t.Fatalf("workflow version = %v, want 1", captured.Body["expected_workflow_version"])
	}
}
