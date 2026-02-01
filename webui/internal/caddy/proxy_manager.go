package caddy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/logger"
)

// ProxyManager manages Caddy reverse proxies via the admin API
type ProxyManager struct {
	client        *APIClient
	serverMapPath string
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

	return &ProxyManager{
		client:        client,
		serverMapPath: serverMapPath,
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

	proxyToCreate := proxy
	proxyToCreate.ID = ""

	route, err := pm.buildRoute(proxyToCreate)
	if err != nil {
		logger.Error("caddy", "Failed to build route for proxy %s: %v", proxy.ID, err)
		return nil, fmt.Errorf("build route: %w", err)
	}

	serverName, err := pm.allocateServerName()
	if err != nil {
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
		return nil, fmt.Errorf("create server: %w", err)
	}

	pm.updateServerMap(proxy, serverName)

	logger.Info("caddy", "Added Caddy proxy: %s:%d -> %s (ID: %s)", proxy.Hostname, proxy.Port, proxy.Target, proxy.ID)
	return &proxy, nil
}

// GetProxy retrieves a proxy by ID using @id tag
func (pm *ProxyManager) GetProxy(id string) (*config.CaddyProxy, error) {
	data, err := pm.client.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get route by id: %w", err)
	}

	var route Route
	if err := json.Unmarshal(data, &route); err != nil {
		return nil, fmt.Errorf("unmarshal route: %w", err)
	}

	proxy, err := pm.routeToProxy(route)
	if err != nil {
		return nil, fmt.Errorf("convert route to proxy: %w", err)
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
	route, err := pm.buildRoute(proxy)
	if err != nil {
		logger.Error("caddy", "Failed to build route for proxy update %s: %v", proxy.ID, err)
		return fmt.Errorf("build route: %w", err)
	}

	serverName, err := pm.getServerNameForProxy(proxy)
	if err != nil {
		logger.Error("caddy", "Failed to find server for proxy %s: %v", proxy.ID, err)
		return fmt.Errorf("find server: %w", err)
	}

	server := &HTTPServer{
		Listen: []string{fmt.Sprintf(":%d", proxy.Port)},
		Routes: []Route{*route},
	}

	path := fmt.Sprintf("/apps/http/servers/%s", serverName)
	if err := pm.client.PatchConfig(path, server); err != nil {
		logger.Error("caddy", "Failed to update server %s for proxy %s via Caddy API: %v", serverName, proxy.ID, err)
		return fmt.Errorf("update server: %w", err)
	}

	pm.updateServerMap(proxy, serverName)

	logger.Info("caddy", "Updated Caddy proxy: %s (ID: %s)", proxy.Hostname, proxy.ID)
	return nil
}

// DeleteProxy removes a proxy by ID
func (pm *ProxyManager) DeleteProxy(id string) error {
	logger.Debug("caddy", "DeleteProxy: removing proxy ID %s", id)

	serverName, err := pm.getServerNameForProxy(config.CaddyProxy{ID: id})
	if err != nil {
		logger.Error("caddy", "Failed to find server for proxy %s: %v", id, err)
		return fmt.Errorf("find server: %w", err)
	}

	path := fmt.Sprintf("/apps/http/servers/%s", serverName)
	if err := pm.client.DeleteConfig(path); err != nil {
		logger.Error("caddy", "Failed to delete server %s for proxy %s via Caddy API: %v", serverName, id, err)
		return fmt.Errorf("delete server: %w", err)
	}

	pm.removeServerMapByID(id, serverName)

	logger.Info("caddy", "Deleted Caddy proxy: %s", id)
	return nil
}

// ListProxies retrieves all proxies for the server
func (pm *ProxyManager) ListProxies() ([]config.CaddyProxy, error) {
	servers, err := pm.listServers()
	if err != nil {
		return nil, fmt.Errorf("get servers: %w", err)
	}

	proxies := make([]config.CaddyProxy, 0)
	for serverName, server := range servers {
		if server == nil {
			continue
		}
		for _, route := range server.Routes {
			proxy, err := pm.routeToProxy(route)
			if err != nil {
				continue
			}
			pm.updateServerMap(*proxy, serverName)
			proxies = append(proxies, *proxy)
		}
	}

	return proxies, nil
}

// ToggleProxy enables or disables a proxy by updating its route
func (pm *ProxyManager) ToggleProxy(id string, enabled bool) error {
	proxy, err := pm.GetProxy(id)
	if err != nil {
		return err
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
	handler := make(Handler)
	handler["handler"] = "reverse_proxy"

	// Add @id if provided
	if proxy.ID != "" {
		handler["@id"] = proxy.ID
	}

	// Build upstreams
	upstreams := []Upstream{
		{Dial: proxy.Target},
	}
	handler["upstreams"] = upstreams

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

	handler["headers"] = headers

	// Add trusted proxies if enabled
	if proxy.TrustedProxies {
		// This is typically handled at the route level or with additional middleware
		// For now, we'll add it as a custom header directive
	}

	// Configure TLS transport for HTTPS targets
	if proxy.TLS || strings.HasPrefix(proxy.Target, "https://") {
		transport := HTTPTransport{
			Protocol: "http",
			TLS: &TLSConfig{
				InsecureSkipVerify: true, // Default for internal services
			},
		}

		// If specific cert files are configured, use them instead
		if certPath, ok := proxy.CustomHeaders["X-TLS-Cert"]; ok {
			transport.TLS.RootCAPEMFiles = []string{certPath}
			transport.TLS.InsecureSkipVerify = false
			delete(proxy.CustomHeaders, "X-TLS-Cert") // Remove from headers
		}

		handler["transport"] = transport
	}

	// Build route with matchers
	route := &Route{
		ID: proxy.ID,
		Match: []MatcherSet{
			{
				Host: []string{fmt.Sprintf("%s:%d", proxy.Hostname, proxy.Port)},
			},
		},
		Handle: []Handler{handler},
	}

	// If disabled, we could add a static_response handler instead
	// or simply not include the route. For now, we'll always include it.
	// The enabled flag is stored but not enforced at the Caddy level.

	return route, nil
}

// routeToProxy converts a Caddy Route back to a config.CaddyProxy
func (pm *ProxyManager) routeToProxy(route Route) (*config.CaddyProxy, error) {
	if len(route.Handle) == 0 {
		return nil, fmt.Errorf("route has no handlers")
	}

	handler := route.Handle[0]

	// Check if it's a reverse_proxy handler
	handlerType, ok := handler["handler"].(string)
	if !ok || handlerType != "reverse_proxy" {
		return nil, fmt.Errorf("not a reverse_proxy handler")
	}

	proxy := &config.CaddyProxy{
		ID:      route.ID,
		Enabled: true, // Default to enabled if route exists
	}

	if proxy.ID == "" {
		if handlerID, ok := handler["@id"].(string); ok {
			proxy.ID = handlerID
		}
	}

	// Extract hostname and port from matchers
	if len(route.Match) > 0 && len(route.Match[0].Host) > 0 {
		hostPort := route.Match[0].Host[0]
		parts := strings.Split(hostPort, ":")
		if len(parts) == 2 {
			proxy.Hostname = NormalizeHostname(parts[0])
			fmt.Sscanf(parts[1], "%d", &proxy.Port)
		}
	}

	// Extract upstreams
	if upstreams, ok := handler["upstreams"].([]interface{}); ok && len(upstreams) > 0 {
		if upstream, ok := upstreams[0].(map[string]interface{}); ok {
			if dial, ok := upstream["dial"].(string); ok {
				proxy.Target = dial
			}
		}
	}

	// Check for TLS transport
	if transport, ok := handler["transport"].(map[string]interface{}); ok {
		if _, hasTLS := transport["tls"]; hasTLS {
			proxy.TLS = true
		}
	}

	// Extract custom headers (excluding the default Host header)
	if headers, ok := handler["headers"].(map[string]interface{}); ok {
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

	return proxy, nil
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
			candidate, err := pm.routeToProxy(route)
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
	return false
}
