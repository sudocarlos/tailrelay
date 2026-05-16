package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
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

// APIListSocat returns TCP relays using the legacy socat API shape.
func (h *ServeHandler) APIListSocat(w http.ResponseWriter, _ *http.Request) {
	relays, err := h.manager.ListRelays()
	if err != nil {
		http.Error(w, "Failed to load relays", http.StatusInternalServerError)
		return
	}

	type socatStatus struct {
		Relay   config.SocatRelay `json:"Relay"`
		Running bool              `json:"Running"`
	}

	out := make([]socatStatus, 0, len(relays))
	for _, r := range relays {
		if r.Type != "tcp" {
			continue
		}
		out = append(out, socatStatus{
			Relay: config.SocatRelay{
				ID:         r.ID,
				ListenPort: r.ListenPort,
				TargetHost: r.TargetHost,
				TargetPort: r.TargetPort,
				Enabled:    r.Enabled,
				Autostart:  r.Autostart,
			},
			Running: r.Enabled,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// APIGetSocat returns one TCP relay using legacy socat API shape.
func (h *ServeHandler) APIGetSocat(w http.ResponseWriter, r *http.Request) {
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
	_ = json.NewEncoder(w).Encode(config.SocatRelay{
		ID:         relay.ID,
		ListenPort: relay.ListenPort,
		TargetHost: relay.TargetHost,
		TargetPort: relay.TargetPort,
		Enabled:    relay.Enabled,
		Autostart:  relay.Autostart,
	})
}

// CreateSocat creates a TCP relay from legacy payload.
func (h *ServeHandler) CreateSocat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var relay config.SocatRelay
	if err := json.NewDecoder(r.Body).Decode(&relay); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if relay.ID == "" {
		relay.ID = fmt.Sprintf("tcp-%d", relay.ListenPort)
	}
	if relay.TargetHost == "" || relay.ListenPort == 0 || relay.TargetPort == 0 {
		http.Error(w, "invalid relay payload", http.StatusBadRequest)
		return
	}
	if err := h.manager.UpsertRelay(config.ServeRelay{
		ID:         relay.ID,
		Type:       "tcp",
		ListenPort: relay.ListenPort,
		TargetHost: relay.TargetHost,
		TargetPort: relay.TargetPort,
		Enabled:    relay.Enabled,
		Autostart:  relay.Autostart,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay created successfully"})
}

// UpdateSocat updates a TCP relay from legacy payload.
func (h *ServeHandler) UpdateSocat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
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
	if err := h.manager.UpsertRelay(config.ServeRelay{
		ID:         relay.ID,
		Type:       "tcp",
		ListenPort: relay.ListenPort,
		TargetHost: relay.TargetHost,
		TargetPort: relay.TargetPort,
		Enabled:    relay.Enabled,
		Autostart:  relay.Autostart,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay updated successfully"})
}

// DeleteSocat deletes a TCP relay.
func (h *ServeHandler) DeleteSocat(w http.ResponseWriter, r *http.Request) {
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

// ToggleSocat enables/disables a TCP relay.
func (h *ServeHandler) ToggleSocat(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.ToggleRelay(request.ID, request.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay toggled successfully"})
}

// StartSocat starts a relay via legacy endpoint.
func (h *ServeHandler) StartSocat(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}
	if err := h.manager.ToggleRelay(id, true); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay started successfully"})
}

// StopSocat stops a relay via legacy endpoint.
func (h *ServeHandler) StopSocat(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}
	if err := h.manager.ToggleRelay(id, false); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay stopped successfully"})
}

// RestartSocat restarts a relay via legacy endpoint.
func (h *ServeHandler) RestartSocat(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Relay ID is required", http.StatusBadRequest)
		return
	}
	if err := h.manager.ToggleRelay(id, true); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Relay restarted successfully"})
}

// RestartAllSocat reconciles all enabled relays.
func (h *ServeHandler) RestartAllSocat(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.Reconcile(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "All relays restarted successfully"})
}

// APIListCaddy returns HTTPS relays using the legacy caddy API shape.
func (h *ServeHandler) APIListCaddy(w http.ResponseWriter, _ *http.Request) {
	relays, err := h.manager.ListRelays()
	if err != nil {
		http.Error(w, "Failed to load proxies", http.StatusInternalServerError)
		return
	}

	hostname := ""
	if status, err := h.tsClient.GetStatusSummary(); err == nil {
		hostname = status.MagicDNSName
	}

	out := make([]config.CaddyProxy, 0, len(relays))
	for _, relay := range relays {
		if relay.Type != "https" {
			continue
		}
		hname := relay.Hostname
		if hname == "" {
			hname = hostname
		}
		out = append(out, config.CaddyProxy{
			ID:        relay.ID,
			Hostname:  hname,
			Port:      relay.ListenPort,
			Target:    fmt.Sprintf("%s:%d", relay.TargetHost, relay.TargetPort),
			Enabled:   relay.Enabled,
			Autostart: relay.Autostart,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// APIGetCaddy returns one HTTPS relay using the legacy caddy API shape.
func (h *ServeHandler) APIGetCaddy(w http.ResponseWriter, r *http.Request) {
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
	_ = json.NewEncoder(w).Encode(config.CaddyProxy{
		ID:        relay.ID,
		Hostname:  relay.Hostname,
		Port:      relay.ListenPort,
		Target:    fmt.Sprintf("%s:%d", relay.TargetHost, relay.TargetPort),
		Enabled:   relay.Enabled,
		Autostart: relay.Autostart,
	})
}

// CreateCaddy creates an HTTPS relay from legacy caddy payload.
func (h *ServeHandler) CreateCaddy(w http.ResponseWriter, r *http.Request) {
	proxy, err := parseLegacyProxy(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if proxy.ID == "" {
		proxy.ID = fmt.Sprintf("https-%d", proxy.Port)
	}
	targetHost, targetPort, err := splitTarget(proxy.Target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.manager.UpsertRelay(config.ServeRelay{
		ID:         proxy.ID,
		Type:       "https",
		Hostname:   proxy.Hostname,
		ListenPort: proxy.Port,
		TargetHost: targetHost,
		TargetPort: targetPort,
		Enabled:    proxy.Enabled,
		Autostart:  proxy.Autostart,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Proxy created successfully"})
}

// UpdateCaddy updates an HTTPS relay via legacy endpoint.
func (h *ServeHandler) UpdateCaddy(w http.ResponseWriter, r *http.Request) {
	proxy, err := parseLegacyProxy(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if proxy.ID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}
	targetHost, targetPort, err := splitTarget(proxy.Target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.manager.UpsertRelay(config.ServeRelay{
		ID:         proxy.ID,
		Type:       "https",
		Hostname:   proxy.Hostname,
		ListenPort: proxy.Port,
		TargetHost: targetHost,
		TargetPort: targetPort,
		Enabled:    proxy.Enabled,
		Autostart:  proxy.Autostart,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Proxy updated successfully"})
}

// DeleteCaddy deletes an HTTPS relay.
func (h *ServeHandler) DeleteCaddy(w http.ResponseWriter, r *http.Request) {
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

// ToggleCaddy toggles an HTTPS relay.
func (h *ServeHandler) ToggleCaddy(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.ToggleRelay(request.ID, request.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Proxy toggled successfully"})
}

// ReloadCaddy keeps compatibility with old reload endpoint.
func (h *ServeHandler) ReloadCaddy(w http.ResponseWriter, r *http.Request) {
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

func parseLegacyProxy(r *http.Request) (config.CaddyProxy, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			return config.CaddyProxy{}, fmt.Errorf("failed to parse form data")
		}
		port, _ := strconv.Atoi(r.FormValue("port"))
		return config.CaddyProxy{
			ID:        r.FormValue("id"),
			Hostname:  strings.TrimSpace(r.FormValue("hostname")),
			Target:    strings.TrimSpace(r.FormValue("target")),
			Port:      port,
			Enabled:   parseBool(r.FormValue("enabled")),
			Autostart: parseBool(r.FormValue("autostart")),
		}, nil
	}

	var proxy config.CaddyProxy
	if err := json.NewDecoder(r.Body).Decode(&proxy); err != nil {
		return config.CaddyProxy{}, fmt.Errorf("invalid request body")
	}
	return proxy, nil
}

func splitTarget(target string) (string, int, error) {
	value := strings.TrimSpace(target)
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	last := strings.LastIndex(value, ":")
	if last <= 0 || last == len(value)-1 {
		return "", 0, fmt.Errorf("target must be in host:port format")
	}
	host := value[:last]
	port, err := strconv.Atoi(value[last+1:])
	if err != nil {
		return "", 0, fmt.Errorf("target port is invalid")
	}
	return host, port, nil
}
