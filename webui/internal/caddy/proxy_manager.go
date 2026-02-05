package caddy

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/logger"
)

// ProxyManager manages Caddy reverse proxies via the admin API
type ProxyManager struct {
	client        *APIClient
	serverMapPath string
	metadataPath  string
	serverMap     *ServerMap
	mapMu         sync.Mutex
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(apiURL, serverMapPath string) *ProxyManager {
	client := NewAPIClient(apiURL)

	serverMap, err := LoadServerMap(serverMapPath)
	if err != nil {
		logger.Error("caddy", "Failed to load server map, starting empty: %v", err)
		serverMap = NewServerMap()
	}

	// Derive metadata path from server map path
	metadataPath := strings.TrimSuffix(serverMapPath, "_servers.json") + "_proxies.json"

	return &ProxyManager{
		client:        client,
		serverMapPath: serverMapPath,
		metadataPath:  metadataPath,
		serverMap:     serverMap,
	}
}

// NormalizeHostname trims whitespace and a trailing dot from hostnames.
func NormalizeHostname(hostname string) string {
	hostname = strings.TrimSpace(hostname)
	return strings.TrimSuffix(hostname, ".")
}

// AddProxy adds a new reverse proxy route to Caddy via API
func (pm *ProxyManager) AddProxy(proxy config.CaddyProxy) (*config.CaddyProxy, error) {
	proxy.Hostname = NormalizeHostname(proxy.Hostname)
	logger.Debug("caddy", "AddProxy: building route for %s:%d -> %s", proxy.Hostname, proxy.Port, proxy.Target)

	if proxy.ID == "" {
		id, err := config.GenerateToken()
		if err != nil {
			return nil, fmt.Errorf("generate proxy id: %w", err)
		}
		proxy.ID = id
	}

	// Save metadata first
	if err := AddProxyMetadata(pm.metadataPath, proxy); err != nil {
		logger.Error("caddy", "Failed to save proxy metadata: %v", err)
		return nil, fmt.Errorf("save metadata: %w", err)
	}

	// Only create route in Caddy if enabled
	if proxy.Enabled {
		route, err := pm.buildRoute(proxy)
		if err != nil {
			logger.Error("caddy", "Failed to build route for proxy %s: %v", proxy.ID, err)
			// Clean up metadata
			DeleteProxyMetadata(pm.metadataPath, proxy.ID)
			return nil, fmt.Errorf("build route: %w", err)
		}

		serverName, err := pm.allocateServerName()
		if err != nil {
			// Clean up metadata
			DeleteProxyMetadata(pm.metadataPath, proxy.ID)
			return nil, fmt.Errorf("allocate server name: %w", err)
		}

		path := fmt.Sprintf("/apps/http/servers/%s", serverName)
		logger.Debug("caddy", "Creating server for proxy at path: %s", path)

		server := &HTTPServer{
			Listen: []string{fmt.Sprintf(":%d", proxy.Port)},
			Routes: []Route{*route},
		}

		if err := pm.client.PutConfig(path, server); err != nil {
			logger.Error("caddy", "Failed to create server %s for %s:%d via Caddy API: %v", serverName, proxy.Hostname, proxy.Port, err)
			// Clean up metadata
			DeleteProxyMetadata(pm.metadataPath, proxy.ID)
			return nil, fmt.Errorf("create server: %w", err)
		}

		pm.updateServerMap(proxy, serverName)
	} else {
		logger.Debug("caddy", "Proxy %s created but not enabled, skipping Caddy route creation", proxy.ID)
	}

	logger.Info("caddy", "Added Caddy proxy: %s:%d -> %s (ID: %s, Enabled: %v)", proxy.Hostname, proxy.Port, proxy.Target, proxy.ID, proxy.Enabled)
	return &proxy, nil
}

// GetProxy retrieves a proxy by ID from metadata
func (pm *ProxyManager) GetProxy(id string) (*config.CaddyProxy, error) {
	// Get from metadata (source of truth)
	proxy, err := GetProxyMetadata(pm.metadataPath, id)
	if err != nil {
		return nil, fmt.Errorf("get proxy metadata: %w", err)
	}

	return proxy, nil
}

// UpdateProxy updates an existing proxy by ID
func (pm *ProxyManager) UpdateProxy(proxy config.CaddyProxy) error {
	logger.Debug("caddy", "UpdateProxy: updating proxy ID %s (%s:%d -> %s)", proxy.ID, proxy.Hostname, proxy.Port, proxy.Target)

	if proxy.ID == "" {
		logger.Error("caddy", "UpdateProxy called with empty ID")
		return fmt.Errorf("proxy ID is required for update")
	}

	proxy.Hostname = NormalizeHostname(proxy.Hostname)

	// Update metadata first
	if err := UpdateProxyMetadata(pm.metadataPath, proxy); err != nil {
		logger.Error("caddy", "Failed to update proxy metadata: %v", err)
		return fmt.Errorf("update metadata: %w", err)
	}

	// Only update route in Caddy if enabled
	if proxy.Enabled {
		route, err := pm.buildRoute(proxy)
		if err != nil {
			logger.Error("caddy", "Failed to build route for proxy update %s: %v", proxy.ID, err)
			return fmt.Errorf("build route: %w", err)
		}

		// Try to get server name from map
		serverName, err := pm.getServerNameForProxy(proxy)
		serverExists := false

		if err == nil {
			// Found in map, but verify it actually exists in Caddy
			serverExists = pm.serverExistsInCaddy(serverName)
			if !serverExists {
				logger.Debug("caddy", "Server %s found in map but not in Caddy for proxy %s, will allocate new server", serverName, proxy.ID)
				// Remove stale entry from map
				pm.removeServerMapByID(proxy.ID, serverName)
			}
		}

		if !serverExists {
			// Server doesn't exist in Caddy (proxy was previously disabled or map is stale)
			// Allocate a new server name
			serverName, err = pm.allocateServerName()
			if err != nil {
				logger.Error("caddy", "Failed to allocate server for proxy %s: %v", proxy.ID, err)
				return fmt.Errorf("allocate server: %w", err)
			}
			logger.Debug("caddy", "Allocated new server name %s for re-enabled proxy %s", serverName, proxy.ID)
		}

		server := &HTTPServer{
			Listen: []string{fmt.Sprintf(":%d", proxy.Port)},
			Routes: []Route{*route},
		}

		path := fmt.Sprintf("/apps/http/servers/%s", serverName)

		// Use PUT if server doesn't exist (create), PATCH if it exists (update)
		var apiErr error
		if serverExists {
			apiErr = pm.client.PatchConfig(path, server)
			logger.Debug("caddy", "Updating existing server %s for proxy %s", serverName, proxy.ID)
		} else {
			apiErr = pm.client.PutConfig(path, server)
			logger.Debug("caddy", "Creating new server %s for proxy %s", serverName, proxy.ID)
		}

		if apiErr != nil {
			logger.Error("caddy", "Failed to %s server %s for proxy %s via Caddy API: %v",
				map[bool]string{true: "update", false: "create"}[serverExists], serverName, proxy.ID, apiErr)
			return fmt.Errorf("update server: %w", apiErr)
		}

		pm.updateServerMap(proxy, serverName)
	} else {
		// If disabled, remove from Caddy but keep metadata
		serverName, err := pm.getServerNameForProxy(proxy)
		if err == nil {
			path := fmt.Sprintf("/apps/http/servers/%s", serverName)
			if err := pm.client.DeleteConfig(path); err != nil {
				logger.Warn("caddy", "Failed to delete server %s for disabled proxy %s: %v", serverName, proxy.ID, err)
			}
			// Remove from server map since it no longer exists in Caddy
			pm.removeServerMapByID(proxy.ID, serverName)
			logger.Debug("caddy", "Removed server %s mapping for disabled proxy %s", serverName, proxy.ID)
		}
	}

	logger.Info("caddy", "Updated Caddy proxy: %s (ID: %s, Enabled: %v)", proxy.Hostname, proxy.ID, proxy.Enabled)
	return nil
}

// DeleteProxy removes a proxy by ID
func (pm *ProxyManager) DeleteProxy(id string) error {
	logger.Debug("caddy", "DeleteProxy: removing proxy ID %s", id)

	// Delete from Caddy if it exists
	serverName, err := pm.getServerNameForProxy(config.CaddyProxy{ID: id})
	if err == nil {
		path := fmt.Sprintf("/apps/http/servers/%s", serverName)
		if err := pm.client.DeleteConfig(path); err != nil {
			logger.Warn("caddy", "Failed to delete server %s for proxy %s via Caddy API: %v", serverName, id, err)
		}
		pm.removeServerMapByID(id, serverName)
	}

	// Delete from metadata
	if err := DeleteProxyMetadata(pm.metadataPath, id); err != nil {
		logger.Error("caddy", "Failed to delete proxy metadata: %v", err)
		return fmt.Errorf("delete metadata: %w", err)
	}

	logger.Info("caddy", "Deleted Caddy proxy: %s", id)
	return nil
}

// ListProxies retrieves all proxies from metadata
func (pm *ProxyManager) ListProxies() ([]config.CaddyProxy, error) {
	// Load from metadata file (source of truth)
	proxies, err := LoadProxyMetadata(pm.metadataPath)
	if err != nil {
		logger.Error("caddy", "Failed to load proxy metadata: %v", err)
		return nil, fmt.Errorf("load metadata: %w", err)
	}

	return proxies, nil
}

// ToggleProxy enables or disables a proxy
func (pm *ProxyManager) ToggleProxy(id string, enabled bool) error {
	// Get proxy from metadata
	proxy, err := GetProxyMetadata(pm.metadataPath, id)
	if err != nil {
		logger.Error("caddy", "Failed to get proxy metadata for %s: %v", id, err)
		return fmt.Errorf("get proxy: %w", err)
	}

	proxy.Enabled = enabled
	return pm.UpdateProxy(*proxy)
}

// GetStatus checks if Caddy API is accessible
func (pm *ProxyManager) GetStatus() (bool, error) {
	_, err := pm.client.GetConfig("/")
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetUpstreams returns the status of all reverse proxy upstreams
func (pm *ProxyManager) GetUpstreams() ([]UpstreamStatus, error) {
	return pm.client.GetReverseProxyUpstreams()
}

// buildRoute converts a config.CaddyProxy to a Caddy Route with ReverseProxyHandler
func (pm *ProxyManager) buildRoute(proxy config.CaddyProxy) (*Route, error) {
	// Build the reverse proxy handler
	reverseProxyHandler := make(Handler)
	reverseProxyHandler["handler"] = "reverse_proxy"

	// Add @id if provided
	if proxy.ID != "" {
		reverseProxyHandler["@id"] = proxy.ID
	}

	// Build upstreams
	upstreams := []Upstream{
		{Dial: proxy.Target},
	}
	reverseProxyHandler["upstreams"] = upstreams

	// Build headers configuration using map form expected by Caddy
	headers := HeaderConfig{
		Request: &HeaderOps{
			Set: map[string][]string{
				"Host": []string{"{http.reverse_proxy.upstream.hostport}"},
			},
		},
	}

	// Add custom headers if provided
	if len(proxy.CustomHeaders) > 0 {
		if headers.Request.Set == nil {
			headers.Request.Set = make(map[string][]string)
		}
		for key, value := range proxy.CustomHeaders {
			headers.Request.Set[key] = []string{value}
		}
	}

	reverseProxyHandler["headers"] = headers

	// Add trusted proxies if enabled
	if proxy.TrustedProxies {
		reverseProxyHandler["trusted_proxies"] = []string{
			"192.168.0.0/16",
			"172.16.0.0/12",
			"10.0.0.0/8",
			"127.0.0.1/8",
			"fd00::/8",
			"::1",
		}
	}

	// Configure TLS transport only when a CA file is provided (srv0-like config)
	if proxy.TLSCertFile != "" {
		transport := HTTPTransport{
			Protocol: "http",
			TLS:      &TLSConfig{},
		}

		transport.TLS.CA = &TLSCAConfig{
			Provider: "file",
			PEMFiles: []string{proxy.TLSCertFile},
		}

		reverseProxyHandler["transport"] = transport
	}

	// Build route with matchers
	subrouteHandler := Handler{
		"handler": "subroute",
		"routes": []Route{
			{
				Handle: []Handler{reverseProxyHandler},
			},
		},
	}

	route := &Route{
		ID:       proxy.ID,
		Terminal: true,
		Match: []MatcherSet{
			{
				Host: []string{NormalizeHostname(proxy.Hostname)},
			},
		},
		Handle: []Handler{subrouteHandler},
	}

	// If disabled, we could add a static_response handler instead
	// or simply not include the route. For now, we'll always include it.
	// The enabled flag is stored but not enforced at the Caddy level.

	return route, nil
}

// routeToProxy converts a Caddy Route back to a config.CaddyProxy
func (pm *ProxyManager) routeToProxy(route Route) (*config.CaddyProxy, error) {
	return pm.routeToProxyWithListen(route, nil)
}

func (pm *ProxyManager) routeToProxyWithListen(route Route, listenAddrs []string) (*config.CaddyProxy, error) {
	if len(route.Handle) == 0 {
		return nil, fmt.Errorf("route has no handlers")
	}

	reverseProxyHandler, ok := extractReverseProxyHandler(route)
	if !ok {
		return nil, fmt.Errorf("not a reverse_proxy handler")
	}

	proxy := &config.CaddyProxy{
		ID:      route.ID,
		Enabled: true, // Default to enabled if route exists
	}

	if proxy.ID == "" {
		if handlerID, ok := reverseProxyHandler["@id"].(string); ok {
			proxy.ID = handlerID
		}
	}

	// Extract hostname and port from matchers
	if len(route.Match) > 0 && len(route.Match[0].Host) > 0 {
		hostValue := route.Match[0].Host[0]
		if strings.Contains(hostValue, ":") {
			if host, portStr, err := net.SplitHostPort(hostValue); err == nil {
				proxy.Hostname = NormalizeHostname(host)
				if proxy.Port == 0 {
					if port, convErr := strconv.Atoi(portStr); convErr == nil {
						proxy.Port = port
					}
				}
			} else if parts := strings.SplitN(hostValue, ":", 2); len(parts) == 2 {
				proxy.Hostname = NormalizeHostname(parts[0])
				if proxy.Port == 0 {
					if port, convErr := strconv.Atoi(parts[1]); convErr == nil {
						proxy.Port = port
					}
				}
			} else {
				proxy.Hostname = NormalizeHostname(hostValue)
			}
		} else {
			proxy.Hostname = NormalizeHostname(hostValue)
		}
	}

	if port, ok := parseListenPort(listenAddrs); ok {
		proxy.Port = port
	}

	// Extract upstreams
	if upstreams, ok := reverseProxyHandler["upstreams"].([]interface{}); ok && len(upstreams) > 0 {
		if upstream, ok := upstreams[0].(map[string]interface{}); ok {
			if dial, ok := upstream["dial"].(string); ok {
				proxy.Target = dial
			}
		}
	}

	// Check for TLS transport
	if transport, ok := reverseProxyHandler["transport"].(map[string]interface{}); ok {
		if tlsConfig, hasTLS := transport["tls"].(map[string]interface{}); hasTLS {
			proxy.TLS = true
			if caCfg, ok := tlsConfig["ca"].(map[string]interface{}); ok {
				if pemFiles, ok := caCfg["pem_files"].([]interface{}); ok && len(pemFiles) > 0 {
					if pemFile, ok := pemFiles[0].(string); ok {
						proxy.TLSCertFile = pemFile
					}
				}
			}
		}
	}

	// Extract custom headers (excluding the default Host header)
	if headers, ok := reverseProxyHandler["headers"].(map[string]interface{}); ok {
		if request, ok := headers["request"].(map[string]interface{}); ok {
			if setMap, ok := request["set"].(map[string]interface{}); ok {
				proxy.CustomHeaders = make(map[string]string)
				for field, val := range setMap {
					if strings.EqualFold(field, "Host") {
						continue
					}
					if values, ok := val.([]interface{}); ok && len(values) > 0 {
						if value, ok := values[0].(string); ok {
							proxy.CustomHeaders[field] = value
						}
					}
				}
			}
		}
	}

	if trustedProxies, ok := reverseProxyHandler["trusted_proxies"]; ok {
		switch values := trustedProxies.(type) {
		case []interface{}:
			if len(values) > 0 {
				proxy.TrustedProxies = true
			}
		case []string:
			if len(values) > 0 {
				proxy.TrustedProxies = true
			}
		}
	}

	return proxy, nil
}

func extractReverseProxyHandler(route Route) (Handler, bool) {
	if len(route.Handle) == 0 {
		return nil, false
	}

	first := route.Handle[0]
	if handlerType, ok := first["handler"].(string); ok {
		switch handlerType {
		case "reverse_proxy":
			return first, true
		case "subroute":
			routesRaw, ok := first["routes"].([]interface{})
			if !ok {
				return nil, false
			}
			for _, routeRaw := range routesRaw {
				routeMap, ok := routeRaw.(map[string]interface{})
				if !ok {
					continue
				}
				handlesRaw, ok := routeMap["handle"].([]interface{})
				if !ok {
					continue
				}
				for _, handleRaw := range handlesRaw {
					handleMap, ok := handleRaw.(map[string]interface{})
					if !ok {
						continue
					}
					if nestedType, ok := handleMap["handler"].(string); ok && nestedType == "reverse_proxy" {
						return handleMap, true
					}
				}
			}
		}
	}

	return nil, false
}

func parseListenPort(listenAddrs []string) (int, bool) {
	for _, addr := range listenAddrs {
		candidate := strings.TrimSpace(addr)
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, "unix/") {
			continue
		}
		host, portStr, err := net.SplitHostPort(candidate)
		if err == nil {
			_ = host
			if port, convErr := strconv.Atoi(portStr); convErr == nil {
				return port, true
			}
			continue
		}
		if strings.HasPrefix(candidate, ":") {
			if port, convErr := strconv.Atoi(strings.TrimPrefix(candidate, ":")); convErr == nil {
				return port, true
			}
		}
	}

	return 0, false
}

func extractIDFromLocation(location string) (string, error) {
	if location == "" {
		return "", fmt.Errorf("empty Location header")
	}

	parsed, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("parse location: %w", err)
	}

	path := parsed.Path
	if path == "" {
		path = location
	}

	marker := "/id/"
	idx := strings.LastIndex(path, marker)
	if idx == -1 {
		return "", fmt.Errorf("Location header missing /id/: %s", location)
	}

	id := strings.TrimPrefix(path[idx:], marker)
	id = strings.Trim(id, "/")
	if id == "" {
		return "", fmt.Errorf("empty id in Location header: %s", location)
	}

	return id, nil
}

