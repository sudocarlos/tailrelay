package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/serve"
	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// newFakeTailscaleScript writes a fake `tailscale` binary that appends every
// invocation to a log file and exits 0, unless failOnFunnel is true, in which
// case any invocation with "funnel" as the first argument fails with output
// matching the funnel-not-allowed error patterns checked by the serve
// package's isFunnelNotAllowed helper.
func newFakeTailscaleScript(t *testing.T, dir string, failOnFunnel bool) string {
	t.Helper()
	scriptPath := filepath.Join(dir, "tailscale")
	logFile := filepath.Join(dir, "commands.log")

	script := "#!/bin/sh\n"
	if failOnFunnel {
		script += "if [ \"$1\" = \"funnel\" ]; then\n" +
			"  echo \"$@\" >> \"" + logFile + "\"\n" +
			"  echo \"Funnel is not enabled for this device; requires funnel node attribute\" >&2\n" +
			"  exit 1\n" +
			"fi\n"
	}
	script += "echo \"$@\" >> \"" + logFile + "\"\nexit 0\n"

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake tailscale script: %v", err)
	}
	return scriptPath
}

// newTestServeHandler builds a ServeHandler backed by a temp relay file and a
// fake `tailscale` binary, bypassing NewServeHandler so tests never shell out
// to a real tailscale installation.
func newTestServeHandler(t *testing.T, failOnFunnel bool) *ServeHandler {
	t.Helper()
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	scriptPath := newFakeTailscaleScript(t, dir, failOnFunnel)

	return &ServeHandler{
		cfg: &config.Config{
			Paths: config.PathsConfig{ServeRelayConfig: relayFile},
		},
		manager:  serve.NewManagerWithBinary(relayFile, scriptPath),
		tsClient: tailscale.NewClient(),
	}
}

func funnelCreateRequest(t *testing.T, body interface{}) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/serve/funnel/create", jsonBody(t, body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// --- CreateFunnel ---

func TestCreateFunnel_RejectsDisallowedPort(t *testing.T) {
	h := newTestServeHandler(t, false)
	req := funnelCreateRequest(t, map[string]interface{}{
		"listen_port": 9999,
		"target_host": "127.0.0.1",
		"target_port": 3000,
	})
	rr := httptest.NewRecorder()
	h.CreateFunnel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for disallowed funnel port, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestCreateFunnel_RejectsMissingFields(t *testing.T) {
	h := newTestServeHandler(t, false)
	req := funnelCreateRequest(t, map[string]interface{}{
		"listen_port": 443,
	})
	rr := httptest.NewRecorder()
	h.CreateFunnel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing target fields, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestCreateFunnel_Success(t *testing.T) {
	h := newTestServeHandler(t, false)
	req := funnelCreateRequest(t, map[string]interface{}{
		"id":               "funnel-443",
		"listen_port":      443,
		"target_host":      "127.0.0.1",
		"target_port":      3000,
		"funnel_transport": "https",
		"enabled":          true,
	})
	rr := httptest.NewRecorder()
	h.CreateFunnel(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	relays, err := h.manager.ListRelays()
	if err != nil {
		t.Fatalf("list relays failed: %v", err)
	}
	if len(relays) != 1 || relays[0].ID != "funnel-443" || relays[0].Type != "funnel" {
		t.Fatalf("expected persisted funnel relay funnel-443, got: %+v", relays)
	}
}

func TestCreateFunnel_ReturnsFunnelNotAllowedAs409(t *testing.T) {
	h := newTestServeHandler(t, true)
	req := funnelCreateRequest(t, map[string]interface{}{
		"id":          "funnel-443",
		"listen_port": 443,
		"target_host": "127.0.0.1",
		"target_port": 3000,
		"enabled":     true,
	})
	rr := httptest.NewRecorder()
	h.CreateFunnel(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 when funnel is not allowed, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp["status"] != "error" {
		t.Errorf("expected status=error, got %q", resp["status"])
	}
}

// --- APIListFunnel ---

func TestAPIListFunnel_ExcludesOtherRelayTypes(t *testing.T) {
	h := newTestServeHandler(t, false)

	// Seed a tcp relay and a funnel relay directly via the manager.
	if err := h.manager.UpsertRelay(config.ServeRelay{
		Type:       "tcp",
		ListenPort: 9000,
		TargetHost: "127.0.0.1",
		TargetPort: 22,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("seed tcp relay failed: %v", err)
	}
	if err := h.manager.UpsertRelay(config.ServeRelay{
		ID:         "funnel-10000",
		Type:       "funnel",
		ListenPort: 10000,
		TargetHost: "127.0.0.1",
		TargetPort: 22,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("seed funnel relay failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/serve/funnel/list", nil)
	rr := httptest.NewRecorder()
	h.APIListFunnel(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var out []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("response is not a JSON array: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected exactly one funnel relay in list, got %d: %+v", len(out), out)
	}
	if out[0]["id"] != "funnel-10000" {
		t.Errorf("expected funnel-10000, got %v", out[0]["id"])
	}
}

// --- DeleteFunnel ---

func TestDeleteFunnel_RequiresID(t *testing.T) {
	h := newTestServeHandler(t, false)
	req := httptest.NewRequest(http.MethodPost, "/api/serve/funnel/delete", nil)
	rr := httptest.NewRecorder()
	h.DeleteFunnel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when id is missing, got %d", rr.Code)
	}
}

func TestDeleteFunnel_NotFoundReturns404(t *testing.T) {
	h := newTestServeHandler(t, false)
	req := httptest.NewRequest(http.MethodPost, "/api/serve/funnel/delete?id=does-not-exist", nil)
	rr := httptest.NewRecorder()
	h.DeleteFunnel(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown funnel id, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
