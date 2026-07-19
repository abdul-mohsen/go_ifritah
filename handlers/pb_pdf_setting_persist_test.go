package handlers

import (
	"afrita/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSaveSettingsSendsPBPDFRequiredToBackend is a regression test for a bug
// where "pb_pdf_required" (whether a PDF attachment is required on purchase
// bills) was treated as a frontend-only setting - excluded from
// settingsCategoryMap and therefore never sent to the backend at all. It
// only ever lived in the process-local settingsByToken map, which is wiped
// on every restart/rebuild, making the setting appear to silently reset.
func TestSaveSettingsSendsPBPDFRequiredToBackend(t *testing.T) {
	var capturedPayloads []map[string]interface{}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v2/settings") {
			_, _ = w.Write([]byte(`{"data":{}}`))
			return
		}
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/api/v2/settings") {
			var payload map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			capturedPayloads = append(capturedPayloads, payload)
			_, _ = w.Write([]byte(`{"detail":"success","updated":1}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer backend.Close()

	origDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = origDomain }()

	cleanup := setupPBTestSession("pb-pdf-setting-test", "pb-pdf-setting-token")
	defer cleanup()

	form := "pb_pdf_required=optional"
	req := httptest.NewRequest(http.MethodPost, "/dashboard/settings", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "pb-pdf-setting-test"})
	w := httptest.NewRecorder()

	HandleSaveSettings(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got %d. body: %s", w.Code, w.Body.String())
	}

	found := false
	for _, payload := range capturedPayloads {
		if payload["category"] != "invoice" {
			continue
		}
		settings, ok := payload["settings"].(map[string]interface{})
		if !ok {
			continue
		}
		if v, ok := settings["pb_pdf_required"]; ok && v == "optional" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected pb_pdf_required=optional to be sent to backend PUT /api/v2/settings under the invoice category, got payloads: %+v", capturedPayloads)
	}
}
