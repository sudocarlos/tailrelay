package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/socat"
)

// SocatHandler handles socat-related requests
type SocatHandler struct {
	cfg       *config.Config
	templates *template.Template
	manager   *socat.Manager
}

// NewSocatHandler creates a new socat handler
func NewSocatHandler(cfg *config.Config, templates *template.Template) *SocatHandler {
	manager := socat.NewManager(
		"socat",
		cfg.Paths.SocatRelayConfig,
	)

	return &SocatHandler{
		cfg:       cfg,
		templates: templates,
		manager:   manager,
	}
}

// InitializeAutostart starts all relays with autostart enabled
func (h *SocatHandler) InitializeAutostart() error {
	return h.manager.StartAll()
}

// List renders the socat relay management page
func (h *SocatHandler) List(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.manager.GetStatus()
	if err != nil {
		log.Printf("Error loading relays: %v", err)
		statuses = []socat.RelayStatus{}
	}

	// Count running and enabled relays
	runningCount := 0
	enabledCount := 0
	for _, status := range statuses {
		if status.Running {
			runningCount++
		}
		if status.Relay.Enabled {
			enabledCount++
		}
	}

	data := map[string]interface{}{
		"Title":        "Socat Relays",
		"Statuses":     statuses,
		"RunningCount": runningCount,
		"EnabledCount": enabledCount,
	}

	if err := h.templates.ExecuteTemplate(w, "socat.html", data); err != nil {
		log.Printf("Error rendering socat template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create handles creating a new relay
func (h *SocatHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var relay config.SocatRelay
	if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate ID if not provided
	if relay.ID == "" {
		relay.ID = generateRelayID()
	}

	// Set default enabled state
	if !relay.Enabled {
		relay.Enabled = true
	}

	// Add relay
	if err := socat.AddRelay(h.cfg.Paths.SocatRelayConfig, relay); err != nil {
		log.Printf("Error adding relay: %v", err)
		http.Error(w, "Failed to add relay", http.StatusInternalServerError)
		return
	}

	// Start relay if enabled
	if relay.Enabled {
		if err := h.manager.StartRelay(&relay); err != nil {
			log.Printf("Warning: failed to start relay: %v", err)
		}
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Relay created successfully",
		"relay":   relay,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Update handles updating an existing relay
func (h *SocatHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var relay config.SocatRelay
	if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if relay.ID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}

	// Get existing relay to check if it's running
	existing, err := socat.GetRelay(h.cfg.Paths.SocatRelayConfig, relay.ID)
	if err != nil {
		log.Printf("Error getting relay: %v", err)
		http.Error(w, "Relay not found", http.StatusNotFound)
		return
	}

	// Stop if running
	if existing.PID != 0 {
		if err := h.manager.StopRelay(existing); err != nil {
			log.Printf("Warning: failed to stop relay: %v", err)
		}
	}

	// Update relay
	if err := socat.UpdateRelay(h.cfg.Paths.SocatRelayConfig, relay); err != nil {
		log.Printf("Error updating relay: %v", err)
		http.Error(w, "Failed to update relay", http.StatusInternalServerError)
		return
	}

	// Restart if enabled
	if relay.Enabled {
		if err := h.manager.StartRelay(&relay); err != nil {
			log.Printf("Warning: failed to start relay: %v", err)
		}
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Relay updated successfully",
		"relay":   relay,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Delete handles deleting a relay
func (h *SocatHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relayID := r.URL.Query().Get("id")
	if relayID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}

	// Get relay to stop if running
	relay, err := socat.GetRelay(h.cfg.Paths.SocatRelayConfig, relayID)
	if err != nil {
		log.Printf("Error getting relay: %v", err)
		http.Error(w, "Relay not found", http.StatusNotFound)
		return
	}

	// Stop if running
	if relay.PID != 0 {
		if err := h.manager.StopRelay(relay); err != nil {
			log.Printf("Warning: failed to stop relay: %v", err)
		}
	}

	// Delete relay
	if err := socat.DeleteRelay(h.cfg.Paths.SocatRelayConfig, relayID); err != nil {
		log.Printf("Error deleting relay: %v", err)
		http.Error(w, "Failed to delete relay", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Relay deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Toggle handles enabling/disabling a relay
func (h *SocatHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.ID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}

	// Get relay
	relay, err := socat.GetRelay(h.cfg.Paths.SocatRelayConfig, request.ID)
	if err != nil {
		log.Printf("Error getting relay: %v", err)
		http.Error(w, "Relay not found", http.StatusNotFound)
		return
	}

	// Toggle relay
	if err := socat.ToggleRelay(h.cfg.Paths.SocatRelayConfig, request.ID, request.Enabled); err != nil {
		log.Printf("Error toggling relay: %v", err)
		http.Error(w, "Failed to toggle relay", http.StatusInternalServerError)
		return
	}

	// Start or stop based on enabled state
	if request.Enabled {
		relay.Enabled = true
		if err := h.manager.StartRelay(relay); err != nil {
			log.Printf("Warning: failed to start relay: %v", err)
		}
	} else {
		if relay.PID != 0 {
			if err := h.manager.StopRelay(relay); err != nil {
				log.Printf("Warning: failed to stop relay: %v", err)
			}
		}
	}

	response := map[string]string{
		"status":  "success",
		"message": "Relay toggled successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Start handles starting a relay
func (h *SocatHandler) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relayID := r.URL.Query().Get("id")
	if relayID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}

	relay, err := socat.GetRelay(h.cfg.Paths.SocatRelayConfig, relayID)
	if err != nil {
		log.Printf("Error getting relay: %v", err)
		http.Error(w, "Relay not found", http.StatusNotFound)
		return
	}

	if err := h.manager.StartRelay(relay); err != nil {
		log.Printf("Error starting relay: %v", err)
		http.Error(w, fmt.Sprintf("Failed to start relay: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Relay started successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Stop handles stopping a relay
func (h *SocatHandler) Stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relayID := r.URL.Query().Get("id")
	if relayID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}

	relay, err := socat.GetRelay(h.cfg.Paths.SocatRelayConfig, relayID)
	if err != nil {
		log.Printf("Error getting relay: %v", err)
		http.Error(w, "Relay not found", http.StatusNotFound)
		return
	}

	if err := h.manager.StopRelay(relay); err != nil {
		log.Printf("Error stopping relay: %v", err)
		http.Error(w, fmt.Sprintf("Failed to stop relay: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Relay stopped successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Restart handles restarting a relay
func (h *SocatHandler) Restart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relayID := r.URL.Query().Get("id")
	if relayID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}

	relay, err := socat.GetRelay(h.cfg.Paths.SocatRelayConfig, relayID)
	if err != nil {
		log.Printf("Error getting relay: %v", err)
		http.Error(w, "Relay not found", http.StatusNotFound)
		return
	}

	if err := h.manager.RestartRelay(relay); err != nil {
		log.Printf("Error restarting relay: %v", err)
		http.Error(w, fmt.Sprintf("Failed to restart relay: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Relay restarted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RestartAll handles restarting all relays
func (h *SocatHandler) RestartAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.RestartAll(); err != nil {
		log.Printf("Error restarting all relays: %v", err)
		http.Error(w, fmt.Sprintf("Failed to restart relays: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "All relays restarted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIList returns all relays as JSON
func (h *SocatHandler) APIList(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.manager.GetStatus()
	if err != nil {
		log.Printf("Error loading relays: %v", err)
		http.Error(w, "Failed to load relays", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
}

// APIGet returns a single relay as JSON
func (h *SocatHandler) APIGet(w http.ResponseWriter, r *http.Request) {
	relayID := r.URL.Query().Get("id")
	if relayID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}

	relay, err := socat.GetRelay(h.cfg.Paths.SocatRelayConfig, relayID)
	if err != nil {
		log.Printf("Error getting relay: %v", err)
		http.Error(w, "Relay not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(relay)
}

// generateRelayID generates a random ID for relays
func generateRelayID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
