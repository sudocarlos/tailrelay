package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/tailscale"
)

// DashboardHandler handles dashboard-related requests
type DashboardHandler struct {
	cfg       *config.Config
	templates *template.Template
	tsClient  *tailscale.Client
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(cfg *config.Config, templates *template.Template) *DashboardHandler {
	return &DashboardHandler{
		cfg:       cfg,
		templates: templates,
		tsClient:  tailscale.NewClient(),
	}
}

// Dashboard renders the main dashboard page
func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// Get Tailscale status
	tsSummary, err := h.tsClient.GetStatusSummary()
	if err != nil {
		log.Printf("Error getting Tailscale status: %v", err)
		tsSummary = &tailscale.StatusSummary{
			Connected:    false,
			BackendState: "Unknown",
		}
	}

	// Count relays and proxies
	relays, _ := config.LoadSocatRelays(h.cfg.Paths.SocatRelayConfig)
	proxies, _ := config.LoadCaddyProxies(h.cfg.Paths.CaddyProxyConfig)

	relayCount := 0
	if relays != nil {
		for _, relay := range relays.Relays {
			if relay.Enabled {
				relayCount++
			}
		}
	}

	proxyCount := 0
	if proxies != nil {
		for _, proxy := range proxies.Proxies {
			if proxy.Enabled {
				proxyCount++
			}
		}
	}

	data := map[string]interface{}{
		"Title":          "Dashboard",
		"Version":        "v0.2.0",
		"TsSummary":      tsSummary,
		"StateFormatted": tailscale.FormatBackendState(tsSummary.BackendState),
		"RelayCount":     relayCount,
		"ProxyCount":     proxyCount,
	}

	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		log.Printf("Error rendering dashboard: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// APIStatus returns system status as JSON
func (h *DashboardHandler) APIStatus(w http.ResponseWriter, r *http.Request) {
	tsSummary, _ := h.tsClient.GetStatusSummary()

	relays, _ := config.LoadSocatRelays(h.cfg.Paths.SocatRelayConfig)
	proxies, _ := config.LoadCaddyProxies(h.cfg.Paths.CaddyProxyConfig)

	relayCount := 0
	if relays != nil {
		relayCount = len(relays.Relays)
	}

	proxyCount := 0
	if proxies != nil {
		proxyCount = len(proxies.Proxies)
	}

	status := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "v0.2.0",
		"services": map[string]interface{}{
			"webui": "running",
			"tailscale": map[string]interface{}{
				"connected": tsSummary.Connected,
				"state":     tsSummary.BackendState,
			},
			"caddy": map[string]interface{}{
				"proxies": proxyCount,
			},
			"socat": map[string]interface{}{
				"relays": relayCount,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
