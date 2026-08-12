package serve

import (
	"errors"
	"fmt"
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

// --- Icon URL ---

func TestManagerUpsertRelay_RoundTripsIconURL(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	script := "#!/bin/sh\nexit 0\n"
	tailscaleScript := filepath.Join(dir, "tailscale")
	if err := os.WriteFile(tailscaleScript, []byte(script), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}
	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript

	const icon = "https://cdn.example.com/app.png"
	if err := m.UpsertRelay(config.ServeRelay{
		ID:         "tcp-1",
		Type:       "tcp",
		ListenPort: 10001,
		TargetHost: "whoami-test",
		TargetPort: 80,
		Enabled:    true,
		IconURL:    icon,
	}); err != nil {
		t.Fatalf("upsert relay with icon failed: %v", err)
	}

	relays, err := m.ListRelays()
	if err != nil {
		t.Fatalf("list relays failed: %v", err)
	}
	if len(relays) != 1 || relays[0].IconURL != icon {
		t.Fatalf("expected icon %q to round-trip, got %+v", icon, relays)
	}
}

func TestManagerUpsertRelay_RejectsBadIconScheme(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	m := NewManager(relayFile)
	m.binaryPath = "/bin/true"
	for _, bad := range []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/plain;base64,aGVsbG8=",
	} {
		err := m.UpsertRelay(config.ServeRelay{
			ID:         "tcp-1",
			Type:       "tcp",
			ListenPort: 10001,
			TargetHost: "whoami-test",
			TargetPort: 80,
			IconURL:    bad,
		})
		if !errors.Is(err, ErrInvalidIconURL) {
			t.Fatalf("expected ErrInvalidIconURL for %q, got %v", bad, err)
		}
	}
}

func TestManagerUpsertRelay_AcceptsDataImageIcon(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	script := "#!/bin/sh\nexit 0\n"
	tailscaleScript := filepath.Join(dir, "tailscale")
	if err := os.WriteFile(tailscaleScript, []byte(script), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}
	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript
	const icon = "data:image/png;base64,iVBORw0KGgoAAAANS=="
	if err := m.UpsertRelay(config.ServeRelay{
		ID:         "tcp-1",
		Type:       "tcp",
		ListenPort: 10001,
		TargetHost: "whoami-test",
		TargetPort: 80,
		IconURL:    icon,
	}); err != nil {
		t.Fatalf("expected data:image/png to be accepted, got %v", err)
	}
}

// TestManagerUpsertRelay_OldConfigWithoutIconField ensures a serve_relays.json
// written before icon_url was introduced still loads (omitempty backward compat).
func TestManagerUpsertRelay_OldConfigWithoutIconField(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	old := `{"relays":[{"id":"tcp-1","type":"tcp","listen_port":10001,"target_host":"whoami-test","target_port":80,"enabled":true}]}`
	if err := os.WriteFile(relayFile, []byte(old), 0644); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	m := NewManager(relayFile)
	m.binaryPath = "/bin/true"
	relays, err := m.ListRelays()
	if err != nil {
		t.Fatalf("list relays failed: %v", err)
	}
	if len(relays) != 1 || relays[0].IconURL != "" {
		t.Fatalf("expected old config to load with empty icon, got %+v", relays)
	}
}

