package caddy

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sudocarlos/tailrelay/internal/config"
)

// Manager handles Caddy API-based management
type Manager struct {
	proxyManager *ProxyManager
	apiURL       string
	serverMap    string
	store        *MetricsStore
}

// NewManager creates a new Caddy manager using the API
func NewManager(apiURL, serverMapPath string) *Manager {
	if apiURL == "" {
		apiURL = DefaultAdminAPI
	}

	proxyMgr := NewProxyManager(apiURL, serverMapPath)

	return &Manager{
		proxyManager: proxyMgr,
		apiURL:       apiURL,
		serverMap:    serverMapPath,
	}
}

// StartMetricsPoller initialises the MetricsStore with the given history file
// and starts the background polling goroutine.  It should be called once during
// server startup and the provided context should be cancelled on shutdown so
// the goroutine flushes to disk before exiting.
func (m *Manager) StartMetricsPoller(ctx context.Context, historyFile string) {
	m.store = NewMetricsStore(historyFile)
	m.store.Start(ctx, m.snapshotMetrics)
}

// FlushMetrics writes the current metrics history to disk.  Call this during
// graceful shutdown if StartMetricsPoller was used.
func (m *Manager) FlushMetrics() {
	if m.store != nil {
		m.store.Flush()
	}
}

// AddProxy adds a new reverse proxy via Caddy API
func (m *Manager) AddProxy(proxy config.CaddyProxy) (*config.CaddyProxy, error) {
	created, err := m.proxyManager.AddProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to add proxy: %w", err)
	}
	log.Printf("Proxy added successfully: %s", created.ID)
	return created, nil
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

// GetProxiesStatus returns a map of proxy IDs to their running status in Caddy
func (m *Manager) GetProxiesStatus() (map[string]bool, error) {
	return m.proxyManager.GetProxiesStatus()
}

// InitializeServer ensures the HTTP server is configured in Caddy
func (m *Manager) InitializeServer(listenAddrs []string) error {
	if err := m.proxyManager.InitializeServer(listenAddrs); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}
	log.Printf("Server initialized")
	return nil
}

// MigrateExistingProxies migrates existing Caddy proxies to metadata storage
func (m *Manager) MigrateExistingProxies() error {
	return m.proxyManager.MigrateExistingProxies()
}

// InitializeAutostart starts all proxies with autostart enabled
func (m *Manager) InitializeAutostart() error {
	proxies, err := m.ListProxies()
	if err != nil {
		return fmt.Errorf("failed to list proxies: %w", err)
	}

	started := 0
	skipped := 0
	for _, proxy := range proxies {
		if proxy.Autostart {
			// Always sync state to Caddy API to ensure the route exists after a container restart
			proxy.Enabled = true
			if err := m.UpdateProxy(proxy); err != nil {
				log.Printf("Warning: failed to autostart proxy %s (ID: %s): %v", proxy.Hostname, proxy.ID, err)
				skipped++
				continue
			}
			started++
			log.Printf("Autostarted proxy %s (ID: %s)", proxy.Hostname, proxy.ID)
		} else {
			// Check if we need to sync disabled state (if it was previously thought to be enabled)
			if proxy.Enabled {
				proxy.Enabled = false
				if err := m.UpdateProxy(proxy); err != nil {
					log.Printf("Warning: failed to sync disabled state for proxy %s (ID: %s): %v", proxy.Hostname, proxy.ID, err)
				} else {
					log.Printf("Disabled non-autostart proxy %s (ID: %s)", proxy.Hostname, proxy.ID)
				}
			}
			skipped++
		}
	}

	log.Printf("Proxy autostart complete: %d started, %d skipped", started, skipped)
	return nil
}

// UpdateProxyHostnames replaces the hostname in all proxies that currently
// use oldFQDN with newFQDN and pushes the updated routes to Caddy. This
// should be called after a Tailscale hostname change.
func (m *Manager) UpdateProxyHostnames(oldFQDN, newFQDN string) error {
	return m.proxyManager.UpdateProxyHostnames(oldFQDN, newFQDN)
}

// GetMetrics returns metrics data for the requested time window.
//
// window == 0  →  latest snapshot values (cumulative totals, current behaviour)
// window > 0   →  traffic delta within that window (e.g. last hour)
//
// When the MetricsStore is active (StartMetricsPoller was called) data is
// served from the store, which preserves history for paused relays.
// If the store is not yet initialised the method falls back to a live Caddy
// fetch so the handler always has something to return.
func (m *Manager) GetMetrics(window time.Duration) (*MetricsData, error) {
	if m.store != nil {
		data := m.store.Query(window)
		return data, nil
	}
	// Fallback: live fetch (no store yet, e.g. very early startup).
	return m.liveFetch()
}

// snapshotMetrics is the fetchFn supplied to MetricsStore.Start.  It fetches
// the current Caddy metrics and annotates each host with the compact label.
func (m *Manager) snapshotMetrics() (*MetricsSnapshot, error) {
	data, err := m.liveFetch()
	if err != nil {
		return nil, err
	}

	snap := &MetricsSnapshot{
		At:    time.Now(),
		Hosts: make(map[string]HostMetrics, len(data.Hosts)),
	}
	for _, hm := range data.Hosts {
		key := hm.Host
		if hm.Label != "" {
			key = hm.Label
		}
		snap.Hosts[key] = hm
	}
	return snap, nil
}

// liveFetch fetches metrics directly from Caddy and annotates labels.
// This is used both by the fallback path in GetMetrics and by snapshotMetrics.
func (m *Manager) liveFetch() (*MetricsData, error) {
	raw, err := m.proxyManager.client.GetMetricsRaw()
	if err != nil {
		return nil, fmt.Errorf("fetch metrics: %w", err)
	}
	data := ParseMetrics(raw)

	// Build a hostname → label map from the current proxy list so we can
	// replace the verbose FQDN with ":port → target" in the UI.
	proxies, err := m.ListProxies()
	if err == nil {
		type labelEntry struct {
			port   int
			target string
		}
		// A hostname may map to multiple proxies; use all of them joined by ", ".
		byHost := map[string][]labelEntry{}
		for _, p := range proxies {
			if p.Hostname != "" {
				byHost[p.Hostname] = append(byHost[p.Hostname], labelEntry{p.Port, p.Target})
			}
		}
		for i, hm := range data.Hosts {
			if entries, ok := byHost[hm.Host]; ok {
				parts := make([]string, 0, len(entries))
				for _, e := range entries {
					parts = append(parts, fmt.Sprintf(":%d \u2192 %s", e.port, e.target))
				}
				data.Hosts[i].Label = strings.Join(parts, ", ")
			}
		}
	}

	return data, nil
}

// Note: Reload, Start, Stop methods are no longer needed
// The Caddy API handles configuration changes atomically and instantly
// No manual reload or restart is required
