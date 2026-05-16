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
