package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
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
	tsClient := tailscale.NewClient()
	return &ServeHandler{
		cfg:       cfg,
		templates: templates,
		manager:   serve.NewManagerWithControlServerDetection(cfg.Paths.ServeRelayConfig, tsClient, cfg.Tailscale.ControlServer != ""),
		tsClient:  tsClient,
	}
}

// InitializeAutostart reconciles autostart serves at startup.
// It applies only relays with Autostart=true, regardless of their runtime
// Enabled state, so that the Auto toggle takes effect on container boot.
func (h *ServeHandler) InitializeAutostart() error {
	return h.manager.ReconcileAutostart()
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

// writeServeResult encodes a JSON success response.  If err is
// ErrTailscaleNotReady it writes 202 Accepted so the UI can show a "pending"
// state instead of an error.  ErrRelayNotFound produces 404.  Any other error
// produces 500.
func writeServeResult(w http.ResponseWriter, err error, msg string) {
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": msg})
		return
	}
	if errors.Is(err, serve.ErrTailscaleNotReady) {
		log.Printf("serve: tailscale not ready, returning 202 Accepted")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "pending",
			"message": msg + " (tailscale not yet ready — relay config saved and will be applied when connected)",
		})
		return
	}
	if errors.Is(err, serve.ErrRelayNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if errors.Is(err, serve.ErrFunnelNotAllowed) {
		log.Printf("serve: funnel not allowed, returning 409 Conflict")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Tailscale Funnel is not allowed for this device. Add the \"funnel\" node attribute to your tailnet policy file (see https://tailscale.com/kb/1223/funnel), then try again.",
		})
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// ── TCP relay handlers (/api/serve/tcp/*) ─────────────────────────────────────

// APIListTCP returns all TCP relays as JSON.
func (h *ServeHandler) APIListTCP(w http.ResponseWriter, _ *http.Request) {
	relays, err := h.manager.ListRelays()
	if err != nil {
		http.Error(w, "Failed to load relays", http.StatusInternalServerError)
		return
	}

	statusJSON, statusErr := h.manager.Status()
	if statusErr != nil {
		log.Printf("serve: status check failed, running state may be inaccurate: %v", statusErr)
	}

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
	writeServeResult(w, h.manager.UpsertRelay(relay), "Relay created successfully")
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
	writeServeResult(w, h.manager.UpsertRelay(relay), "Relay updated successfully")
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
	err := h.manager.DeleteRelay(id)
	writeServeResult(w, err, "Relay deleted successfully")
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
	err := h.manager.ToggleRelay(req.ID, req.Enabled)
	writeServeResult(w, err, "Relay toggled successfully")
}

// ── HTTPS relay handlers (/api/serve/https/*) ─────────────────────────────────

// currentHostnameAndStatus returns the live MagicDNS name (empty if
// Tailscale status can't be read) and the current `tailscale serve
// status --json` snapshot, logging a warning if the latter fails.
func (h *ServeHandler) currentHostnameAndStatus() (string, *serve.ServeStatusJSON) {
	hostname := ""
	if status, err := h.tsClient.GetStatusSummary(); err == nil {
		hostname = status.MagicDNSName
	}

	statusJSON, statusErr := h.manager.Status()
	if statusErr != nil {
		log.Printf("serve: status check failed, running state may be inaccurate: %v", statusErr)
	}

	return hostname, statusJSON
}

