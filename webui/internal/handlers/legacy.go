package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

// legacyGone writes a 410 Gone response with a JSON body explaining the
// migration path to the replacement endpoint.
func legacyGone(w http.ResponseWriter, replacement string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusGone)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "gone",
		"message": "This endpoint has been removed. Use " + replacement + " instead.",
		"migrate": replacement,
	})
}

// LegacyCaddyGone handles all /api/caddy/* requests with a 410 Gone response
// that points callers to the replacement /api/serve/https/* endpoints.
func LegacyCaddyGone(w http.ResponseWriter, r *http.Request) {
	// Map common caddy sub-paths to their serve equivalents so the message is
	// as actionable as possible.
	sub := strings.TrimPrefix(r.URL.Path, "/api/caddy")
	replacement := "/api/serve/https" + sub
	if sub == "" || sub == "/" {
		replacement = "/api/serve/https/*"
	}
	legacyGone(w, replacement)
}

// LegacySocatGone handles all /api/socat/* requests with a 410 Gone response
// that points callers to the replacement /api/serve/tcp/* endpoints.
func LegacySocatGone(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, "/api/socat")
	replacement := "/api/serve/tcp" + sub
	if sub == "" || sub == "/" {
		replacement = "/api/serve/tcp/*"
	}
	legacyGone(w, replacement)
}
