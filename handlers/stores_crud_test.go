package handlers

import (
	"afrita/config"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleCreateStorePropagatesBackendFailure(t *testing.T) {
	seedTestSession()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/store" {
			t.Fatalf("unexpected backend request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "store unavailable"})
	}))
	defer backend.Close()

	previousDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = previousDomain })

	form := url.Values{
		"name":      {"مخزن الاختبار"},
		"branch_id": {"7"},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/stores/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})

	recorder := httptest.NewRecorder()
	HandleCreateStore(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
	}
	if recorder.Header().Get("HX-Redirect") != "" {
		t.Fatal("backend failure must not produce a success redirect")
	}
	if !strings.Contains(recorder.Body.String(), "store unavailable") {
		t.Fatalf("backend detail missing from response: %s", recorder.Body.String())
	}
}

func TestHandleCreateStoreSuccessRedirects(t *testing.T) {
	seedTestSession()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/store" {
			t.Fatalf("unexpected backend request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"detail":{"id":8,"name":"مخزن الاختبار"}}`))
	}))
	defer backend.Close()

	previousDomain := config.BackendDomain
	config.BackendDomain = backend.URL
	t.Cleanup(func() { config.BackendDomain = previousDomain })

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("name", "مخزن الاختبار"); err != nil {
		t.Fatalf("write name: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart form: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dashboard/stores/create", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})

	recorder := httptest.NewRecorder()
	HandleCreateStore(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("HX-Redirect"); got != "/dashboard/stores" {
		t.Fatalf("HX-Redirect = %q, want /dashboard/stores", got)
	}
}
