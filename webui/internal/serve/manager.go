package serve

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/logger"
)

// Manager manages relay rules backed by `tailscale serve`.
type Manager struct {
	binaryPath string
	relayFile  string
}

// NewManager creates a new serve manager.
func NewManager(relayFile string) *Manager {
	return &Manager{
		binaryPath: "tailscale",
		relayFile:  relayFile,
	}
}

// ServeStatusJSON is the structure returned by `tailscale serve status --json`
type ServeStatusJSON struct {
	TCP map[string]struct {
		HTTPS      bool   `json:"HTTPS,omitempty"`
		TCPForward string `json:"TCPForward,omitempty"`
	} `json:"TCP"`
	Web map[string]struct {
		Handlers map[string]struct {
			Proxy string `json:"Proxy,omitempty"`
		} `json:"Handlers"`
	} `json:"Web"`
}

// Status returns the parsed output of `tailscale serve status --json`
func (m *Manager) Status() (*ServeStatusJSON, error) {
	cmd := exec.Command(m.binaryPath, "serve", "status", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tailscale serve status failed: %w", err)
	}
	var status ServeStatusJSON
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// RelayStatus is a UI status shape similar to legacy relay status APIs.
type RelayStatus struct {
	Relay   config.ServeRelay `json:"relay"`
	Running bool              `json:"running"`
}

// ListRelays returns all stored relay definitions.
func (m *Manager) ListRelays() ([]config.ServeRelay, error) {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return nil, err
	}
	return list.Relays, nil
}

// GetRelay gets a relay by ID.
func (m *Manager) GetRelay(id string) (*config.ServeRelay, error) {
	relays, err := m.ListRelays()
	if err != nil {
		return nil, err
	}
	for i := range relays {
		if relays[i].ID == id {
			return &relays[i], nil
		}
	}
	return nil, fmt.Errorf("relay not found")
}

// UpsertRelay creates or updates a relay and reconciles tailscale serve state.
func (m *Manager) UpsertRelay(relay config.ServeRelay) error {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return err
	}

	if relay.ID == "" {
		relay.ID = fmt.Sprintf("%s-%d", relay.Type, relay.ListenPort)
	}
	if relay.Type == "" {
		relay.Type = "tcp"
	}
	relay.Type = strings.ToLower(strings.TrimSpace(relay.Type))
	if relay.Type != "https" && relay.Type != "tcp" {
		return fmt.Errorf("relay type must be https or tcp")
	}

	if relay.ListenPort < 1 || relay.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be between 1 and 65535")
	}
	if relay.TargetPort < 1 || relay.TargetPort > 65535 {
		return fmt.Errorf("target_port must be between 1 and 65535")
	}
	if strings.TrimSpace(relay.TargetHost) == "" {
		return fmt.Errorf("target_host is required")
	}

	updated := false
	for i := range list.Relays {
		if list.Relays[i].ID == relay.ID {
			list.Relays[i] = relay
			updated = true
			break
		}
	}
	if !updated {
		list.Relays = append(list.Relays, relay)
	}

	if err := config.SaveServeRelays(m.relayFile, list); err != nil {
		return err
	}
	return m.Reconcile()
}

// DeleteRelay removes a relay and reconciles tailscale serve state.
func (m *Manager) DeleteRelay(id string) error {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return err
	}
	filtered := make([]config.ServeRelay, 0, len(list.Relays))
	found := false
	for _, r := range list.Relays {
		if r.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}
	if !found {
		return ErrRelayNotFound
	}
	list.Relays = filtered
	if err := config.SaveServeRelays(m.relayFile, list); err != nil {
		return err
	}
	return m.Reconcile()
}

// ToggleRelay toggles a relay enabled state and reconciles tailscale serve state.
func (m *Manager) ToggleRelay(id string, enabled bool) error {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return err
	}
	found := false
	for i := range list.Relays {
		if list.Relays[i].ID == id {
			list.Relays[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		return ErrRelayNotFound
	}
	if err := config.SaveServeRelays(m.relayFile, list); err != nil {
		return err
	}
	return m.Reconcile()
}

// ErrTailscaleNotReady is returned by Reconcile when Tailscale is not yet
// authenticated or connected. Callers can use errors.Is to distinguish a
// deferred reconcile from a real failure.
var ErrTailscaleNotReady = fmt.Errorf("tailscale not ready")

// ErrRelayNotFound is returned by DeleteRelay and ToggleRelay when no relay
// with the given ID exists in the config file.
var ErrRelayNotFound = fmt.Errorf("relay not found")

// isTailscaleNotReady reports whether err indicates Tailscale is not yet
// authenticated or connected (e.g. during container startup in CI).
func isTailscaleNotReady(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "netMap is nil") ||
		strings.Contains(s, "not logged in") ||
		strings.Contains(s, "NeedsLogin") ||
		strings.Contains(s, "connect: no such file or directory")
}

