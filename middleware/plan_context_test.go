package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"afrita/config"
)

func TestCSRFMiddlewareAddsPlanContext(t *testing.T) {
	previousTenantID := config.TenantID
	previousMasterDB := config.MasterDB
	t.Cleanup(func() {
		config.TenantID = previousTenantID
		config.MasterDB = previousMasterDB
	})

	config.TenantID = "plan-context-test"
	config.MasterDB = nil

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Context().Value(config.TenantIDContextKey); got != config.TenantID {
			t.Errorf("tenantID context = %v, want %q", got, config.TenantID)
		}
		if got := r.Context().Value(config.PlanContextKey); got != config.PlanSolo {
			t.Errorf("plan context = %v, want %q", got, config.PlanSolo)
		}
		if got := r.Context().Value(config.PlanLevelContextKey); got != 1 {
			t.Errorf("planLevel context = %v, want 1", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	CSRFMiddleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("response status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
