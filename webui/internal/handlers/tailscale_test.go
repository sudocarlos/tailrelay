package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// newLoginWithKeyTestHandler builds a TailscaleHandler backed by both a fake
// `tailscale` binary (so LoginWithAuthKey doesn't shell out for real) and a
// temp webui.yaml config file (since LoginWithKey reads the persisted
// control server via h.controlServer(), which requires cfg to be non-nil).
func newLoginWithKeyTestHandler(t *testing.T, exitCode int) (*TailscaleHandler, string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "tailscale")
	logFile := filepath.Join(dir, "commands.log")

	script := "#!/bin/sh\necho \"$@\" >> \"" + logFile + "\"\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake tailscale script: %v", err)
	}

	configFile := filepath.Join(dir, "webui.yaml")
	cfg := &config.Config{ConfigFile: configFile}
	if err := config.Save(configFile, cfg); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	return &TailscaleHandler{cfg: cfg, tsClient: tailscale.NewClientWithBinary(scriptPath)}, logFile
}

// --- LoginWithKey ---

func TestLoginWithKey_RejectsNonPOST(t *testing.T) {
	h, _ := newLoginWithKeyTestHandler(t, 0)
	req := httptest.NewRequest(http.MethodGet, "/api/tailscale/login-with-key", nil)
	rr := httptest.NewRecorder()
	h.LoginWithKey(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestLoginWithKey_RejectsEmptyKey(t *testing.T) {
	h, _ := newLoginWithKeyTestHandler(t, 0)
	rr := postJSON(t, h.LoginWithKey, "/api/tailscale/login-with-key", map[string]interface{}{
		"auth_key": "   ",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestLoginWithKey_RejectsInvalidPrefix(t *testing.T) {
	h, _ := newLoginWithKeyTestHandler(t, 0)
	rr := postJSON(t, h.LoginWithKey, "/api/tailscale/login-with-key", map[string]interface{}{
		"auth_key": "not-a-valid-key",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestLoginWithKey_AcceptsTskeyPrefix(t *testing.T) {
	h, logFile := newLoginWithKeyTestHandler(t, 0)
	rr := postJSON(t, h.LoginWithKey, "/api/tailscale/login-with-key", map[string]interface{}{
		"auth_key": "tskey-auth-abc123",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if got := readLog(t, logFile); got == "" {
		t.Error("expected `tailscale up` to be invoked")
	}
}

func TestLoginWithKey_AcceptsHskeyPrefix(t *testing.T) {
	h, logFile := newLoginWithKeyTestHandler(t, 0)
	rr := postJSON(t, h.LoginWithKey, "/api/tailscale/login-with-key", map[string]interface{}{
		"auth_key": "hskey-abc123",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if got := readLog(t, logFile); got == "" {
		t.Error("expected `tailscale up` to be invoked")
	}
}

// --- ChangeHostname ---

func TestChangeHostname_RejectsNonPOST(t *testing.T) {
	h, _ := newLoginWithKeyTestHandler(t, 0)
	req := httptest.NewRequest(http.MethodGet, "/api/tailscale/hostname", nil)
	rr := httptest.NewRecorder()
	h.ChangeHostname(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestChangeHostname_RejectsEmptyHostname(t *testing.T) {
	h, _ := newLoginWithKeyTestHandler(t, 0)
	rr := postJSON(t, h.ChangeHostname, "/api/tailscale/hostname", map[string]interface{}{
		"hostname": "   ",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestChangeHostname_NoControlServerOmitsLoginServer(t *testing.T) {
	h, logFile := newLoginWithKeyTestHandler(t, 0)
	rr := postJSON(t, h.ChangeHostname, "/api/tailscale/hostname", map[string]interface{}{
		"hostname": "my-device",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	got := readLog(t, logFile)
	if !strings.Contains(got, "--hostname=my-device") {
		t.Errorf("expected `tailscale up --hostname=my-device`, got %q", got)
	}
	if strings.Contains(got, "--login-server") {
		t.Errorf("expected no --login-server without a configured control server, got %q", got)
	}
}

// TestChangeHostname_PassesLoginServerWhenControlServerSet is a regression
// test: `tailscale up --reset` resets ControlURL to Tailscale's default
// control plane unless --login-server is re-specified, so ChangeHostname
// must pass the persisted control server through on every hostname change
// or a device registered to a self-hosted Headscale instance gets detached
// from it.
func TestChangeHostname_PassesLoginServerWhenControlServerSet(t *testing.T) {
	h, logFile := newLoginWithKeyTestHandler(t, 0)
	h.cfg.Tailscale.ControlServer = "https://headscale.example.com"

	rr := postJSON(t, h.ChangeHostname, "/api/tailscale/hostname", map[string]interface{}{
		"hostname": "my-device",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	got := readLog(t, logFile)
	if !strings.Contains(got, "--login-server=https://headscale.example.com") {
		t.Errorf("expected `--login-server=https://headscale.example.com`, got %q", got)
	}
}