// Reconcile resets tailscale serve config and reapplies all enabled relays.
// If Tailscale is not yet authenticated or connected the reconcile is skipped
// with a warning — relay config is already persisted and will be applied on
// the next successful reconcile (e.g. after Tailscale authenticates).
func (m *Manager) Reconcile() error {
	logger.Debug("serve", "Starting serve reconcile process")
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		logger.Error("serve", "Failed to load serve relays: %v", err)
		return err
	}

	active := make([]config.ServeRelay, 0, len(list.Relays))
	for _, r := range list.Relays {
		if r.Enabled {
			active = append(active, r)
		}
	}
	return m.resetAndApply(active)
}

// ReconcileAutostart resets tailscale serve config and reapplies all relays
// that have Autostart=true. Intended to be called once at container startup so
// that relays marked for autostart are brought up regardless of their last
// runtime Enabled state.
// If Tailscale is not yet authenticated or connected the reconcile is skipped —
// relay config is already persisted and will be applied on the next successful
// reconcile (e.g. after Tailscale authenticates).
func (m *Manager) ReconcileAutostart() error {
	logger.Debug("serve", "Starting autostart reconcile process")
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		logger.Error("serve", "Failed to load serve relays: %v", err)
		return err
	}

	active := make([]config.ServeRelay, 0, len(list.Relays))
	for _, r := range list.Relays {
		if r.Autostart {
			active = append(active, r)
		}
	}
	return m.resetAndApply(active)
}

// resetAndApply resets the tailscale serve config and applies the given relays.
func (m *Manager) resetAndApply(relays []config.ServeRelay) error {
	logger.Debug("serve", "Resetting tailscale serve config")
	if err := m.run("serve", "reset"); err != nil {
		if isTailscaleNotReady(err) {
			logger.Debug("serve", "Tailscale not ready, deferring reconcile: %v", err)
			return ErrTailscaleNotReady
		}
		logger.Error("serve", "tailscale serve reset failed: %v", err)
		return err
	}

	sort.Slice(relays, func(i, j int) bool {
		if relays[i].ListenPort == relays[j].ListenPort {
			return relays[i].ID < relays[j].ID
		}
		return relays[i].ListenPort < relays[j].ListenPort
	})

	for _, relay := range relays {
		logger.Debug("serve", "Applying relay %s (type: %s, port: %d)", relay.ID, relay.Type, relay.ListenPort)
		if err := m.applyRelay(relay); err != nil {
			logger.Error("serve", "Failed to apply relay %s: %v", relay.ID, err)
			return err
		}
	}
	logger.Debug("serve", "Finished reconcile process")
	return nil
}

func (m *Manager) applyRelay(relay config.ServeRelay) error {
	target := ""
	switch relay.Type {
	case "https":
		protocol := "http"
		if relay.TargetHTTPS {
			protocol = "https+insecure"
		}
		target = fmt.Sprintf("%s://%s:%d", protocol, relay.TargetHost, relay.TargetPort)
		return m.run("serve", "--bg", "--https", fmt.Sprintf("%d", relay.ListenPort), target)
	case "tcp":
		target = fmt.Sprintf("tcp://%s:%d", relay.TargetHost, relay.TargetPort)
		return m.run("serve", "--bg", "--tcp", fmt.Sprintf("%d", relay.ListenPort), target)
	default:
		return fmt.Errorf("unsupported relay type: %s", relay.Type)
	}
}

func (m *Manager) run(args ...string) error {
	cmdStr := fmt.Sprintf("%s %s", m.binaryPath, strings.Join(args, " "))
	logger.Debug("serve", "Executing: %s", cmdStr)
	
	cmd := exec.Command(m.binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errStr := fmt.Sprintf("tailscale %s failed: %s (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		if strings.Contains(errStr, "netMap is nil") {
			logger.Debug("serve", "Tailscale not fully ready yet: %v", errStr)
		} else {
			logger.Error("serve", "%s", errStr)
		}
		return fmt.Errorf("%s", errStr)
	}
	logger.Debug("serve", "Execution successful: %s", cmdStr)
	return nil
}
