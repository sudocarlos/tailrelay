package serve

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/sudocarlos/tailrelay/internal/config"
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
		return fmt.Errorf("relay not found")
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
		return fmt.Errorf("relay not found")
	}
	if err := config.SaveServeRelays(m.relayFile, list); err != nil {
		return err
	}
	return m.Reconcile()
}

// Reconcile resets tailscale serve config and reapplies all enabled relays.
func (m *Manager) Reconcile() error {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return err
	}

	if err := m.run("serve", "reset"); err != nil {
		return err
	}

	enabled := make([]config.ServeRelay, 0, len(list.Relays))
	for _, r := range list.Relays {
		if r.Enabled {
			enabled = append(enabled, r)
		}
	}
	sort.Slice(enabled, func(i, j int) bool {
		if enabled[i].ListenPort == enabled[j].ListenPort {
			return enabled[i].ID < enabled[j].ID
		}
		return enabled[i].ListenPort < enabled[j].ListenPort
	})

	for _, relay := range enabled {
		if err := m.applyRelay(relay); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) applyRelay(relay config.ServeRelay) error {
	target := ""
	switch relay.Type {
	case "https":
		target = fmt.Sprintf("http://%s:%d", relay.TargetHost, relay.TargetPort)
		return m.run("serve", "--bg", "--https", fmt.Sprintf("%d", relay.ListenPort), target)
	case "tcp":
		target = fmt.Sprintf("tcp://%s:%d", relay.TargetHost, relay.TargetPort)
		return m.run("serve", "--bg", "--tcp", fmt.Sprintf("%d", relay.ListenPort), target)
	default:
		return fmt.Errorf("unsupported relay type: %s", relay.Type)
	}
}

func (m *Manager) run(args ...string) error {
	cmd := exec.Command(m.binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tailscale %s failed: %w (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
