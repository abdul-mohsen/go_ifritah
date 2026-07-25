package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"afrita/config"

	"github.com/gorilla/mux"
)

func userRequest(t *testing.T, method, path string, values url.Values) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
	return req
}

func TestUserCreateProxyAndValidation(t *testing.T) {
	var received map[string]interface{}
	requests := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/user" {
			t.Fatalf("unexpected backend request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer backend.Close()
	previous := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = previous }()
	seedTestSession()

	req := userRequest(t, http.MethodPost, "/dashboard/users/create", url.Values{
		"username": {"manager1"}, "role": {"manager"}, "password": {"correct horse"},
		"confirm_password": {"correct horse"}, "active": {"true"},
	})
	rec := httptest.NewRecorder()
	HandleCreateUser(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("HX-Redirect") != "/dashboard/users" {
		t.Fatalf("create response = %d, HX-Redirect=%q", rec.Code, rec.Header().Get("HX-Redirect"))
	}
	if received["username"] != "manager1" || received["role"] != "manager" || received["password"] != "correct horse" || received["active"] != true {
		t.Fatalf("create payload = %#v", received)
	}

	invalid := userRequest(t, http.MethodPost, "/dashboard/users/create", url.Values{
		"role": {"employee"}, "password": {"correct horse"}, "confirm_password": {"correct horse"},
	})
	invalidRec := httptest.NewRecorder()
	HandleCreateUser(invalidRec, invalid)
	if invalidRec.Code != http.StatusOK || !strings.Contains(invalidRec.Body.String(), "اسم المستخدم") {
		t.Fatalf("validation response = %d, body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	if requests != 1 {
		t.Fatalf("validation unexpectedly proxied request; count=%d", requests)
	}
}

func TestUserUpdateAndPasswordResetProxy(t *testing.T) {
	var methods []string
	var updatePayload, resetPayload map[string]interface{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/user/all":
			_, _ = w.Write([]byte(`[{"id":9,"username":"worker","role":"employee","active":true}]`))
		case "/api/v2/user/9":
			if r.Method != http.MethodPut {
				t.Fatalf("user update method = %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusOK)
		case "/api/v2/user/9/password":
			if err := json.NewDecoder(r.Body).Decode(&resetPayload); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected backend path: %s", r.URL.Path)
		}
	}))
	defer backend.Close()
	previous := config.BackendDomain
	config.BackendDomain = backend.URL
	defer func() { config.BackendDomain = previous }()
	seedTestSession()

	req := userRequest(t, http.MethodPost, "/dashboard/users/9/update", url.Values{
		"username": {"worker"}, "role": {"manager"}, "active": {"true"},
	})
	req = mux.SetURLVars(req, map[string]string{"id": "9"})
	rec := httptest.NewRecorder()
	HandleUpdateUser(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("HX-Redirect") != "/dashboard/users" {
		t.Fatalf("update response = %d, HX-Redirect=%q", rec.Code, rec.Header().Get("HX-Redirect"))
	}
	if updatePayload["role"] != "manager" || updatePayload["active"] != true {
		t.Fatalf("update payload = %#v", updatePayload)
	}

	reset := userRequest(t, http.MethodPost, "/dashboard/users/9/update", url.Values{"new_password": {"new secret"}})
	reset = mux.SetURLVars(reset, map[string]string{"id": "9"})
	resetRec := httptest.NewRecorder()
	HandleUpdateUser(resetRec, reset)
	if resetRec.Code != http.StatusOK || resetRec.Header().Get("HX-Redirect") != "/dashboard/users/9/edit" {
		t.Fatalf("reset response = %d, HX-Redirect=%q", resetRec.Code, resetRec.Header().Get("HX-Redirect"))
	}
	if resetPayload["new_password"] != "new secret" {
		t.Fatalf("reset payload = %#v", resetPayload)
	}
	if len(methods) != 2 || methods[0] != "PUT /api/v2/user/9" || methods[1] != "POST /api/v2/user/9/password" {
		t.Fatalf("backend requests = %v", methods)
	}
}

func TestUserTemplatesAreRegistered(t *testing.T) {
	for _, name := range []string{"users", "add-user", "edit-user"} {
		if config.Templates[name] == nil {
			t.Fatalf("template %q is not registered", name)
		}
	}
}
