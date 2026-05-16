package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/sudocarlos/tailrelay/internal/config"
)

// TargetsHandler handles fetching the list of targets
type TargetsHandler struct {
	cfg *config.Config
}

// NewTargetsHandler creates a new TargetsHandler
func NewTargetsHandler(cfg *config.Config) *TargetsHandler {
	return &TargetsHandler{
		cfg: cfg,
	}
}

// APIList returns the unmarshaled targets array as JSON
func (h *TargetsHandler) APIList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Fallback to default path if not set in config
	targetPath := h.cfg.Paths.TargetsFile
	if targetPath == "" {
		targetPath = "/targets.json"
	}

	// Read file
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty array gracefully
			w.Write([]byte("[]"))
			return
		}
		log.Printf("Error reading targets file: %v", err)
		// Try to return empty array instead of 500 error to not break frontend
		w.Write([]byte("[]"))
		return
	}

	targets := []interface{}{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &targets); err != nil {
			log.Printf("Error parsing targets json: %v", err)
			w.Write([]byte("[]"))
			return
		}
	}

	json.NewEncoder(w).Encode(targets)
}
