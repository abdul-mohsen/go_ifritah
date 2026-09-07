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
	oldCommitShort, oldWorkflowRun := config.AppCommitShort, config.AppWorkflowRun
	oldSource, oldBuiltAt := config.AppSource, config.AppBuiltAt
	oldImageRef, oldImageDigest := config.AppImageRef, config.AppImageDigest
	t.Cleanup(func() {
		config.AppVersion = oldVersion
		config.AppChannel = oldChannel
		config.AppCommit = oldCommit
		config.AppCommitShort = oldCommitShort
		config.AppWorkflowRun = oldWorkflowRun
		config.AppSource = oldSource
		config.AppBuiltAt = oldBuiltAt
		config.AppImageRef = oldImageRef
		config.AppImageDigest = oldImageDigest
	})
	config.AppVersion = "v1.2.3"
	config.AppChannel = "dev"
	config.AppCommit = "abcdef0123456789"
	config.AppCommitShort = "abcdef0"
	config.AppWorkflowRun = "https://github.com/acme/ifritah/actions/runs/42"
	config.AppSource = "https://github.com/acme/ifritah"
	config.AppBuiltAt = "2026-09-07T12:00:00Z"
	config.AppImageRef = "docker.io/acme/ifritah-web:dev"
	config.AppImageDigest = "sha256:" + "a" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

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
	want := versionResponse{
		Version:     "v1.2.3",
		Channel:     "dev",
		Commit:      "abcdef0123456789",
		CommitShort: "abcdef0",
		WorkflowRun: "https://github.com/acme/ifritah/actions/runs/42",
		Source:      "https://github.com/acme/ifritah",
		BuiltAt:     "2026-09-07T12:00:00Z",
		ImageRef:    "docker.io/acme/ifritah-web:dev",
		ImageDigest: "sha256:" + "a" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if got != want {
		t.Fatalf("response = %+v, want %+v", got, want)
	}
}
