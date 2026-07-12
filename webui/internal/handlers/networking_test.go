package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// newNetworkingTestHandler builds a TailscaleHandler backed by a fake
// `tailscale` binary that appends every invocation to a log file and exits
// with exitCode. Returns the handler and the log file path.
func newNetworkingTestHandler(t *testing.T, exitCode int) (*TailscaleHandler, string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "tailscale")
	logFile := filepath.Join(dir, "commands.log")

	script := "#!/bin/sh\necho \"$@\" >> \"" + logFile + "\"\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake tailscale script: %v", err)
	}

	return &TailscaleHandler{tsClient: tailscale.NewClientWithBinary(scriptPath)}, logFile
}

func readLog(t *testing.T, logFile string) string {
	t.Helper()
	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read log file: %v", err)
	}
	return string(data)
}

// --- APINetworking ---

func TestAPINetworking_RejectsNonGET(t *testing.T) {
	h, _ := newNetworkingTestHandler(t, 0)
	req := httptest.NewRequest(http.MethodPost, "/api/tailscale/networking", nil)
	rr := httptest.NewRecorder()
	h.APINetworking(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// --- UpdateNetworking ---

func TestUpdateNetworking_RejectsNonPOST(t *testing.T) {
	h, _ := newNetworkingTestHandler(t, 0)
	req := httptest.NewRequest(http.MethodGet, "/api/tailscale/networking/update", nil)
	rr := httptest.NewRecorder()
	h.UpdateNetworking(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateNetworking_RejectsInvalidJSON(t *testing.T) {
	h, _ := newNetworkingTestHandler(t, 0)
	req := httptest.NewRequest(http.MethodPost, "/api/tailscale/networking/update", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.UpdateNetworking(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON body, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateNetworking_RejectsInvalidRoute(t *testing.T) {
	h, logFile := newNetworkingTestHandler(t, 0)
	rr := postJSON(t, h.UpdateNetworking, "/api/tailscale/networking/update", map[string]interface{}{
		"advertise_routes": []string{"0.0.0.0/0"},
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for reserved exit-node CIDR, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if log := readLog(t, logFile); log != "" {
		t.Fatalf("expected `tailscale set` to never run on validation failure, but got: %q", log)
	}
}

func TestUpdateNetworking_Success(t *testing.T) {
	h, logFile := newNetworkingTestHandler(t, 0)
	rr := postJSON(t, h.UpdateNetworking, "/api/tailscale/networking/update", map[string]interface{}{
		"advertise_exit_node": true,
		"ssh":                 true,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	log := readLog(t, logFile)
	if !strings.Contains(log, "--advertise-exit-node=true") || !strings.Contains(log, "--ssh=true") {
		t.Fatalf("expected tailscale set to receive both flags, got: %q", log)
	}
}

func TestUpdateNetworking_ReportsCLIFailure(t *testing.T) {
	h, _ := newNetworkingTestHandler(t, 1)
	rr := postJSON(t, h.UpdateNetworking, "/api/tailscale/networking/update", map[string]interface{}{
		"ssh": true,
	})

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when the CLI invocation fails, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
