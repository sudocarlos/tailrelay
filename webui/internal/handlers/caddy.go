package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/sudocarlos/tailrelay-webui/internal/caddy"
	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/tailscale"
)

// CaddyHandler handles Caddy-related requests
type CaddyHandler struct {
	cfg       *config.Config
	templates *template.Template
	manager   *caddy.Manager
	tsClient  *tailscale.Client
}

// NewCaddyHandler creates a new Caddy handler
func NewCaddyHandler(cfg *config.Config, templates *template.Template) *CaddyHandler {
	// Use Caddy API instead of file-based config
	// Pass empty string for server name to enable auto-discovery
	manager := caddy.NewManager(
		caddy.DefaultAdminAPI,
		"", // Auto-discover server name from Caddy
	)

	return &CaddyHandler{
		cfg:       cfg,
		templates: templates,
		manager:   manager,
		tsClient:  tailscale.NewClient(),
	}
}

// List renders the Caddy proxy management page
func (h *CaddyHandler) List(w http.ResponseWriter, r *http.Request) {
	proxies, err := h.manager.ListProxies()
	if err != nil {
		log.Printf("Error loading proxies: %v", err)
		proxies = []config.CaddyProxy{}
	}

	// Get Caddy status
	running, _ := h.manager.GetStatus()

	// Get Tailscale FQDN
	tailscaleFQDN := ""
	if status, err := h.tsClient.GetStatusSummary(); err == nil {
		tailscaleFQDN = status.MagicDNSName
	}

	// Count enabled proxies
	enabledCount := 0
	for _, proxy := range proxies {
		if proxy.Enabled {
			enabledCount++
		}
	}

	data := map[string]interface{}{
		"Title":         "Caddy Proxies",
		"Proxies":       proxies,
		"Running":       running,
		"EnabledCount":  enabledCount,
		"TailscaleFQDN": tailscaleFQDN,
	}

	if err := h.templates.ExecuteTemplate(w, "caddy.html", data); err != nil {
		log.Printf("Error rendering caddy template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create handles creating a new proxy
func (h *CaddyHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var proxy config.CaddyProxy
	if err := json.NewDecoder(r.Body).Decode(&proxy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate ID if not provided
	if proxy.ID == "" {
		proxy.ID = generateID()
	}

	// Set default enabled state
	if !proxy.Enabled {
		proxy.Enabled = true
	}

	// Add proxy via API (no reload needed - API handles it instantly)
	if err := h.manager.AddProxy(proxy); err != nil {
		log.Printf("Error adding proxy: %v", err)
		http.Error(w, "Failed to add proxy", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Proxy created successfully",
		"proxy":   proxy,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Update handles updating an existing proxy
func (h *CaddyHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var proxy config.CaddyProxy
	if err := json.NewDecoder(r.Body).Decode(&proxy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if proxy.ID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}

	// Update proxy via API (no reload needed - API handles it instantly)
	if err := h.manager.UpdateProxy(proxy); err != nil {
		log.Printf("Error updating proxy: %v", err)
		http.Error(w, "Failed to update proxy", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Proxy updated successfully",
		"proxy":   proxy,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Delete handles deleting a proxy
func (h *CaddyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxyID := r.URL.Query().Get("id")
	if proxyID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}

	// Delete proxy via API (no reload needed - API handles it instantly)
	if err := h.manager.DeleteProxy(proxyID); err != nil {
		log.Printf("Error deleting proxy: %v", err)
		http.Error(w, "Failed to delete proxy", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Proxy deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Toggle handles enabling/disabling a proxy
func (h *CaddyHandler) Toggle(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}

	// Toggle proxy via API (no reload needed - API handles it instantly)
	if err := h.manager.ToggleProxy(request.ID, request.Enabled); err != nil {
		log.Printf("Error toggling proxy: %v", err)
		http.Error(w, "Failed to toggle proxy", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Proxy toggled successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Reload handles reloading Caddy configuration
// Note: This is now a no-op since Caddy API handles changes instantly
// Kept for backwards compatibility with the Web UI
func (h *CaddyHandler) Reload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if Caddy API is accessible
	running, err := h.manager.GetStatus()
	if err != nil || !running {
		log.Printf("Error checking Caddy status: %v", err)
		http.Error(w, fmt.Sprintf("Caddy API not accessible: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Caddy configuration is up to date (API-based management)",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIList returns all proxies as JSON
func (h *CaddyHandler) APIList(w http.ResponseWriter, r *http.Request) {
	proxies, err := h.manager.ListProxies()
	if err != nil {
		log.Printf("Error loading proxies: %v", err)
		http.Error(w, "Failed to load proxies", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxies)
}

// APIGet returns a single proxy as JSON
func (h *CaddyHandler) APIGet(w http.ResponseWriter, r *http.Request) {
	proxyID := r.URL.Query().Get("id")
	if proxyID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}

	proxy, err := h.manager.GetProxy(proxyID)
	if err != nil {
		log.Printf("Error getting proxy: %v", err)
		http.Error(w, "Proxy not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxy)
}

// generateID generates a random ID for proxies
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
