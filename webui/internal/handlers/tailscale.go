package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sudocarlos/tailrelay/internal/auth"
	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/serve"
	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// TailscaleHandler handles Tailscale-related requests
type TailscaleHandler struct {
	cfg       *config.Config
	templates *template.Template
	tsClient  *tailscale.Client
	authMW    *auth.Middleware
	serveMgr  *serve.Manager
	// cfgMu guards reads/writes of cfg.Tailscale.ControlServer, since cfg is
	// a pointer shared across handlers with no locking of its own.
	cfgMu sync.Mutex
}

// NewTailscaleHandler creates a new Tailscale handler
func NewTailscaleHandler(cfg *config.Config, templates *template.Template, authMW *auth.Middleware, serveMgr *serve.Manager) *TailscaleHandler {
	return &TailscaleHandler{
		cfg:       cfg,
		templates: templates,
		tsClient:  tailscale.NewClient(),
		authMW:    authMW,
		serveMgr:  serveMgr,
	}
}

// writeJSON writes a JSON response with Content-Type set.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// writeJSONError writes a JSON error response with the given HTTP status code.
func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "error",
		"message": message,
	})
}

// reconcileRelaysAsync runs Reconcile in the background after a short delay,
// allowing Tailscale to fully come up before restoring relay state.
func (h *TailscaleHandler) reconcileRelaysAsync() {
	if h.serveMgr == nil {
		return
	}
	go func() {
		// Wait for Tailscale to finish connecting before reconciling.
		time.Sleep(2 * time.Second)
		if err := h.serveMgr.Reconcile(); err != nil {
			log.Printf("Warning: failed to reconcile relays after Tailscale connect: %v", err)
		} else {
			log.Printf("Relays reconciled after Tailscale connect")
		}
	}()
}

// LoginWithKey handles non-interactive Tailscale authentication using a pre-generated
// auth key (e.g. "tskey-auth-k..." from Tailscale, or "hskey-..." from a
// self-hosted Headscale instance). The key is passed directly to `tailscale up
// --authkey=<key>`, authenticating and connecting in a single step without requiring
// the user to visit an auth URL.
func (h *TailscaleHandler) LoginWithKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		AuthKey string `json:"auth_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	body.AuthKey = strings.TrimSpace(body.AuthKey)
	if body.AuthKey == "" {
		writeJSONError(w, "auth_key cannot be empty", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(body.AuthKey, "tskey-") && !strings.HasPrefix(body.AuthKey, "hskey-") {
		writeJSONError(w, "invalid auth key: must start with 'tskey-' or 'hskey-'", http.StatusBadRequest)
		return
	}

	if err := h.tsClient.LoginWithAuthKey(body.AuthKey, h.controlServer()); err != nil {
		log.Printf("Error authenticating Tailscale with auth key: %v", err)
		writeJSONError(w, "Failed to authenticate with auth key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.reconcileRelaysAsync()

	writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Authenticated and connected successfully",
	})
}

// Login handles Tailscale login initiation.
// Returns the Tailscale auth URL so the user can authenticate in a browser.
func (h *TailscaleHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authURL, err := h.tsClient.Login(h.controlServer())
	if err != nil {
		log.Printf("Error initiating Tailscale login: %v", err)
		writeJSONError(w, "Failed to get login URL from Tailscale. The daemon may not be running or may already be connected.", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"status":   "success",
		"auth_url": authURL,
		"message":  "Visit the URL to authenticate with Tailscale",
	})
}

// Logout handles Tailscale logout
func (h *TailscaleHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.tsClient.Logout(); err != nil {
		log.Printf("Error logging out of Tailscale: %v", err)
		writeJSONError(w, "Failed to logout", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Logged out successfully",
	})
}

// Connect handles Tailscale connection
func (h *TailscaleHandler) Connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.tsClient.Up(); err != nil {
		log.Printf("Error connecting Tailscale: %v", err)
		writeJSONError(w, "Failed to connect", http.StatusInternalServerError)
		return
	}

	h.reconcileRelaysAsync()

	writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Connected successfully",
	})
}

// Disconnect handles Tailscale disconnection
func (h *TailscaleHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.tsClient.Down(); err != nil {
		log.Printf("Error disconnecting Tailscale: %v", err)
		writeJSONError(w, "Failed to disconnect", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Disconnected successfully",
	})
}

// ChangeHostname changes the Tailscale hostname without changing the active
// control server or other node preferences.
func (h *TailscaleHandler) ChangeHostname(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	body.Hostname = strings.TrimSpace(body.Hostname)
	if body.Hostname == "" {
		writeJSONError(w, "hostname cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.tsClient.UpWithHostname(body.Hostname); err != nil {
		log.Printf("Error changing Tailscale hostname: %v", err)
		writeJSONError(w, "Failed to change hostname: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Hostname changed to " + body.Hostname,
	})
}

// APIStatus returns Tailscale status as JSON
func (h *TailscaleHandler) APIStatus(w http.ResponseWriter, r *http.Request) {
	summary, err := h.tsClient.GetStatusSummary()
	if err != nil {
		log.Printf("Error getting Tailscale status: %v", err)
		// Return a degraded status rather than an error so the UI can still render
		writeJSON(w, &tailscale.StatusSummary{
			Connected:    false,
			BackendState: "Unknown",
			LastCheck:    time.Now(),
		})
		return
	}

	writeJSON(w, summary)
}

// APIPeers returns peer list as JSON
func (h *TailscaleHandler) APIPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := h.tsClient.GetPeers()
	if err != nil {
		log.Printf("Error getting Tailscale peers: %v", err)
		writeJSONError(w, "Failed to get peers", http.StatusInternalServerError)
		return
	}

	writeJSON(w, peers)
}

// PollStatus polls for login completion and optionally sets a session cookie
// once the daemon reports it is connected.
func (h *TailscaleHandler) PollStatus(w http.ResponseWriter, r *http.Request) {
	connected, err := h.tsClient.IsConnected()
	if err != nil {
		log.Printf("Error checking Tailscale connection: %v", err)
		writeJSONError(w, "Failed to check status", http.StatusInternalServerError)
		return
	}

	// If connected and token auth is enabled, set session cookie to allow
	// access from localhost without a separate password.
	if connected && h.authMW != nil {
		_ = h.authMW.SetSessionCookie(w, r)
	}

	writeJSON(w, map[string]interface{}{
		"connected": connected,
		"timestamp": time.Now().Unix(),
	})
}
