package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"afrita/config"
)

func TestHandleVersionReturnsBuildIdentity(t *testing.T) {
	oldVersion, oldChannel, oldCommit := config.AppVersion, config.AppChannel, config.AppCommit
	oldCommitShort, oldWorkflowRun, oldWorkflowURL := config.AppCommitShort, config.AppWorkflowRun, config.AppWorkflowURL
	oldSource, oldBuiltAt := config.AppSource, config.AppBuiltAt
	oldImageRef, oldImageTag, oldImageDigest := config.AppImageRef, config.AppImageTag, config.AppImageDigest
	t.Cleanup(func() {
		config.AppVersion = oldVersion
		config.AppChannel = oldChannel
		config.AppCommit = oldCommit
		config.AppCommitShort = oldCommitShort
		config.AppWorkflowRun = oldWorkflowRun
		config.AppWorkflowURL = oldWorkflowURL
		config.AppSource = oldSource
		config.AppBuiltAt = oldBuiltAt
		config.AppImageRef = oldImageRef
		config.AppImageTag = oldImageTag
		config.AppImageDigest = oldImageDigest
	})
	config.AppVersion = "v1.2.3"
	config.AppChannel = "dev"
	config.AppCommit = "abcdef0123456789"
	config.AppCommitShort = "abcdef0"
	config.AppWorkflowRun = "42"
	config.AppWorkflowURL = "https://github.com/acme/ifritah/actions/runs/42"
	config.AppSource = "https://github.com/acme/ifritah"
	config.AppBuiltAt = "2026-09-07T12:00:00Z"
	config.AppImageRef = "docker.io/acme/ifritah-web:dev"
	config.AppImageTag = "dev"
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

	body := rr.Body.Bytes()
	var got versionResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := versionResponse{
		Version:     "v1.2.3",
		Channel:     "dev",
		Tag:         "dev",
		Ref:         "docker.io/acme/ifritah-web:dev",
		Digest:      "sha256:" + "a" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Commit:      "abcdef0123456789",
		ShortCommit: "abcdef0",
		CommitShort: "abcdef0",
		WorkflowRun: "42",
		WorkflowID:  "42",
		WorkflowURL: "https://github.com/acme/ifritah/actions/runs/42",
		Source:      "https://github.com/acme/ifritah",
		BuiltAt:     "2026-09-07T12:00:00Z",
		ImageRef:    "docker.io/acme/ifritah-web:dev",
		ImageTag:    "dev",
		ImageDigest: "sha256:" + "a" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if got != want {
		t.Fatalf("response = %+v, want %+v", got, want)
	}

	var raw map[string]string
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	for canonical, legacy := range map[string]string{
		"tag":          "image_tag",
		"ref":          "image_ref",
		"digest":       "image_digest",
		"short_commit": "commit_short",
	} {
		if raw[canonical] == "" {
			t.Fatalf("canonical field %q is empty or missing", canonical)
		}
		if raw[canonical] != raw[legacy] {
			t.Fatalf("canonical field %q = %q, legacy field %q = %q", canonical, raw[canonical], legacy, raw[legacy])
		}
	}
}
