package handlers

import (
	"encoding/json"
	"net/http"
)

// InfoHandler returns static build metadata.
type InfoHandler struct {
	version string
	commit  string
}

// NewInfoHandler creates a new InfoHandler with the given version and commit strings.
func NewInfoHandler(version, commit string) *InfoHandler {
	return &InfoHandler{version: version, commit: commit}
}

// Info writes build metadata as JSON. This endpoint is unauthenticated.
func (h *InfoHandler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version": h.version,
		"commit":  h.commit,
	})
}
