package handlers

import (
	"encoding/json"
	"net/http"

	"afrita/config"
)

type versionResponse struct {
	Version string `json:"version"`
	Channel string `json:"channel"`
	Commit  string `json:"commit"`
}

func HandleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(versionResponse{
		Version: config.AppVersion,
		Channel: config.AppChannel,
		Commit:  config.AppCommit,
	})
}
