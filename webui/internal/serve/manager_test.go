package serve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
)

func TestManagerUpsertAndReconcile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "commands.log")
	tailscaleScript := filepath.Join(dir, "tailscale")
	relayFile := filepath.Join(dir, "serve_relays.json")

	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + logFile + "\"\n" +
		"exit 0\n"
	if err := os.WriteFile(tailscaleScript, []byte(script), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript

	err := m.UpsertRelay(config.ServeRelay{
		ID:         "tcp-1",
		Type:       "tcp",
		ListenPort: 10001,
		TargetHost: "whoami-test",
		TargetPort: 80,
		Enabled:    true,
		Autostart:  true,
	})
	if err != nil {
		t.Fatalf("upsert relay failed: %v", err)
	}

	// Add HTTPS relay and ensure another reconcile happens.
	err = m.UpsertRelay(config.ServeRelay{
		ID:         "https-1",
		Type:       "https",
		ListenPort: 10002,
		TargetHost: "whoami-test",
		TargetPort: 80,
		Enabled:    true,
		Autostart:  true,
	})
	if err != nil {
		t.Fatalf("upsert https relay failed: %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "serve reset") {
		t.Fatalf("expected serve reset command, got logs:\n%s", out)
	}
	if !strings.Contains(out, "serve --bg --tcp 10001 tcp://whoami-test:80") {
		t.Fatalf("expected tcp serve command, got logs:\n%s", out)
	}
	if !strings.Contains(out, "serve --bg --https 10002 http://whoami-test:80") {
		t.Fatalf("expected https serve command, got logs:\n%s", out)
	}
}

// TestReconcileOnlyAppliesEnabledRelays verifies that Reconcile applies relays
// with Enabled=true and ignores relays where only Autostart=true.
func TestReconcileOnlyAppliesEnabledRelays(t *testing.T) {
	dir := t.TempDir()
	tailscaleScript := filepath.Join(dir, "tailscale")
	relayFile := filepath.Join(dir, "serve_relays.json")
	logFile := filepath.Join(dir, "commands.log")

	script := "#!/bin/sh\necho \"$@\" >> \"" + logFile + "\"\nexit 0\n"
	if err := os.WriteFile(tailscaleScript, []byte(script), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	// enabled=true, autostart=false — should be applied by Reconcile
	// enabled=false, autostart=true — should NOT be applied by Reconcile
	if err := config.SaveServeRelays(relayFile, &config.ServeRelayList{
		Relays: []config.ServeRelay{
			{ID: "tcp-enabled", Type: "tcp", ListenPort: 11001, TargetHost: "host-a", TargetPort: 80, Enabled: true, Autostart: false},
			{ID: "tcp-autostart-only", Type: "tcp", ListenPort: 11002, TargetHost: "host-b", TargetPort: 80, Enabled: false, Autostart: true},
		},
	}); err != nil {
		t.Fatalf("save relays: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript
	if err := m.Reconcile(); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "serve --bg --tcp 11001 tcp://host-a:80") {
		t.Fatalf("expected enabled relay to be applied, got:\n%s", out)
	}
	if strings.Contains(out, "tcp://host-b:80") {
		t.Fatalf("expected autostart-only relay to NOT be applied by Reconcile, got:\n%s", out)
	}
}

// TestReconcileAutostartOnlyAppliesAutostartRelays verifies that
// ReconcileAutostart applies relays with Autostart=true and ignores relays
// where only Enabled=true.
func TestReconcileAutostartOnlyAppliesAutostartRelays(t *testing.T) {
	dir := t.TempDir()
	tailscaleScript := filepath.Join(dir, "tailscale")
	relayFile := filepath.Join(dir, "serve_relays.json")
	logFile := filepath.Join(dir, "commands.log")

	script := "#!/bin/sh\necho \"$@\" >> \"" + logFile + "\"\nexit 0\n"
	if err := os.WriteFile(tailscaleScript, []byte(script), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	// enabled=false, autostart=true — should be applied by ReconcileAutostart
	// enabled=true, autostart=false — should NOT be applied by ReconcileAutostart
	if err := config.SaveServeRelays(relayFile, &config.ServeRelayList{
		Relays: []config.ServeRelay{
			{ID: "tcp-autostart", Type: "tcp", ListenPort: 12001, TargetHost: "host-c", TargetPort: 80, Enabled: false, Autostart: true},
			{ID: "tcp-enabled-only", Type: "tcp", ListenPort: 12002, TargetHost: "host-d", TargetPort: 80, Enabled: true, Autostart: false},
		},
	}); err != nil {
		t.Fatalf("save relays: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript
	if err := m.ReconcileAutostart(); err != nil {
		t.Fatalf("reconcile autostart failed: %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "serve --bg --tcp 12001 tcp://host-c:80") {
		t.Fatalf("expected autostart relay to be applied, got:\n%s", out)
	}
	if strings.Contains(out, "tcp://host-d:80") {
		t.Fatalf("expected enabled-only relay to NOT be applied by ReconcileAutostart, got:\n%s", out)
	}
}

func TestManagerToggleRelay(t *testing.T) {
	dir := t.TempDir()
	tailscaleScript := filepath.Join(dir, "tailscale")
	relayFile := filepath.Join(dir, "serve_relays.json")
	logFile := filepath.Join(dir, "commands.log")

	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + logFile + "\"\n" +
		"exit 0\n"
	if err := os.WriteFile(tailscaleScript, []byte(script), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	if err := config.SaveServeRelays(relayFile, &config.ServeRelayList{
		Relays: []config.ServeRelay{{
			ID:         "tcp-1",
			Type:       "tcp",
			ListenPort: 10001,
			TargetHost: "whoami-test",
			TargetPort: 80,
			Enabled:    true,
			Autostart:  true,
		}},
	}); err != nil {
		t.Fatalf("save seed relay: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript
	if err := m.ToggleRelay("tcp-1", false); err != nil {
		t.Fatalf("toggle relay failed: %v", err)
	}

	relay, err := m.GetRelay("tcp-1")
	if err != nil {
		t.Fatalf("get relay failed: %v", err)
	}
	if relay.Enabled {
		t.Fatalf("expected relay to be disabled")
	}
}
