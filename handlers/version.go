package handlers

import (
	"encoding/json"
	"net/http"

	"afrita/config"
)

type versionResponse struct {
	Version     string `json:"version"`
	Channel     string `json:"channel"`
	Tag         string `json:"tag,omitempty"`
	Ref         string `json:"ref,omitempty"`
	Digest      string `json:"digest,omitempty"`
	Commit      string `json:"commit"`
	ShortCommit string `json:"short_commit"`
	CommitShort string `json:"commit_short"`
	WorkflowRun string `json:"workflow_run,omitempty"`
	WorkflowID  string `json:"workflow_run_id,omitempty"`
	WorkflowURL string `json:"workflow_run_url,omitempty"`
	Source      string `json:"source,omitempty"`
	BuiltAt     string `json:"built_at,omitempty"`
	ImageRef    string `json:"image_ref,omitempty"`
	ImageTag    string `json:"image_tag,omitempty"`
	ImageDigest string `json:"image_digest,omitempty"`
}

func HandleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(versionResponse{
		Version:     config.AppVersion,
		Channel:     config.AppChannel,
		Tag:         config.AppImageTag,
		Ref:         config.AppImageRef,
		Digest:      config.AppImageDigest,
		Commit:      config.AppCommit,
		ShortCommit: config.AppCommitShort,
		CommitShort: config.AppCommitShort,
		WorkflowRun: config.AppWorkflowRun,
		WorkflowID:  config.AppWorkflowRun,
		WorkflowURL: config.AppWorkflowURL,
		Source:      config.AppSource,
		BuiltAt:     config.AppBuiltAt,
		ImageRef:    config.AppImageRef,
		ImageTag:    config.AppImageTag,
		ImageDigest: config.AppImageDigest,
	})
}
