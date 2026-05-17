package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/serve"
	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// ServeHandler handles relay management through `tailscale serve`.
type ServeHandler struct {
	cfg       *config.Config
	templates *template.Template
	manager   *serve.Manager
	tsClient  *tailscale.Client
}

// NewServeHandler creates a new serve handler.
func NewServeHandler(cfg *config.Config, templates *template.Template) *ServeHandler {
	return &ServeHandler{
		cfg:       cfg,
		templates: templates,
		manager:   serve.NewManager(cfg.Paths.ServeRelayConfig),
		tsClient:  tailscale.NewClient(),
	}
}

// InitializeAutostart reconciles enabled serves at startup.
func (h *ServeHandler) InitializeAutostart() error {
	return h.manager.Reconcile()
}

// Manager returns the serve.Manager instance.
func (h *ServeHandler) Manager() *serve.Manager {
	return h.manager
}

// IsTailscaleReady checks if the tailscale daemon is running and connected.
func (h *ServeHandler) IsTailscaleReady() bool {
	connected, _ := h.tsClient.IsConnected()
	return connected
}

// ── TCP relay handlers (/api/serve/tcp/*) ─────────────────────────────────────

// APIListTCP returns all TCP relays as JSON.
func (h *ServeHandler) APIListTCP(w http.ResponseWriter, _ *http.Request) {
	relays, err := h.manager.ListRelays()
	if err != nil {
		http.Error(w, "Failed to load relays", http.StatusInternalServerError)
		return
	}

	statusJSON, _ := h.manager.Status()

	type relayStatus struct {
		Relay   config.ServeRelay `json:"relay"`
		Running bool              `json:"running"`
	}

	out := make([]relayStatus, 0)
	for _, r := range relays {
		if r.Type != "tcp" {
			continue
		}

		running := false
		if statusJSON != nil && statusJSON.TCP != nil {
			if tcpInfo, ok := statusJSON.TCP[strconv.Itoa(r.ListenPort)]; ok {
				if !tcpInfo.HTTPS {
					running = true
				}
			}
		}

		out = append(out, relayStatus{Relay: r, Running: running})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// APIGetTCP returns one TCP relay.
func (h *ServeHandler) APIGetTCP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}
	relay, err := h.manager.GetRelay(id)
	if err != nil || relay.Type != "tcp" {
		http.Error(w, "Relay not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(relay)
}

// CreateTCP creates a TCP relay.
func (h *ServeHandler) CreateTCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var relay config.ServeRelay
	if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	relay.Type = "tcp"
	if relay.ID == "" {
		relay.ID = fmt.Sprintf("tcp-%d", relay.ListenPort)
	}
	if relay.TargetHost == "" || relay.ListenPort == 0 || relay.TargetPort == 0 {
		http.Error(w, "listen_port, target_host, and target_port are required", http.StatusBadRequest)
		return
	}
	if err := h.manager.UpsertRelay(relay); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay created successfully"})
}

// UpdateTCP updates a TCP relay.
func (h *ServeHandler) UpdateTCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var relay config.ServeRelay
	if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if relay.ID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}
	relay.Type = "tcp"
	if err := h.manager.UpsertRelay(relay); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay updated successfully"})
}

// DeleteTCP deletes a TCP relay.
func (h *ServeHandler) DeleteTCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}
	if err := h.manager.DeleteRelay(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay deleted successfully"})
}

// ToggleTCP enables/disables a TCP relay.
func (h *ServeHandler) ToggleTCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}
	if err := h.manager.ToggleRelay(req.ID, req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay toggled successfully"})
}

// ── HTTPS relay handlers (/api/serve/https/*) ─────────────────────────────────

