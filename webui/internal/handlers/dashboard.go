package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/serve"
	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// DashboardHandler handles dashboard-related requests
type DashboardHandler struct {
	cfg       *config.Config
	templates *template.Template
	serveMgr  *serve.Manager
	tsClient  *tailscale.Client
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(cfg *config.Config, templates *template.Template) *DashboardHandler {
	return &DashboardHandler{
		cfg:       cfg,
		templates: templates,
		serveMgr:  serve.NewManager(cfg.Paths.ServeRelayConfig),
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
	serveRelays, err := h.serveMgr.ListRelays()
	if err != nil {
		log.Printf("Error loading relays: %v", err)
		serveRelays = []config.ServeRelay{}
	}

	relayCount := 0
	proxyCount := 0
	for _, relay := range serveRelays {
		if relay.Enabled {
			if relay.Type == "tcp" {
				relayCount++
			} else if relay.Type == "https" {
				proxyCount++
			}
		}
	}

	data := map[string]interface{}{
		"Title":          "Dashboard",
		"Version":        "v0.6.0",
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

	serveRelays, err := h.serveMgr.ListRelays()
	if err != nil {
		log.Printf("Error loading relays: %v", err)
		serveRelays = []config.ServeRelay{}
	}

	relayCount := 0
	proxyCount := 0
	for _, relay := range serveRelays {
		if relay.Type == "tcp" {
			relayCount++
		} else if relay.Type == "https" {
			proxyCount++
		}
	}

	status := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "v0.6.0",
		"services": map[string]interface{}{
			"webui": "running",
			"tailscale": map[string]interface{}{
				"connected": tsSummary.Connected,
				"state":     tsSummary.BackendState,
			},
			"serve": map[string]interface{}{
				"https_relays": proxyCount,
				"tcp_relays":   relayCount,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
