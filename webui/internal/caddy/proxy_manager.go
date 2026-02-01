package caddy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/logger"
)

// ProxyManager manages Caddy reverse proxies via the admin API
type ProxyManager struct {
	client     *APIClient
	serverName string
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(apiURL, serverName string) *ProxyManager {
	client := NewAPIClient(apiURL)

	// If no server name provided, try to discover it from Caddy
	if serverName == "" {
		discoveredName, err := client.DiscoverServerName()
		if err != nil {
			logger.Error("caddy", "Failed to discover server name, using default 'tailrelay': %v", err)
			serverName = "tailrelay"
		} else {
			serverName = discoveredName
			logger.Info("caddy", "Auto-detected Caddy server name: %s", serverName)
		}
	}

	return &ProxyManager{
		client:     client,
		serverName: serverName,
	}
}

// AddProxy adds a new reverse proxy route to Caddy via API
func (pm *ProxyManager) AddProxy(proxy config.CaddyProxy) error {
	logger.Debug("caddy", "AddProxy: building route for %s:%d -> %s", proxy.Hostname, proxy.Port, proxy.Target)

	// Ensure the HTTP server and routes array exist
	if err := pm.ensureServerExists(); err != nil {
		logger.Error("caddy", "Failed to ensure server exists: %v", err)
		return fmt.Errorf("ensure server exists: %w", err)
	}

	route, err := pm.buildRoute(proxy)
	if err != nil {
		logger.Error("caddy", "Failed to build route for proxy %s: %v", proxy.ID, err)
		return fmt.Errorf("build route: %w", err)
	}

	// Use POST to append to routes array
	// The /... suffix tells Caddy to expand array elements
	path := fmt.Sprintf("/apps/http/servers/%s/routes", pm.serverName)
	logger.Debug("caddy", "Adding route to Caddy at path: %s", path)

	if err := pm.client.PostConfig(path, route); err != nil {
		logger.Error("caddy", "Failed to add proxy route %s:%d via Caddy API: %v", proxy.Hostname, proxy.Port, err)
		return fmt.Errorf("add route: %w", err)
	}

	logger.Info("caddy", "Added Caddy proxy: %s:%d -> %s (ID: %s)", proxy.Hostname, proxy.Port, proxy.Target, proxy.ID)
	return nil
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

	route, err := pm.buildRoute(proxy)
	if err != nil {
		logger.Error("caddy", "Failed to build route for proxy update %s: %v", proxy.ID, err)
		return fmt.Errorf("build route: %w", err)
	}

	// Use PATCH to replace the entire route by ID
	if err := pm.client.PatchByID(proxy.ID, route); err != nil {
		logger.Error("caddy", "Failed to update proxy %s via Caddy API: %v", proxy.ID, err)
		return fmt.Errorf("update route: %w", err)
	}

	logger.Info("caddy", "Updated Caddy proxy: %s (ID: %s)", proxy.Hostname, proxy.ID)
	return nil
}

// DeleteProxy removes a proxy by ID
func (pm *ProxyManager) DeleteProxy(id string) error {
	logger.Debug("caddy", "DeleteProxy: removing proxy ID %s", id)

	if err := pm.client.DeleteByID(id); err != nil {
		logger.Error("caddy", "Failed to delete proxy %s via Caddy API: %v", id, err)
		return fmt.Errorf("delete route: %w", err)
	}

	logger.Info("caddy", "Deleted Caddy proxy: %s", id)
	return nil
}

// ListProxies retrieves all proxies for the server
func (pm *ProxyManager) ListProxies() ([]config.CaddyProxy, error) {
	path := fmt.Sprintf("/apps/http/servers/%s/routes", pm.serverName)
	data, err := pm.client.GetConfig(path)
	if err != nil {
		return nil, fmt.Errorf("get routes: %w", err)
	}

	var routes []Route
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("unmarshal routes: %w", err)
	}

	proxies := make([]config.CaddyProxy, 0, len(routes))
	for _, route := range routes {
		proxy, err := pm.routeToProxy(route)
		if err != nil {
			// Skip routes that can't be converted (may not be reverse proxies)
			continue
		}
		proxies = append(proxies, *proxy)
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

	// Build headers configuration
	headers := HeaderConfig{
		Request: &HeaderOps{
			Set: []HeaderOperation{
				{
					Field:  "Host",
					Values: []string{"{http.reverse_proxy.upstream.hostport}"},
				},
			},
		},
	}

	// Add custom headers if provided
	if len(proxy.CustomHeaders) > 0 {
		for key, value := range proxy.CustomHeaders {
			headers.Request.Set = append(headers.Request.Set, HeaderOperation{
				Field:  key,
				Values: []string{value},
			})
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

	// Extract hostname and port from matchers
	if len(route.Match) > 0 && len(route.Match[0].Host) > 0 {
		hostPort := route.Match[0].Host[0]
		parts := strings.Split(hostPort, ":")
		if len(parts) == 2 {
			proxy.Hostname = parts[0]
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
			if setOps, ok := request["set"].([]interface{}); ok {
				proxy.CustomHeaders = make(map[string]string)
				for _, op := range setOps {
					if opMap, ok := op.(map[string]interface{}); ok {
						field, _ := opMap["field"].(string)
						if field != "Host" { // Skip default Host header
							if values, ok := opMap["value"].([]interface{}); ok && len(values) > 0 {
								if value, ok := values[0].(string); ok {
									proxy.CustomHeaders[field] = value
								}
							}
						}
					}
				}
			}
		}
	}

	return proxy, nil
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

	path := fmt.Sprintf("/apps/http/servers/%s", pm.serverName)

	// Try to create the server (will fail if it exists, which is fine)
	err := pm.client.PutConfig(path, server)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		// If error is not "already exists", return it
		return fmt.Errorf("initialize server: %w", err)
	}

	return nil
}

// ensureServerExists ensures the HTTP server and routes array exist before adding routes
func (pm *ProxyManager) ensureServerExists() error {
	// Check if the server exists
	path := fmt.Sprintf("/apps/http/servers/%s", pm.serverName)
	_, err := pm.client.GetConfig(path)
	
	if err != nil {
		// Server doesn't exist, try to create it
		logger.Info("caddy", "HTTP server '%s' not found, creating it...", pm.serverName)
		server := &HTTPServer{
			Listen: []string{":80", ":443"},
			Routes: []Route{},
		}
		
		if err := pm.client.PutConfig(path, server); err != nil {
			return fmt.Errorf("create server: %w", err)
		}
		logger.Info("caddy", "Created HTTP server '%s'", pm.serverName)
	}
	
	// Ensure routes array exists (might be null)
	routesPath := fmt.Sprintf("/apps/http/servers/%s/routes", pm.serverName)
	_, err = pm.client.GetConfig(routesPath)
	
	if err != nil {
		// Routes array doesn't exist, initialize it
		logger.Info("caddy", "Routes array not found, initializing empty array...")
		emptyRoutes := []Route{}
		if err := pm.client.PutConfig(routesPath, emptyRoutes); err != nil {
			return fmt.Errorf("initialize routes: %w", err)
		}
		logger.Info("caddy", "Initialized routes array for server '%s'", pm.serverName)
	}
	
	return nil
}