// InitializeServer ensures the HTTP server exists in Caddy config
func (pm *ProxyManager) InitializeServer(listenAddrs []string) error {
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":80", ":443"}
	}

	server := &HTTPServer{
		Listen: listenAddrs,
		Routes: []Route{},
	}

	serverName, err := pm.allocateServerName()
	if err != nil {
		return fmt.Errorf("allocate server name: %w", err)
	}

	path := fmt.Sprintf("/apps/http/servers/%s", serverName)
	if err := pm.client.PutConfig(path, server); err != nil {
		return fmt.Errorf("initialize server: %w", err)
	}

	return nil
}

func (pm *ProxyManager) listServers() (map[string]*HTTPServer, error) {
	data, err := pm.client.GetConfig("/apps/http/servers")
	if err != nil {
		return nil, err
	}

	var servers map[string]*HTTPServer
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("unmarshal servers: %w", err)
	}

	return servers, nil
}

func (pm *ProxyManager) getServerNameForProxy(proxy config.CaddyProxy) (string, error) {
	pm.mapMu.Lock()
	if proxy.ID != "" {
		if serverName, ok := pm.serverMap.ByProxyID[proxy.ID]; ok {
			pm.mapMu.Unlock()
			return serverName, nil
		}
	}
	if proxy.Hostname != "" && proxy.Port != 0 {
		if serverName, ok := pm.serverMap.ByHostPort[pm.hostPortKey(proxy)]; ok {
			pm.mapMu.Unlock()
			return serverName, nil
		}
	}
	pm.mapMu.Unlock()

	serverName, err := pm.findServerNameInCaddy(proxy)
	if err != nil {
		return "", err
	}

	pm.updateServerMap(proxy, serverName)
	return serverName, nil
}