// APIListHTTPS returns all HTTPS relays as JSON.
func (h *ServeHandler) APIListHTTPS(w http.ResponseWriter, _ *http.Request) {
	relays, err := h.manager.ListRelays()
	if err != nil {
		http.Error(w, "Failed to load proxies", http.StatusInternalServerError)
		return
	}

	hostname, statusJSON := h.currentHostnameAndStatus()
	// Resolve once per request: WebListenerScheme() may perform a live
	// LocalAPI prefs lookup, so it must not be called per-relay in the loop
	// below (see serve.Manager.WebListenerScheme doc comment).
	scheme := h.manager.WebListenerScheme()

	type relayStatus struct {
		config.ServeRelay
		Hostname       string `json:"hostname"`
		Running        bool   `json:"running"`
		ListenerScheme string `json:"listener_scheme"`
	}

	out := make([]relayStatus, 0)
	for _, relay := range relays {
		if relay.Type != "https" {
			continue
		}

		running := false
		if statusJSON != nil && statusJSON.TCP != nil {
			if tcpInfo, ok := statusJSON.TCP[strconv.Itoa(relay.ListenPort)]; ok {
				if tcpInfo.HTTPS == (scheme == "https") {
					running = true
				}
			}
		}

		out = append(out, relayStatus{
			ServeRelay:     relay,
			Hostname:       hostname,
			Running:        running,
			ListenerScheme: scheme,
		})
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
	writeServeResult(w, h.manager.UpsertRelay(relay), "Proxy created successfully")
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
	writeServeResult(w, h.manager.UpsertRelay(relay), "Proxy updated successfully")
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
	err := h.manager.DeleteRelay(id)
	writeServeResult(w, err, "Proxy deleted successfully")
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
	err := h.manager.ToggleRelay(req.ID, req.Enabled)
	writeServeResult(w, err, "Proxy toggled successfully")
}

// ── Funnel relay handlers (/api/serve/funnel/*) ───────────────────────────────

// APIListFunnel returns all funnel relays as JSON.
func (h *ServeHandler) APIListFunnel(w http.ResponseWriter, _ *http.Request) {
	relays, err := h.manager.ListRelays()
	if err != nil {
		http.Error(w, "Failed to load funnels", http.StatusInternalServerError)
		return
	}

	hostname, statusJSON := h.currentHostnameAndStatus()

	type relayStatus struct {
		config.ServeRelay
		Hostname string `json:"hostname"`
		Running  bool   `json:"running"`
	}

	out := make([]relayStatus, 0)
	for _, relay := range relays {
		if relay.Type != "funnel" {
			continue
		}

		out = append(out, relayStatus{ServeRelay: relay, Hostname: hostname, Running: funnelIsRunning(statusJSON, relay.ListenPort)})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// funnelIsRunning reports whether the given port appears in the AllowFunnel
// map of `tailscale serve status --json`. AllowFunnel is keyed by
// "host:port", so we match on the port suffix.
func funnelIsRunning(status *serve.ServeStatusJSON, port int) bool {
	if status == nil || status.AllowFunnel == nil {
		return false
	}
	suffix := fmt.Sprintf(":%d", port)
	for key, allowed := range status.AllowFunnel {
		if allowed && strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

// APIGetFunnel returns one funnel relay.
func (h *ServeHandler) APIGetFunnel(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Funnel ID is required", http.StatusBadRequest)
		return
	}
	relay, err := h.manager.GetRelay(id)
	if err != nil || relay.Type != "funnel" {
		http.Error(w, "Funnel not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(relay)
}

// CreateFunnel creates a funnel relay.
func (h *ServeHandler) CreateFunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	relay, err := parseFunnelRelay(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if relay.ID == "" {
		relay.ID = fmt.Sprintf("funnel-%d", relay.ListenPort)
	}
	writeServeResult(w, h.manager.UpsertRelay(relay), "Funnel created successfully")
}

// UpdateFunnel updates a funnel relay.
func (h *ServeHandler) UpdateFunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	relay, err := parseFunnelRelay(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if relay.ID == "" {
		http.Error(w, "Funnel ID is required", http.StatusBadRequest)
		return
	}
	writeServeResult(w, h.manager.UpsertRelay(relay), "Funnel updated successfully")
}

// DeleteFunnel deletes a funnel relay.
func (h *ServeHandler) DeleteFunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Funnel ID is required", http.StatusBadRequest)
		return
	}
	err := h.manager.DeleteRelay(id)
	writeServeResult(w, err, "Funnel deleted successfully")
}

// ToggleFunnel enables/disables a funnel relay.
func (h *ServeHandler) ToggleFunnel(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Funnel ID is required", http.StatusBadRequest)
		return
	}
	err := h.manager.ToggleRelay(req.ID, req.Enabled)
	writeServeResult(w, err, "Funnel toggled successfully")
}

// ReloadServe reconciles all enabled relays.
func (h *ServeHandler) ReloadServe(w http.ResponseWriter, r *http.Request) {
	writeServeResult(w, h.manager.Reconcile(), "tailscale serve configuration is up to date")
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

// parseFunnelRelay decodes a funnel ServeRelay from a JSON request body and
// validates the required fields shared by both funnel transports.
func parseFunnelRelay(r *http.Request) (config.ServeRelay, error) {
	var relay config.ServeRelay
	if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
		return config.ServeRelay{}, fmt.Errorf("invalid request body")
	}
	relay.Type = "funnel"
	if relay.ListenPort == 0 || relay.TargetHost == "" || relay.TargetPort == 0 {
		return config.ServeRelay{}, fmt.Errorf("listen_port, target_host, and target_port are required")
	}
	if !serve.IsFunnelPort(relay.ListenPort) {
		return config.ServeRelay{}, fmt.Errorf("listen_port must be one of %v", serve.FunnelPorts)
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
