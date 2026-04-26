package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"afrita/config"

	"github.com/gorilla/mux"
)

// TestSaveZatcaConfig_FieldMappingMatchesSchema locks down the contract that
// HandleSaveZatcaConfig forwards to the backend with EXACTLY the column names
// of branch_zatca_config (the writable subset). If a column is added/renamed
// in the schema, this test forces the handler to be updated.
//
// Reference: tmp/backend_schema.sql `branch_zatca_config` writable columns:
//   branch_id, csr_org_identifier, csr_org_unit, csr_org_name,
//   csr_country, csr_location, business_category,
//   seller_vat, seller_crn, street, building, district, postal_code,
//   zatca_status
func TestSaveZatcaConfig_FieldMappingMatchesSchema(t *testing.T) {
	expected := []string{
		"branch_id",
		"csr_org_identifier",
		"csr_org_unit",
		"csr_org_name",
		"csr_country",
		"csr_location",
		"business_category",
		"seller_vat",
		"seller_crn",
		"street",
		"building",
		"district",
		"postal_code",
		"zatca_status",
	}

	captured, _ := runFakeBackendSave(t, map[string]string{
		"csr_org_identifier":    "1-A|2-B|3-C",
		"csr_org_unit":          "IT",
		"csr_org_name":          "ACME LLC",
		"csr_country":           "SA",
		"csr_location":          "Riyadh",
		"csr_business_category": "Supply activities",
		"seller_vat":            "300000000000003",
		"seller_crn":            "7000000000",
		"street":                "King Fahd Rd",
		"building":              "1234",
		"district":              "Olaya",
		"postal_code":           "12345",
	}, /*existingStatus=*/ 0)

	if captured == nil {
		t.Fatal("no payload captured from handler")
	}

	for _, k := range expected {
		if _, ok := captured[k]; !ok {
			t.Errorf("missing required backend field %q in PUT payload; payload=%v", k, captured)
		}
	}

	// Reject obsolete frontend names leaking through unmapped.
	for _, k := range []string{"csr_business_category"} {
		if _, ok := captured[k]; ok {
			t.Errorf("frontend-only field %q should be mapped, not forwarded as-is", k)
		}
	}

	// Spot-check the rename mapping.
	if v, _ := captured["business_category"].(string); v != "Supply activities" {
		t.Errorf("business_category should be mapped from csr_business_category; got %q", v)
	}
}

// TestSaveZatcaConfig_PreservesActiveStatus locks down the bug fix:
// when an already-onboarded branch (zatca_status=1 active) saves edits, the
// handler must echo back the existing status, not reset to 3 (not_active).
func TestSaveZatcaConfig_PreservesActiveStatus(t *testing.T) {
	captured, _ := runFakeBackendSave(t, map[string]string{
		"csr_org_identifier":    "x", "csr_org_unit": "x", "csr_org_name": "x",
		"csr_country": "SA", "csr_location": "x", "csr_business_category": "x",
		"seller_vat": "300000000000003", "seller_crn": "7000000000",
		"street": "x", "building": "x", "district": "x", "postal_code": "x",
	}, /*existingStatus=*/ 1)

	gotStatus, _ := captured["zatca_status"].(float64) // JSON numbers decode as float64
	if int(gotStatus) != 1 {
		t.Errorf("expected zatca_status=1 preserved, got %v", captured["zatca_status"])
	}
}

// TestSaveZatcaConfig_NewBranchDefaultsNotActive: a branch with no existing
// row (GET 404) should default zatca_status to 3.
func TestSaveZatcaConfig_NewBranchDefaultsNotActive(t *testing.T) {
	captured, _ := runFakeBackendSave(t, map[string]string{
		"csr_org_identifier":    "x", "csr_org_unit": "x", "csr_org_name": "x",
		"csr_country": "SA", "csr_location": "x", "csr_business_category": "x",
		"seller_vat": "300000000000003", "seller_crn": "7000000000",
		"street": "x", "building": "x", "district": "x", "postal_code": "x",
	}, /*existingStatus=*/ -1) // -1 sentinel = backend returns 404

	gotStatus, _ := captured["zatca_status"].(float64)
	if int(gotStatus) != 3 {
		t.Errorf("expected default zatca_status=3 for new branch, got %v", captured["zatca_status"])
	}
}

// runFakeBackendSave spins up a fake backend that:
//   - Responds to GET /api/v2/branch/:id/zatca with the requested existingStatus
//     (or 404 when existingStatus == -1).
//   - Captures the PUT body to the same URL.
// It then drives HandleSaveZatcaConfig with the given frontend payload.
func runFakeBackendSave(t *testing.T, frontendPayload map[string]string, existingStatus int) (map[string]interface{}, int) {
	t.Helper()

	var (
		mu       sync.Mutex
		captured map[string]interface{}
		putCode  int
	)

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if existingStatus < 0 {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"orgid": "", "orgunit": "", "orgname": "", "csrcountry": "SA",
				"csrloc": "", "bizcat": "", "vat": "", "crn": "",
				"street": "", "building": "", "district": "", "postal": "",
				"csrlen": 0, "prodlen": 0,
				"zatca_status": existingStatus,
				"name":         "Test Branch",
			})
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			payload := map[string]interface{}{}
			_ = json.Unmarshal(body, &payload)
			mu.Lock()
			captured = payload
			putCode = http.StatusOK
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"detail":"ok"}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(fake.Close)

	prevDomain := config.BackendDomain
	config.BackendDomain = strings.TrimRight(fake.URL, "/")
	t.Cleanup(func() { config.BackendDomain = prevDomain })

	// Wire a fake session with a token so DoAuthedRequestWithRetry doesn't bail.
	const sessionID = "test-session-zatca-mapping"
	prevToken, prevHadToken := config.SessionTokens[sessionID]
	config.SessionTokens[sessionID] = "fake-jwt"
	t.Cleanup(func() {
		if prevHadToken {
			config.SessionTokens[sessionID] = prevToken
		} else {
			delete(config.SessionTokens, sessionID)
		}
	})

	bodyBytes, err := json.Marshal(frontendPayload)
	if err != nil {
		t.Fatalf("marshal frontend payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/zatca/branch/42", bytes.NewReader(bodyBytes))
	req = mux.SetURLVars(req, map[string]string{"id": "42"})
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	HandleSaveZatcaConfig(rr, req)

	if rr.Code >= 500 {
		t.Fatalf("handler returned %d: %s", rr.Code, rr.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	return captured, putCode
}