// serverExistsInCaddy checks if a server name actually exists in Caddy's configuration
func (pm *ProxyManager) serverExistsInCaddy(serverName string) bool {
	path := fmt.Sprintf("/apps/http/servers/%s", serverName)
	data, err := pm.client.GetConfig(path)
	if err != nil {
		return false
	}
	// Caddy returns "null" for non-existent paths
	dataStr := strings.TrimSpace(string(data))
	exists := len(dataStr) > 0 && dataStr != "null"
	logger.Debug("caddy", "Checking if server %s exists in Caddy: %v (data: %s)", serverName, exists, dataStr)
	return exists
}

func (pm *ProxyManager) allocateServerName() (string, error) {
	pm.mapMu.Lock()
	defer pm.mapMu.Unlock()

	servers, err := pm.listServers()
	if err != nil {
		return "", err
	}

	serverNames := make(map[string]bool)
	for name := range servers {
		serverNames[name] = true
	}
	for _, name := range pm.serverMap.ByProxyID {
		serverNames[name] = true
	}
	for _, name := range pm.serverMap.ByHostPort {
		serverNames[name] = true
	}

	for i := pm.serverMap.NextIndex; ; i++ {
		candidate := fmt.Sprintf("srv%d", i)
		if !serverNames[candidate] {
			pm.serverMap.NextIndex = i + 1
			if err := SaveServerMap(pm.serverMapPath, pm.serverMap); err != nil {
				logger.Error("caddy", "Failed to save server map: %v", err)
			}
			return candidate, nil
		}
	}
}

