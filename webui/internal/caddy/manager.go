package caddy

import (
	"fmt"
	"log"
	"os/exec"
)

// Manager handles Caddy process management
type Manager struct {
	caddyBinary string
	caddyConfig string
	proxiesFile string
}

// NewManager creates a new Caddy manager
func NewManager(caddyBinary, caddyConfig, proxiesFile string) *Manager {
	if caddyBinary == "" {
		caddyBinary = "caddy" // Default to PATH
	}

	return &Manager{
		caddyBinary: caddyBinary,
		caddyConfig: caddyConfig,
		proxiesFile: proxiesFile,
	}
}

// Reload reloads the Caddy configuration
func (m *Manager) Reload() error {
	// First, regenerate the Caddyfile from JSON
	if err := m.RegenerateCaddyfile(); err != nil {
		return fmt.Errorf("failed to regenerate Caddyfile: %w", err)
	}

	// Validate the configuration
	if err := m.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Reload Caddy
	cmd := exec.Command(m.caddyBinary, "reload", "--config", m.caddyConfig)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reload Caddy: %w (output: %s)", err, string(output))
	}

	log.Printf("Caddy reloaded successfully")
	return nil
}

// Validate validates the Caddy configuration without reloading
func (m *Manager) Validate() error {
	cmd := exec.Command(m.caddyBinary, "validate", "--config", m.caddyConfig)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("validation failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// RegenerateCaddyfile regenerates the Caddyfile from proxy configurations
func (m *Manager) RegenerateCaddyfile() error {
	proxies, err := LoadProxies(m.proxiesFile)
	if err != nil {
		return fmt.Errorf("failed to load proxies: %w", err)
	}

	if err := GenerateCaddyfile(proxies, m.caddyConfig); err != nil {
		return fmt.Errorf("failed to generate Caddyfile: %w", err)
	}

	return nil
}

// Start starts Caddy (for initial startup)
func (m *Manager) Start() error {
	// Regenerate Caddyfile before starting
	if err := m.RegenerateCaddyfile(); err != nil {
		return fmt.Errorf("failed to regenerate Caddyfile: %w", err)
	}

	cmd := exec.Command(m.caddyBinary, "run", "--config", m.caddyConfig)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Caddy: %w", err)
	}

	log.Printf("Caddy started successfully")
	return nil
}

// Stop stops Caddy gracefully
func (m *Manager) Stop() error {
	cmd := exec.Command(m.caddyBinary, "stop")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop Caddy: %w (output: %s)", err, string(output))
	}

	log.Printf("Caddy stopped successfully")
	return nil
}

// GetStatus returns Caddy status
func (m *Manager) GetStatus() (bool, error) {
	// Try to validate - if Caddy is running and config is valid, this should work
	cmd := exec.Command(m.caddyBinary, "version")
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("Caddy not accessible: %w", err)
	}

	// Check if Caddy process is running by trying to get config
	cmd = exec.Command(m.caddyBinary, "adapt", "--config", m.caddyConfig)
	if err := cmd.Run(); err != nil {
		return false, nil // Config invalid but Caddy binary works
	}

	return true, nil
}