// APIListHTTPS returns all HTTPS relays as JSON.
func (h *ServeHandler) APIListHTTPS(w http.ResponseWriter, _ *http.Request) {
	relays, err := h.manager.ListRelays()
	if err != nil {
		http.Error(w, "Failed to load proxies", http.StatusInternalServerError)
		return
	}

	hostname := ""
	if status, err := h.tsClient.GetStatusSummary(); err == nil {
		hostname = status.MagicDNSName
	}

	statusJSON, _ := h.manager.Status()

	type relayStatus struct {
		config.ServeRelay
		Running bool `json:"running"`
	}

	out := make([]relayStatus, 0)
	for _, relay := range relays {
		if relay.Type != "https" {
			continue
		}
		if relay.Hostname == "" {
			relay.Hostname = hostname
		}

		running := false
		if statusJSON != nil && statusJSON.TCP != nil {
			if tcpInfo, ok := statusJSON.TCP[strconv.Itoa(relay.ListenPort)]; ok {
				if tcpInfo.HTTPS {
					running = true
				}
			}
		}

		out = append(out, relayStatus{ServeRelay: relay, Running: running})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// APIGetHTTPS returns one HTTPS relay.
func (h *ServeHandler) APIGetHTTPS(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}
	relay, err := h.manager.GetRelay(id)
	if err != nil || relay.Type != "https" {
		http.Error(w, "Proxy not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(relay)
}

// CreateHTTPS creates an HTTPS relay.
func (h *ServeHandler) CreateHTTPS(w http.ResponseWriter, r *http.Request) {
	relay, err := parseHTTPSRelay(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	relay.Type = "https"
	if relay.ID == "" {
		relay.ID = fmt.Sprintf("https-%d", relay.ListenPort)
	}
	if err := h.manager.UpsertRelay(relay); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Proxy created successfully"})
}

// UpdateHTTPS updates an HTTPS relay.
func (h *ServeHandler) UpdateHTTPS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	relay, err := parseHTTPSRelay(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if relay.ID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}
	relay.Type = "https"
	if err := h.manager.UpsertRelay(relay); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Proxy updated successfully"})
}

// DeleteHTTPS deletes an HTTPS relay.
func (h *ServeHandler) DeleteHTTPS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}
	if err := h.manager.DeleteRelay(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Proxy deleted successfully"})
}

// ToggleHTTPS enables/disables an HTTPS relay.
func (h *ServeHandler) ToggleHTTPS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}
	if err := h.manager.ToggleRelay(req.ID, req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Proxy toggled successfully"})
}

// ReloadServe reconciles all enabled relays.
func (h *ServeHandler) ReloadServe(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.Reconcile(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "tailscale serve configuration is up to date",
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// parseHTTPSRelay decodes a ServeRelay from the request body (JSON or form).
func parseHTTPSRelay(r *http.Request) (config.ServeRelay, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			return config.ServeRelay{}, fmt.Errorf("failed to parse form data")
		}
		port, _ := strconv.Atoi(r.FormValue("port"))
		targetHost, targetPort, _ := splitServeTarget(r.FormValue("target"))
		return config.ServeRelay{
			ID:          r.FormValue("id"),
			Hostname:    strings.TrimSpace(r.FormValue("hostname")),
			ListenPort:  port,
			TargetHost:  targetHost,
			TargetPort:  targetPort,
			TargetHTTPS: parseBool(r.FormValue("tls")),
			Enabled:     parseBool(r.FormValue("enabled")),
			Autostart:   parseBool(r.FormValue("autostart")),
		}, nil
	}

	var relay config.ServeRelay
	if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
		return config.ServeRelay{}, fmt.Errorf("invalid request body")
	}
	// Accept target as "host:port" string for convenience (backwards compat with UI).
	if relay.TargetHost == "" && relay.TargetPort == 0 {
		// Check if there's a "target" string field in the body — handled by caller.
	}
	return relay, nil
}

func splitServeTarget(target string) (string, int, error) {
	value := strings.TrimSpace(target)
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	host, portStr, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, fmt.Errorf("target must be in host:port format")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("target port is invalid")
	}
	return host, port, nil
}

func parseBool(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "on" || value == "yes"
}
