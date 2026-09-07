package handlers

import (
	"encoding/json"
	"net/http"

	"afrita/config"
)

type versionResponse struct {
	Version     string `json:"version"`
	Channel     string `json:"channel"`
	Commit      string `json:"commit"`
	CommitShort string `json:"commit_short"`
	WorkflowRun string `json:"workflow_run,omitempty"`
	Source      string `json:"source,omitempty"`
	BuiltAt     string `json:"built_at,omitempty"`
	ImageRef    string `json:"image_ref,omitempty"`
	ImageDigest string `json:"image_digest,omitempty"`
}

func HandleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(versionResponse{
		Version:     config.AppVersion,
		Channel:     config.AppChannel,
		Commit:      config.AppCommit,
		CommitShort: config.AppCommitShort,
		WorkflowRun: config.AppWorkflowRun,
		Source:      config.AppSource,
		BuiltAt:     config.AppBuiltAt,
		ImageRef:    config.AppImageRef,
		ImageDigest: config.AppImageDigest,
	})
}
