package caddy

import (
	"fmt"
	"log"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

// Manager handles Caddy API-based management
type Manager struct {
	proxyManager *ProxyManager
	apiURL       string
	serverName   string
}

// NewManager creates a new Caddy manager using the API
func NewManager(apiURL, serverName string) *Manager {
	if apiURL == "" {
		apiURL = DefaultAdminAPI
	}
	// Don't set default here - let ProxyManager discover it
	// This allows auto-detection of server names like 'srv0' when Caddy
	// auto-generates them from Caddyfile

	proxyMgr := NewProxyManager(apiURL, serverName)

	return &Manager{
		proxyManager: proxyMgr,
		apiURL:       apiURL,
		serverName:   proxyMgr.serverName,
	}
}

// AddProxy adds a new reverse proxy via Caddy API
func (m *Manager) AddProxy(proxy config.CaddyProxy) error {
	if err := m.proxyManager.AddProxy(proxy); err != nil {
		return fmt.Errorf("failed to add proxy: %w", err)
	}
	log.Printf("Proxy added successfully: %s", proxy.ID)
	return nil
}

// GetProxy retrieves a proxy by ID
func (m *Manager) GetProxy(id string) (*config.CaddyProxy, error) {
	return m.proxyManager.GetProxy(id)
}

// UpdateProxy updates an existing proxy
func (m *Manager) UpdateProxy(proxy config.CaddyProxy) error {
	if err := m.proxyManager.UpdateProxy(proxy); err != nil {
		return fmt.Errorf("failed to update proxy: %w", err)
	}
	log.Printf("Proxy updated successfully: %s", proxy.ID)
	return nil
}

// DeleteProxy removes a proxy by ID
func (m *Manager) DeleteProxy(id string) error {
	if err := m.proxyManager.DeleteProxy(id); err != nil {
		return fmt.Errorf("failed to delete proxy: %w", err)
	}
	log.Printf("Proxy deleted successfully: %s", id)
	return nil
}

// ListProxies retrieves all proxies
func (m *Manager) ListProxies() ([]config.CaddyProxy, error) {
	return m.proxyManager.ListProxies()
}

// ToggleProxy enables or disables a proxy
func (m *Manager) ToggleProxy(id string, enabled bool) error {
	if err := m.proxyManager.ToggleProxy(id, enabled); err != nil {
		return fmt.Errorf("failed to toggle proxy: %w", err)
	}
	status := "enabled"
	if !enabled {
		status = "disabled"
	}
	log.Printf("Proxy %s: %s", status, id)
	return nil
}

// GetStatus checks if Caddy API is accessible
func (m *Manager) GetStatus() (bool, error) {
	return m.proxyManager.GetStatus()
}

// GetUpstreams returns the status of all reverse proxy upstreams
func (m *Manager) GetUpstreams() ([]UpstreamStatus, error) {
	return m.proxyManager.GetUpstreams()
}

// InitializeServer ensures the HTTP server is configured in Caddy
func (m *Manager) InitializeServer(listenAddrs []string) error {
	if err := m.proxyManager.InitializeServer(listenAddrs); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}
	log.Printf("Server initialized: %s", m.serverName)
	return nil
}

// Note: Reload, Start, Stop methods are no longer needed
// The Caddy API handles configuration changes atomically and instantly
// No manual reload or restart is required