func (pm *ProxyManager) updateServerMap(proxy config.CaddyProxy, serverName string) {
	if serverName == "" {
		return
	}

	pm.mapMu.Lock()
	defer pm.mapMu.Unlock()

	if proxy.ID != "" {
		pm.serverMap.ByProxyID[proxy.ID] = serverName
	}
	if proxy.Hostname != "" && proxy.Port != 0 {
		pm.serverMap.ByHostPort[pm.hostPortKey(proxy)] = serverName
	}

	if err := SaveServerMap(pm.serverMapPath, pm.serverMap); err != nil {
		logger.Error("caddy", "Failed to save server map: %v", err)
	}
}

func (pm *ProxyManager) removeServerMapByID(proxyID, serverName string) {
	pm.mapMu.Lock()
	defer pm.mapMu.Unlock()

	if proxyID != "" {
		delete(pm.serverMap.ByProxyID, proxyID)
	}
	if serverName != "" {
		for key, name := range pm.serverMap.ByHostPort {
			if name == serverName {
				delete(pm.serverMap.ByHostPort, key)
			}
		}
	}

	if err := SaveServerMap(pm.serverMapPath, pm.serverMap); err != nil {
		logger.Error("caddy", "Failed to save server map: %v", err)
	}
}

func (pm *ProxyManager) hostPortKey(proxy config.CaddyProxy) string {
	return fmt.Sprintf("%s:%d", NormalizeHostname(proxy.Hostname), proxy.Port)
}

