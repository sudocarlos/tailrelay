package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/sudocarlos/tailrelay-webui/internal/auth"
	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/tailscale"
)

// TailscaleHandler handles Tailscale-related requests
type TailscaleHandler struct {
	cfg       *config.Config
	templates *template.Template
	tsClient  *tailscale.Client
	authMW    *auth.Middleware
}

// NewTailscaleHandler creates a new Tailscale handler
func NewTailscaleHandler(cfg *config.Config, templates *template.Template, authMW *auth.Middleware) *TailscaleHandler {
	return &TailscaleHandler{
		cfg:       cfg,
		templates: templates,
		tsClient:  tailscale.NewClient(),
		authMW:    authMW,
	}
}

// Status renders the Tailscale status page
func (h *TailscaleHandler) Status(w http.ResponseWriter, r *http.Request) {
	summary, err := h.tsClient.GetStatusSummary()
	if err != nil {
		log.Printf("Error getting Tailscale status: %v", err)
		summary = &tailscale.StatusSummary{
			Connected:    false,
			BackendState: "Error",
		}
	}

	peers, err := h.tsClient.GetPeers()
	if err != nil {
		log.Printf("Error getting Tailscale peers: %v", err)
		peers = []tailscale.PeerInfo{}
	}

	data := map[string]interface{}{
		"Title":          "Tailscale Status",
		"Summary":        summary,
		"Peers":          peers,
		"StateFormatted": tailscale.FormatBackendState(summary.BackendState),
	}

	if err := h.templates.ExecuteTemplate(w, "tailscale.html", data); err != nil {
		log.Printf("Error rendering tailscale template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Login handles Tailscale login initiation
func (h *TailscaleHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authURL, err := h.tsClient.LoginWithQR()
	if err != nil {
		log.Printf("Error initiating login: %v", err)
		http.Error(w, "Failed to initiate login", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":   "success",
		"auth_url": authURL,
		"message":  "Please visit the URL to authenticate",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Logout handles Tailscale logout
func (h *TailscaleHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.tsClient.Logout(); err != nil {
		log.Printf("Error logging out: %v", err)
		http.Error(w, "Failed to logout", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Logged out successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Connect handles Tailscale connection
func (h *TailscaleHandler) Connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.tsClient.Up(); err != nil {
		log.Printf("Error connecting: %v", err)
		http.Error(w, "Failed to connect", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Connected successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Disconnect handles Tailscale disconnection
func (h *TailscaleHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.tsClient.Down(); err != nil {
		log.Printf("Error disconnecting: %v", err)
		http.Error(w, "Failed to disconnect", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Disconnected successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIStatus returns Tailscale status as JSON
func (h *TailscaleHandler) APIStatus(w http.ResponseWriter, r *http.Request) {
	summary, err := h.tsClient.GetStatusSummary()
	if err != nil {
		log.Printf("Error getting status: %v", err)
		http.Error(w, "Failed to get status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// APIPeers returns peer list as JSON
func (h *TailscaleHandler) APIPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := h.tsClient.GetPeers()
	if err != nil {
		log.Printf("Error getting peers: %v", err)
		http.Error(w, "Failed to get peers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peers)
}

// PollStatus polls for login completion
func (h *TailscaleHandler) PollStatus(w http.ResponseWriter, r *http.Request) {
	// Check if connected
	connected, err := h.tsClient.IsConnected()
	if err != nil {
		log.Printf("Error checking connection: %v", err)
		http.Error(w, "Failed to check status", http.StatusInternalServerError)
		return
	}

	// If connected and token auth is enabled, set session cookie to allow access from localhost
	if connected && h.authMW != nil {
		h.authMW.SetSessionCookie(w, r)
	}

	response := map[string]interface{}{
		"connected": connected,
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
