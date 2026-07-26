package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"afrita/config"
	"afrita/helpers"
)

func TestHandlePOSRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dashboard/pos", nil)
	rr := httptest.NewRecorder()

	HandlePOS(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect for an anonymous request, got %d", rr.Code)
	}
}

func TestHandlePOSRendersStoresAndSelectedStore(t *testing.T) {
	seedTestSession()
	helpers.APICache.Set("stores", nil, -time.Second)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/stores/all" {
			t.Fatalf("unexpected backend request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected authenticated backend request, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":7,"name":"Main Store"},{"id":8,"name":"Second Store"}]}`)
	}))
	defer backend.Close()

	previousBackend := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = previousBackend })

	req := httptest.NewRequest(http.MethodGet, "/dashboard/pos?store_id=8", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	rr := httptest.NewRecorder()

	HandlePOS(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, expected := range []string{"id=\"pos-search\"", "Main Store", "Second Store"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected rendered POS page to contain %q", expected)
		}
	}
	if !strings.Contains(body, `value="8" selected`) {
		t.Fatalf("expected requested store to be selected: %s", body)
	}
}

func TestHandlePOSStoreLoadErrorIsRendered(t *testing.T) {
	seedTestSession()
	helpers.APICache.Set("stores", nil, -time.Second)

	previousBackend := config.BackendDomain
	config.BackendDomain = "http://127.0.0.1:1"
	t.Cleanup(func() { config.BackendDomain = previousBackend })

	req := httptest.NewRequest(http.MethodGet, "/dashboard/pos", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	rr := httptest.NewRecorder()

	HandlePOS(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with an in-page error, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "تعذر تحميل المخازن") {
		t.Fatalf("expected store loading error in rendered page")
	}
}
