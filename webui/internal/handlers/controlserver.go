package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// controlServer returns the currently persisted control server URL,
// guarded by cfgMu since cfg is a pointer shared across handlers.
func (h *TailscaleHandler) controlServer() string {
	h.cfgMu.Lock()
	defer h.cfgMu.Unlock()
	return h.cfg.Tailscale.ControlServer
}

// GetControlServer returns the persisted custom control server URL used for
// `tailscale login`/`up --authkey` (e.g. a self-hosted Headscale instance).
// An empty value means Tailscale's default control plane.
func (h *TailscaleHandler) GetControlServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, map[string]string{
		"control_server": h.controlServer(),
	})
}

// UpdateControlServer validates and persists a custom control server URL to
// webui.yaml. It takes effect on the next `tailscale login`/auth-key
// connection; it has no effect on a device that's already registered to a
// control server until it's logged out and re-authenticated. Pass an empty
// string to reset to Tailscale's default control plane.
func (h *TailscaleHandler) UpdateControlServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ControlServer string `json:"control_server"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	body.ControlServer = strings.TrimSpace(body.ControlServer)
	if err := tailscale.ValidateControlServerURL(body.ControlServer); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.cfgMu.Lock()
	previous := h.cfg.Tailscale.ControlServer
	h.cfg.Tailscale.ControlServer = body.ControlServer
	err := config.Save(h.cfg.ConfigFile, h.cfg)
	if err != nil {
		// Keep in-memory state consistent with what's actually on disk.
		h.cfg.Tailscale.ControlServer = previous
	}
	h.cfgMu.Unlock()

	if err != nil {
		log.Printf("Error saving control server setting: %v", err)
		writeJSONError(w, "Failed to save control server setting", http.StatusInternalServerError)
		return
	}

	message := "Control server updated"
	if body.ControlServer == "" {
		message = "Control server reset to Tailscale's default"
	}
	writeJSON(w, map[string]string{
		"status":  "success",
		"message": message,
	})
}