func (pm *ProxyManager) findServerNameInCaddy(proxy config.CaddyProxy) (string, error) {
	servers, err := pm.listServers()
	if err != nil {
		return "", err
	}

	for serverName, server := range servers {
		if server == nil {
			continue
		}
		for _, route := range server.Routes {
			if proxy.ID != "" && routeHasID(route, proxy.ID) {
				return serverName, nil
			}
			candidate, err := pm.routeToProxyWithListen(route, server.Listen)
			if err != nil {
				continue
			}
			if proxy.Hostname != "" && proxy.Port != 0 {
				if NormalizeHostname(candidate.Hostname) == NormalizeHostname(proxy.Hostname) && candidate.Port == proxy.Port {
					return serverName, nil
				}
			}
		}
	}

	return "", fmt.Errorf("server not found for proxy")
}

func routeHasID(route Route, id string) bool {
	if route.ID == id {
		return true
	}
	if len(route.Handle) == 0 {
		return false
	}
	if handlerID, ok := route.Handle[0]["@id"].(string); ok {
		return handlerID == id
	}

	reverseProxyHandler, ok := extractReverseProxyHandler(route)
	if !ok {
		return false
	}
	if handlerID, ok := reverseProxyHandler["@id"].(string); ok {
		return handlerID == id
	}
	return false
}
