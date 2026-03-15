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

// ToggleProxy enables or disables a proxy.
// When disabling, the current Caddy counters are snapshotted into the metrics
// baseline before the server is removed so history is not lost on resume.
func (m *Manager) ToggleProxy(id string, enabled bool) error {
	if !enabled && m.store != nil {
		// Capture the current counters for this proxy before Caddy deletes the
		// server and resets them to zero.
		m.recordProxyPauseBaseline(id)
	}

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

// recordProxyPauseBaseline captures the current Caddy counter values for proxy
// id into the metrics store baseline so they survive the server deletion.
func (m *Manager) recordProxyPauseBaseline(id string) {
	// Find the server name for this proxy.
	m.proxyManager.mapMu.Lock()
	srvName := m.proxyManager.serverMap.ByProxyID[id]
	m.proxyManager.mapMu.Unlock()
	if srvName == "" {
		return
	}

	// Find the label for this proxy so we can key the baseline correctly.
	proxies, err := m.ListProxies()
	if err != nil {
		return
	}
	label := ""
	for _, p := range proxies {
		if p.ID == id {
			label = fmt.Sprintf(":%d \u2192 %s", p.Port, p.Target)
			break
		}
	}
	if label == "" {
		return
	}

	// Pull the latest value from the store for this server key.
	m.store.mu.RLock()
	var latest HostMetrics
	found := false
	if n := len(m.store.snapshots); n > 0 {
		if hm, ok := m.store.snapshots[n-1].Hosts[srvName]; ok {
			latest = hm
			found = true
		}
	}
	m.store.mu.RUnlock()

	if found {
		m.store.RecordPause(label, latest)
		log.Printf("metrics_store: recorded pause baseline for %s (%s)", label, srvName)
	}
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
// the current Caddy metrics and stores a baseline-adjusted snapshot so that
// the stored counter sequence is monotonically increasing even across Caddy
// config changes that reset all HTTP metrics counters.
//
// Counter-reset detection: Caddy resets the Prometheus counters of ALL HTTP
// servers whenever the HTTP app config changes (adding or removing any srvN).
// This means toggling any proxy causes every other proxy's counter to jump
// back to 0 on the next scrape.  We detect this by comparing each server's
// new raw value against the last stored value for that server key.  If any
// server's request counter decreased, we treat it as a global reset:
//   1. Save the pre-reset (last stored) value as a baseline for every server
//      that was present in the previous snapshot.
//   2. Apply all accumulated baselines to the new raw values before storing.
//
// The ToggleProxy → recordProxyPauseBaseline path is an additional early save
// that fires immediately at disable time (before Caddy's DELETE), catching the
// case where the background poller hasn't run yet.
func (m *Manager) snapshotMetrics() (*MetricsSnapshot, error) {
	data, err := m.liveFetch()
	if err != nil {
		return nil, err
	}

	// Build a map of new raw values keyed by server name for fast lookup.
	newByKey := make(map[string]HostMetrics, len(data.Hosts))
	for _, hm := range data.Hosts {
		key := hm.Server
		if key == "" {
			key = hm.Host
		}
		newByKey[key] = hm
	}

	// Compare against the previous snapshot to detect a Caddy-wide counter reset.
	if m.store != nil {
		prev := m.store.LastSnapshot()
		if prev != nil {
			resetDetected := false
			for key, prevHM := range prev.Hosts {
				if newHM, ok := newByKey[key]; ok {
					if newHM.Requests < prevHM.Requests {
						resetDetected = true
						break
					}
				}
			}
			if resetDetected {
				// Save all previous (pre-reset) values as baselines before the
				// raw counter values hit storage.  This covers every active relay,
				// not just the one whose toggle triggered the config change.
				log.Printf("metrics_store: Caddy counter reset detected — saving baselines for all active relays")
				for _, prevHM := range prev.Hosts {
					if prevHM.Label != "" {
						m.store.RecordPause(prevHM.Label, prevHM)
					}
				}
			}
		}
	}

	// Build the final snapshot with baselines applied.
	snap := &MetricsSnapshot{
		At:    time.Now(),
		Hosts: make(map[string]HostMetrics, len(newByKey)),
	}
	for key, hm := range newByKey {
		if hm.Label != "" && m.store != nil {
			hm = m.store.ApplyBaseline(hm.Label, hm)
		}
		snap.Hosts[key] = hm
	}
	return snap, nil
}

// liveFetch fetches metrics directly from Caddy and annotates labels.
// This is used both by the fallback path in GetMetrics and by snapshotMetrics.
//
// Label resolution strategy:
//  1. Primary — server-map lookup: each proxy owns a dedicated Caddy server
//     (srv0, srv1, …).  We build a reverse map srvN → ":<port> → <target>"
//     from the persisted ServerMap and the proxy metadata list.  This gives
//     an exact per-proxy label even when multiple proxies share the same FQDN.
//  2. Fallback — hostname lookup: used when the server label is absent from
//     the Prometheus output or the server is not in the map.  In that case we
//     match against the "host" Prometheus label as before.  If multiple proxies
//     share the hostname we take the first one (alphabetically by port) rather
//     than joining them all into an unreadable string.
func (m *Manager) liveFetch() (*MetricsData, error) {
	raw, err := m.proxyManager.client.GetMetricsRaw()
	if err != nil {
		return nil, fmt.Errorf("fetch metrics: %w", err)
	}
	data := ParseMetrics(raw)

	proxies, err := m.ListProxies()
	if err != nil {
		return data, nil
	}

	// Build proxyID → ":<port> → <target>" label.
	labelByID := make(map[string]string, len(proxies))
	for _, p := range proxies {
		if p.ID != "" {
			labelByID[p.ID] = fmt.Sprintf(":%d \u2192 %s", p.Port, p.Target)
		}
	}

	// Primary: srvN → label via server map (proxyID → srvN → label).
	// Invert: srvN → proxyID first, then resolve to label.
	bySrv := make(map[string]string) // srvN → label
	m.proxyManager.mapMu.Lock()
	for proxyID, srvName := range m.proxyManager.serverMap.ByProxyID {
		if lbl, ok := labelByID[proxyID]; ok {
			bySrv[srvName] = lbl
		}
	}
	m.proxyManager.mapMu.Unlock()

	// Fallback: host → label (for the case where server label is absent).
	// When multiple proxies share a hostname, pick the one whose port matches
	// the port suffix in the host label (e.g. "mynode.ts.net:8080" → :8080),
	// or if no port suffix, just use the first proxy for that hostname.
	type fallbackEntry struct {
		port  int
		label string
	}
	byHost := make(map[string][]fallbackEntry)
	for _, p := range proxies {
		if p.Hostname != "" {
			h := NormalizeHostname(p.Hostname)
			byHost[h] = append(byHost[h], fallbackEntry{p.Port, labelByID[p.ID]})
		}
	}

	for i, hm := range data.Hosts {
		// 1. Try exact server-name lookup.
		if hm.Server != "" {
			if lbl, ok := bySrv[hm.Server]; ok {
				data.Hosts[i].Label = lbl
				continue
			}
		}

		// 2. Fallback: match on hostname, disambiguating by port when possible.
		bare := hm.Host
		portHint := 0
		if h, portStr, splitErr := strings.Cut(hm.Host, ":"); splitErr {
			// strings.Cut returns found=true when the sep exists
			bare = h
			fmt.Sscanf(portStr, "%d", &portHint)
		}
		if entries, ok := byHost[bare]; ok {
			chosen := entries[0]
			for _, e := range entries {
				if portHint != 0 && e.port == portHint {
					chosen = e
					break
				}
			}
			data.Hosts[i].Label = chosen.label
		}
	}

	return data, nil
}

// Note: Reload, Start, Stop methods are no longer needed
// The Caddy API handles configuration changes atomically and instantly
// No manual reload or restart is required