func TestManagerUpsertAndReconcileWithCustomControlServer(t *testing.T) {
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

	m := NewManagerWithCustomControlServer(relayFile, true)
	m.binaryPath = tailscaleScript
	if err := m.UpsertRelay(config.ServeRelay{
		ID:         "https-3333",
		Type:       "https",
		ListenPort: 3333,
		TargetHost: "whoami-test",
		TargetPort: 80,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("upsert web relay failed: %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "serve --bg --http 3333 http://whoami-test:80") {
		t.Fatalf("expected HTTP serve command, got logs:\n%s", out)
	}
	if strings.Contains(out, "--https 3333") {
		t.Fatalf("expected no HTTPS serve command, got logs:\n%s", out)
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

// TestManagerUpsertFunnelRelay verifies that funnel relays are applied via
// `tailscale funnel` with the correct transport flag.
func TestManagerUpsertFunnelRelay(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "commands.log")
	tailscaleScript := filepath.Join(dir, "tailscale")
	relayFile := filepath.Join(dir, "serve_relays.json")

	script := "#!/bin/sh\necho \"$@\" >> \"" + logFile + "\"\nexit 0\n"
	if err := os.WriteFile(tailscaleScript, []byte(script), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript

	err := m.UpsertRelay(config.ServeRelay{
		ID:              "funnel-443",
		Type:            "funnel",
		FunnelTransport: "https",
		ListenPort:      443,
		TargetHost:      "127.0.0.1",
		TargetPort:      3000,
		Enabled:         true,
		Autostart:       true,
	})
	if err != nil {
		t.Fatalf("upsert funnel relay failed: %v", err)
	}

	err = m.UpsertRelay(config.ServeRelay{
		ID:              "funnel-10000",
		Type:            "funnel",
		FunnelTransport: "tcp",
		ListenPort:      10000,
		TargetHost:      "127.0.0.1",
		TargetPort:      2222,
		Enabled:         true,
		Autostart:       true,
	})
	if err != nil {
		t.Fatalf("upsert tcp funnel relay failed: %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "funnel --bg --https 443 http://127.0.0.1:3000") {
		t.Fatalf("expected https funnel command, got logs:\n%s", out)
	}
	if !strings.Contains(out, "funnel --bg --tcp 10000 tcp://127.0.0.1:2222") {
		t.Fatalf("expected tcp funnel command, got logs:\n%s", out)
	}
}

// TestManagerUpsertFunnelRejectsInvalidPort verifies that only ports 443,
// 8443, and 10000 are accepted for funnel relays.
func TestManagerUpsertFunnelRejectsInvalidPort(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	tailscaleScript := filepath.Join(dir, "tailscale")
	if err := os.WriteFile(tailscaleScript, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript

	err := m.UpsertRelay(config.ServeRelay{
		Type:       "funnel",
		ListenPort: 8080,
		TargetHost: "127.0.0.1",
		TargetPort: 3000,
	})
	if err == nil {
		t.Fatal("expected error for disallowed funnel port, got nil")
	}
}

// TestManagerUpsertFunnelRejectsInvalidTransport verifies that only "https"
// and "tcp" are accepted funnel transports.
func TestManagerUpsertFunnelRejectsInvalidTransport(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	tailscaleScript := filepath.Join(dir, "tailscale")
	if err := os.WriteFile(tailscaleScript, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript

	err := m.UpsertRelay(config.ServeRelay{
		Type:            "funnel",
		FunnelTransport: "udp",
		ListenPort:      443,
		TargetHost:      "127.0.0.1",
		TargetPort:      3000,
	})
	if err == nil {
		t.Fatal("expected error for invalid funnel transport, got nil")
	}
}

// TestManagerUpsertDefaultIDUsesNormalizedType verifies that the auto-generated
// relay ID is derived from the normalized (defaulted, lowercased) Type rather
// than the raw input Type, so a caller that omits both ID and Type still gets
// a correctly prefixed ID (e.g. "tcp-443", not "-443").
func TestManagerUpsertDefaultIDUsesNormalizedType(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	tailscaleScript := filepath.Join(dir, "tailscale")
	if err := os.WriteFile(tailscaleScript, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript

	// Omit both ID and Type; Type should default to "tcp" before the ID is derived.
	if err := m.UpsertRelay(config.ServeRelay{
		ListenPort: 443,
		TargetHost: "127.0.0.1",
		TargetPort: 3000,
	}); err != nil {
		t.Fatalf("upsert relay failed: %v", err)
	}

	relays, err := m.ListRelays()
	if err != nil {
		t.Fatalf("list relays failed: %v", err)
	}
	if len(relays) != 1 {
		t.Fatalf("expected exactly one relay, got %d", len(relays))
	}
	if relays[0].ID != "tcp-443" {
		t.Fatalf("expected default ID %q, got %q", "tcp-443", relays[0].ID)
	}

	// Same check for an explicit Type with mixed case/whitespace.
	if err := m.UpsertRelay(config.ServeRelay{
		Type:       " FUNNEL ",
		ListenPort: 8443,
		TargetHost: "127.0.0.1",
		TargetPort: 3000,
	}); err != nil {
		t.Fatalf("upsert funnel relay failed: %v", err)
	}
	relays, err = m.ListRelays()
	if err != nil {
		t.Fatalf("list relays failed: %v", err)
	}
	ids := make([]string, 0, len(relays))
	for _, r := range relays {
		ids = append(ids, r.ID)
	}
	found := false
	for _, id := range ids {
		if id == "funnel-8443" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a relay with default ID %q, got ids: %v", "funnel-8443", ids)
	}
}

// TestFunnelRelaysExcludedFromTypedLists verifies that a funnel relay is
// distinguishable from tcp/https relays by Type, so callers filtering by
// Type=="tcp" or Type=="https" correctly exclude it.
func TestFunnelRelaysExcludedFromTypedLists(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")
	tailscaleScript := filepath.Join(dir, "tailscale")
	if err := os.WriteFile(tailscaleScript, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write tailscale script: %v", err)
	}

	m := NewManager(relayFile)
	m.binaryPath = tailscaleScript

	if err := m.UpsertRelay(config.ServeRelay{
		ID:              "funnel-443",
		Type:            "funnel",
		FunnelTransport: "https",
		ListenPort:      443,
		TargetHost:      "127.0.0.1",
		TargetPort:      3000,
		Enabled:         true,
	}); err != nil {
		t.Fatalf("upsert funnel relay failed: %v", err)
	}

	relays, err := m.ListRelays()
	if err != nil {
		t.Fatalf("list relays failed: %v", err)
	}

	tcpCount, httpsCount, funnelCount := 0, 0, 0
	for _, r := range relays {
		switch r.Type {
		case "tcp":
			tcpCount++
		case "https":
			httpsCount++
		case "funnel":
			funnelCount++
		}
	}
	if tcpCount != 0 || httpsCount != 0 {
		t.Fatalf("expected funnel relay to not be counted as tcp/https, got tcp=%d https=%d", tcpCount, httpsCount)
	}
	if funnelCount != 1 {
		t.Fatalf("expected exactly one funnel relay, got %d", funnelCount)
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

// fakeControlServerChecker implements controlServerChecker for tests, so
// live control server detection can be exercised without a real tailscaled.
type fakeControlServerChecker struct {
	custom bool
	err    error
}

func (f fakeControlServerChecker) IsCustomControlServer() (bool, error) {
	return f.custom, f.err
}

func TestWebListenerSchemePrefersLiveDetectionOverFallback(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")

	// Fallback says "default Tailscale" (https), but the live checker
	// reports the node is actually authenticated against a custom control
	// server — the live result must win, since a node authenticated outside
	// the Web UI (CLI login, restored state) would otherwise be missed.
	m := NewManagerWithControlServerDetection(relayFile, fakeControlServerChecker{custom: true}, false)
	if got := m.WebListenerScheme(); got != "http" {
		t.Fatalf("WebListenerScheme() = %q, want %q", got, "http")
	}
}

func TestWebListenerSchemeFallsBackWhenLiveCheckFails(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")

	// If the live checker errors (e.g. tailscaled unreachable), the manager
	// should fall back to the persisted/seed value instead of defaulting to
	// https, which would fail reconcile against a custom control server.
	m := NewManagerWithControlServerDetection(relayFile, fakeControlServerChecker{err: fmt.Errorf("tailscaled unreachable")}, true)
	if got := m.WebListenerScheme(); got != "http" {
		t.Fatalf("WebListenerScheme() = %q, want %q", got, "http")
	}
}

func TestWebListenerSchemeWithNoChecker(t *testing.T) {
	dir := t.TempDir()
	relayFile := filepath.Join(dir, "serve_relays.json")

	// NewManager/NewManagerWithBinary have no checker configured; the
	// scheme should simply reflect the fallback flag (default false/https).
	m := NewManager(relayFile)
	if got := m.WebListenerScheme(); got != "https" {
		t.Fatalf("WebListenerScheme() = %q, want %q", got, "https")
	}
}
