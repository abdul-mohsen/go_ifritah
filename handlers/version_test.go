package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"afrita/config"
)

func TestHandleVersionReturnsBuildIdentity(t *testing.T) {
	oldVersion, oldChannel, oldCommit := config.AppVersion, config.AppChannel, config.AppCommit
	t.Cleanup(func() {
		config.AppVersion = oldVersion
		config.AppChannel = oldChannel
		config.AppCommit = oldCommit
	})
	config.AppVersion = "v1.2.3"
	config.AppChannel = "dev"
	config.AppCommit = "abc123"

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rr := httptest.NewRecorder()

	HandleVersion(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("HandleVersion status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var got versionResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := versionResponse{Version: "v1.2.3", Channel: "dev", Commit: "abc123"}
	if got != want {
		t.Fatalf("response = %+v, want %+v", got, want)
	}
}
