package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// newControlServerTestHandler builds a TailscaleHandler backed by a temp
// webui.yaml config file, so UpdateControlServer's config.Save round-trips
// against a real file the same way it does in production.
func newControlServerTestHandler(t *testing.T) *TailscaleHandler {
	t.Helper()
	dir := t.TempDir()
	configFile := filepath.Join(dir, "webui.yaml")
	cfg := &config.Config{ConfigFile: configFile}
	if err := config.Save(configFile, cfg); err != nil {
		t.Fatalf("seed config file: %v", err)
	}
	return &TailscaleHandler{cfg: cfg, tsClient: tailscale.NewClient()}
}

// --- GetControlServer ---

func TestGetControlServer_RejectsNonGET(t *testing.T) {
	h := newControlServerTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/tailscale/control-server", nil)
	rr := httptest.NewRecorder()
	h.GetControlServer(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestGetControlServer_DefaultsEmpty(t *testing.T) {
	h := newControlServerTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/tailscale/control-server", nil)
	rr := httptest.NewRecorder()
	h.GetControlServer(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var body struct {
		ControlServer string `json:"control_server"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ControlServer != "" {
		t.Errorf("expected empty control_server by default, got %q", body.ControlServer)
	}
}

// --- UpdateControlServer ---

func TestUpdateControlServer_RejectsNonPOST(t *testing.T) {
	h := newControlServerTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/tailscale/control-server/update", nil)
	rr := httptest.NewRecorder()
	h.UpdateControlServer(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateControlServer_RejectsInvalidURL(t *testing.T) {
	h := newControlServerTestHandler(t)
	rr := postJSON(t, h.UpdateControlServer, "/api/tailscale/control-server/update", map[string]interface{}{
		"control_server": "not a url",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid URL, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if h.cfg.Tailscale.ControlServer != "" {
		t.Errorf("expected config to be untouched after a rejected update, got %q", h.cfg.Tailscale.ControlServer)
	}
}

func TestUpdateControlServer_PersistsAndRoundTrips(t *testing.T) {
	h := newControlServerTestHandler(t)
	rr := postJSON(t, h.UpdateControlServer, "/api/tailscale/control-server/update", map[string]interface{}{
		"control_server": "https://headscale.example.com",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if h.cfg.Tailscale.ControlServer != "https://headscale.example.com" {
		t.Errorf("expected in-memory config to be updated, got %q", h.cfg.Tailscale.ControlServer)
	}

	// Confirm it was actually written to disk, not just held in memory.
	reloaded, err := config.Load(h.cfg.ConfigFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Tailscale.ControlServer != "https://headscale.example.com" {
		t.Errorf("expected persisted control_server, got %q", reloaded.Tailscale.ControlServer)
	}

	// GET should reflect the same persisted value.
	getReq := httptest.NewRequest(http.MethodGet, "/api/tailscale/control-server", nil)
	getRR := httptest.NewRecorder()
	h.GetControlServer(getRR, getReq)
	var body struct {
		ControlServer string `json:"control_server"`
	}
	if err := json.NewDecoder(getRR.Body).Decode(&body); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if body.ControlServer != "https://headscale.example.com" {
		t.Errorf("expected GET to return persisted value, got %q", body.ControlServer)
	}
}

func TestUpdateControlServer_EmptyResetsToDefault(t *testing.T) {
	h := newControlServerTestHandler(t)
	h.cfg.Tailscale.ControlServer = "https://headscale.example.com"

	rr := postJSON(t, h.UpdateControlServer, "/api/tailscale/control-server/update", map[string]interface{}{
		"control_server": "",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if h.cfg.Tailscale.ControlServer != "" {
		t.Errorf("expected control server to be cleared, got %q", h.cfg.Tailscale.ControlServer)
	}
}

func TestUpdateControlServer_RejectsInvalidJSON(t *testing.T) {
	h := newControlServerTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/tailscale/control-server/update", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.UpdateControlServer(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON body, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
