package caddy

import (
	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/logger"
)

// MigrateExistingProxies discovers and syncs existing proxies from Caddy to metadata storage
// This runs on every startup to ensure the UI tracks all proxies in Caddy
func (pm *ProxyManager) MigrateExistingProxies() error {
	logger.Info("caddy", "Discovering existing proxies in Caddy...")

	// Load existing metadata
	existing, err := LoadProxyMetadata(pm.metadataPath)
	if err != nil && err.Error() != "open : no such file or directory" {
		logger.Warn("caddy", "Failed to load existing metadata: %v", err)
	}

	// Build map of existing proxies by route ID for quick lookup
	existingByID := make(map[string]config.CaddyProxy)
	for _, proxy := range existing {
		if proxy.ID != "" {
			existingByID[proxy.ID] = proxy
		}
	}

	// Get all servers from Caddy
	servers, err := pm.listServers()
	if err != nil {
		logger.Warn("caddy", "Failed to list Caddy servers for discovery: %v", err)
		return nil // Don't fail, just skip discovery
	}

	var discoveredProxies []config.CaddyProxy
	discovered := 0
	updated := 0

	for serverName, server := range servers {
		if server == nil {
			continue
		}
		for _, route := range server.Routes {
			proxy, err := pm.routeToProxyWithListen(route, server.Listen)
			if err != nil {
				logger.Warn("caddy", "Failed to convert route to proxy in server %s: %v", serverName, err)
				continue
			}

			// Check if this proxy already exists in metadata
			if existingProxy, exists := existingByID[proxy.ID]; exists {
				// Preserve existing settings (especially autostart)
				proxy.Autostart = existingProxy.Autostart
				proxy.Enabled = true // If it's in Caddy, it's enabled
				logger.Debug("caddy", "Found existing proxy in metadata: %s (ID: %s)", proxy.Hostname, proxy.ID)
				updated++
			} else {
				// New proxy discovered - set defaults
				proxy.Enabled = true   // Already in Caddy, so it's enabled
				proxy.Autostart = true // Default autostart to true for discovered proxies
				logger.Info("caddy", "Discovered new proxy in Caddy: %s:%d -> %s (ID: %s)", proxy.Hostname, proxy.Port, proxy.Target, proxy.ID)
				discovered++
			}

			discoveredProxies = append(discoveredProxies, *proxy)
			pm.updateServerMap(*proxy, serverName)
		}
	}

	// Merge with disabled proxies from metadata (not in Caddy but in DB)
	for _, existingProxy := range existing {
		found := false
		for _, discovered := range discoveredProxies {
			if discovered.ID == existingProxy.ID {
				found = true
				break
			}
		}
		if !found {
			// Proxy is in metadata but not in Caddy - keep it as disabled
			existingProxy.Enabled = false
			discoveredProxies = append(discoveredProxies, existingProxy)
			logger.Debug("caddy", "Keeping disabled proxy from metadata: %s (ID: %s)", existingProxy.Hostname, existingProxy.ID)
		}
	}

	if discovered > 0 || updated > 0 {
		logger.Info("caddy", "Syncing proxy metadata: %d new, %d updated, %d total", discovered, updated, len(discoveredProxies))
		if err := SaveProxyMetadata(pm.metadataPath, discoveredProxies); err != nil {
			logger.Error("caddy", "Failed to save proxy metadata: %v", err)
			return err
		}
		logger.Info("caddy", "Successfully synced %d proxies to metadata storage", len(discoveredProxies))
	} else if len(discoveredProxies) > 0 {
		logger.Info("caddy", "No changes detected, %d proxies already tracked", len(discoveredProxies))
	} else {
		logger.Info("caddy", "No proxies found in Caddy")
	}

	return nil
}
